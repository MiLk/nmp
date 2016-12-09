package collectd

import (
	"reflect"
	"sync"
	"sync/atomic"

	"github.com/Sirupsen/logrus"

	"github.com/milk/nmp/shared"
)

type Transformer struct {
	logger         *logrus.Logger
	listener       CollectdCheckerListener
	wg             sync.WaitGroup
	emitterChan    chan shared.RecordSet
	isShuttingDown uintptr
	tagList        TagList
}

func (transformer *Transformer) TransformRecord(tag string, record shared.TinyRecord, transformed *CollectdRecord) error {
	p := reflect.ValueOf(transformed).Elem()
	p.Set(reflect.Zero(p.Type()))

	transformed.Tag = tag
	transformed.Timestamp = record.Timestamp
	transformed.Raw = record.Data

	for k, v := range record.Data {
		switch k {
		case "host":
			transformed.Host = v.(string)
		case "plugin":
			transformed.Plugin = v.(string)
		case "plugin_instance":
			transformed.PluginInstance = v.(string)
		case "type":
			transformed.Type = v.(string)
		case "type_instance":
			transformed.TypeInstance = v.(string)
		case "values":
			transformed.Values = v.([]interface{})
		case "dstypes":
			transformed.DsTypes = v
		case "dsnames":
			transformed.DsNames = v
		case "interval":
			transformed.Interval = uint8(v.(float64))
		default:
			transformer.logger.Warnf("Unhandled field %s: %+v\n", k, v)
		}
	}
	return nil
}

func (transformer *Transformer) spawnTransformer() {
	transformer.logger.Info("Spawning transformer")
	transformer.wg.Add(1)
	go func() {
		defer func() {
			transformer.wg.Done()
		}()
		transformer.logger.Info("Transformer started")

		transformed := CollectdRecord{}
		for recordSet := range transformer.emitterChan {
			for _, record := range recordSet.Records {
				transformer.TransformRecord(recordSet.Tag, record, &transformed)
				transformer.listener.Emit(transformed)
			}
		}
		transformer.logger.Info("Transformer ended")
	}()
}

func (transformer *Transformer) Emit(recordSets []shared.RecordSet) error {
	defer func() {
		recover()
	}()
	for _, recordSet := range recordSets {
		if transformer.tagList[recordSet.Tag] {
			transformer.emitterChan <- recordSet
		}
	}
	return nil
}

func (transformer *Transformer) String() string {
	return "transformer"
}

func (transformer *Transformer) Stop() {
	if atomic.CompareAndSwapUintptr(&transformer.isShuttingDown, 0, 1) {
		close(transformer.emitterChan)
	}
}

func (transformer *Transformer) WaitForShutdown() {
	transformer.wg.Wait()
}

func (transformer *Transformer) Start() {
	transformer.spawnTransformer()
}

func NewTransformer(logger *logrus.Logger, tagList []string, listener CollectdCheckerListener) (*Transformer, error) {

	_tagList := TagList{}
	for _, _tag := range tagList {
		_tagList[_tag] = true
	}

	transformer := &Transformer{
		logger:         logger,
		listener:       listener,
		wg:             sync.WaitGroup{},
		emitterChan:    make(chan shared.RecordSet),
		isShuttingDown: 0,
		tagList:        _tagList,
	}
	return transformer, nil
}
