package nagios

import (
	"bytes"
	"sync"
	"sync/atomic"
	"text/template"
	"time"

	"github.com/Sirupsen/logrus"

	"github.com/milk/nmp/shared"
)

type Transformer struct {
	logger          *logrus.Logger
	wg              sync.WaitGroup
	transformerChan chan []shared.CheckResult
	isShuttingDown  uintptr
	template        *template.Template
	writer          shared.Writer
}

type TemplateData struct {
	CheckResult shared.CheckResult
	FinishTime  int64
	Date        string
}

func (transformer *Transformer) spawnTransformer() {
	transformer.logger.Info("Spawning transformer")
	transformer.wg.Add(1)
	go func() {
		defer func() {
			transformer.wg.Done()
		}()
		transformer.logger.Info("Transformer started")

		buf := new(bytes.Buffer)
		for checks := range transformer.transformerChan {
			for _, check := range checks {
				now := time.Now()
				err := transformer.template.Execute(buf, TemplateData{
					CheckResult: check,
					FinishTime:  now.Unix(),
					Date:        now.Format("Mon Jan 02 15:04:05 -0700 2006"),
				})
				if err != nil {
					transformer.logger.Error(err)
					continue
				}
				transformer.writer.Emit(buf.String())
			}
		}
		transformer.logger.Info("Transformer ended")
	}()
}

func (transformer *Transformer) Emit(resultChecks []shared.CheckResult) error {
	defer func() {
		recover()
	}()
	transformer.transformerChan <- resultChecks
	return nil
}

func (transformer *Transformer) String() string {
	return "transformer"
}

func (transformer *Transformer) Stop() {
	if atomic.CompareAndSwapUintptr(&transformer.isShuttingDown, 0, 1) {
		close(transformer.transformerChan)
	}
}

func (transformer *Transformer) WaitForShutdown() {
	transformer.wg.Wait()
}

func (transformer *Transformer) Start() {
	transformer.spawnTransformer()
}

func NewTransformer(logger *logrus.Logger, writer shared.Writer) (*Transformer, error) {

	checkTemplate := `### NMP Check ###
latency=0
start_time={{ .FinishTime }}.0
finish_time={{ .FinishTime }}.0
# Time: {{ .Date -}}
{{ with .CheckResult }}
host_name={{ .Hostname }}
{{ if (eq .Type "service") }}service_description={{ .ServiceName }}{{ end }}
check_type=1
early_timeout=1
exited_ok=1
return_code={{ .Code }}
output={{ .Output }}
{{ end }}
`
	t, err := template.New("nagios writter").Parse(checkTemplate)
	if err != nil {
		return nil, err
	}

	transformer := &Transformer{
		logger:          logger,
		wg:              sync.WaitGroup{},
		transformerChan: make(chan []shared.CheckResult),
		isShuttingDown:  0,
		template:        t,
		writer:          writer,
	}
	return transformer, nil
}
