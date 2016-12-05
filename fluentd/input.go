package fluentd

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/ugorji/go/codec"
	"io"
	"net"
	"reflect"
	"sync"
	"sync/atomic"
	"github.com/milk/nmp/shared"
)

type forwardClient struct {
	input  *ForwardInput
	logger *logrus.Logger
	conn   *net.TCPConn
	codec  *codec.MsgpackHandle
	dec    *codec.Decoder
}

type ForwardInput struct {
	entries        int64 // This variable must be on 64-bit alignment. Otherwise atomic.AddInt64 will cause a crash on ARM and x86-32
	port           shared.InputListener
	logger         *logrus.Logger
	bind           string
	listener       *net.TCPListener
	codec          *codec.MsgpackHandle
	clientsMtx     sync.Mutex
	clients        map[*net.TCPConn]*forwardClient
	wg             sync.WaitGroup
	acceptChan     chan *net.TCPConn
	shutdownChan   chan struct{}
	isShuttingDown uintptr
}

type EntryCountTopic struct{}

type ConnectionCountTopic struct{}

type ForwardInputFactory struct{}

func coerceInPlace(data map[string]interface{}) {
	for k, v := range data {
		switch v_ := v.(type) {
		case []byte:
			data[k] = string(v_) // XXX: byte => rune
		case map[string]interface{}:
			coerceInPlace(v_)
		}
	}
}

func (c *forwardClient) decodeRecordSet(tag []byte, entries []interface{}) (shared.RecordSet, error) {

	records := make([]shared.TinyRecord, len(entries))
	for i, _entry := range entries {
		entry, ok := _entry.([]interface{})
		if !ok {
			return shared.RecordSet{}, errors.New("Failed to decode recordSet")
		}
		var timestamp = uint64(0)
		timestampFloat, ok := entry[0].(float64)
		if !ok {
			timestamp, ok = entry[0].(uint64)
			if !ok {
				return shared.RecordSet{}, errors.New("Failed to decode timestamp field")
			}
		} else {
			timestamp = uint64(timestampFloat)
		}

		data, ok := entry[1].(map[string]interface{})
		if !ok {
			return shared.RecordSet{}, errors.New("Failed to decode data field")
		}
		coerceInPlace(data)
		records[i] = shared.TinyRecord{
			Timestamp: timestamp,
			Data:      data,
		}
	}
	return shared.RecordSet{
		Tag:     string(tag), // XXX: byte => rune
		Records: records,
	}, nil
}

func (c *forwardClient) decodeEntries() ([]shared.RecordSet, error) {
	v := []interface{}{nil, nil, nil}
	err := c.dec.Decode(&v)
	if err != nil {
		return nil, err
	}
	tag, ok := v[0].([]byte)
	if !ok {
		return nil, errors.New("Failed to decode tag field")
	}

	var retval []shared.RecordSet
	switch timestamp_or_entries := v[1].(type) {
	case uint64:
		timestamp := timestamp_or_entries
		data, ok := v[2].(map[string]interface{})
		if !ok {
			return nil, errors.New("Failed to decode data field")
		}
		coerceInPlace(data)
		retval = []shared.RecordSet{
			{
				Tag: string(tag), // XXX: byte => rune
				Records: []shared.TinyRecord{
					{
						Timestamp: timestamp,
						Data:      data,
					},
				},
			},
		}
	case float64:
		timestamp := uint64(timestamp_or_entries)
		data, ok := v[2].(map[string]interface{})
		if !ok {
			return nil, errors.New("Failed to decode data field")
		}
		retval = []shared.RecordSet{
			{
				Tag: string(tag), // XXX: byte => rune
				Records: []shared.TinyRecord{
					{
						Timestamp: timestamp,
						Data:      data,
					},
				},
			},
		}
	case []interface{}:
		if !ok {
			return nil, errors.New("Unexpected payload format")
		}
		recordSet, err := c.decodeRecordSet(tag, timestamp_or_entries)
		if err != nil {
			return nil, err
		}
		retval = []shared.RecordSet{recordSet}
	case []byte:
		entries := make([]interface{}, 0)
		reader := bytes.NewReader(timestamp_or_entries)
		dec := codec.NewDecoder(reader, c.codec)
		for reader.Len() > 0 { // codec.Decoder doesn't return EOF.
			entry := []interface{}{}
			if err != dec.Decode(&entry) {
				if err == io.EOF { // in case codec.Decoder changes its behavior
					break
				}
				return nil, err
			}
			entries = append(entries, entry)
		}
		recordSet, err := c.decodeRecordSet(tag, entries)
		if err != nil {
			return nil, err
		}
		retval = []shared.RecordSet{recordSet}
	default:
		return nil, errors.New(fmt.Sprintf("Unknown type: %t", timestamp_or_entries))
	}
	atomic.AddInt64(&c.input.entries, int64(len(retval)))
	return retval, nil
}

func (c *forwardClient) startHandling() {
	c.input.wg.Add(1)
	go func() {
		defer func() {
			err := c.conn.Close()
			if err != nil {
				c.logger.Debugf("Close: %s", err.Error())
			}
			c.input.markDischarged(c)
			c.input.wg.Done()
		}()
		c.input.logger.Infof("Started handling connection from %s", c.conn.RemoteAddr().String())
		for {
			recordSets, err := c.decodeEntries()
			if err != nil {
				err_, ok := err.(net.Error)
				if ok {
					if err_.Temporary() {
						c.logger.Infof("Temporary failure: %s", err_.Error())
						continue
					}
				}
				if err == io.EOF {
					c.logger.Infof("Client %s closed the connection", c.conn.RemoteAddr().String())
				} else {
					c.logger.Error(err.Error())
				}
				break
			}

			if len(recordSets) > 0 {
				err_ := c.input.port.Emit(recordSets)
				if err_ != nil {
					c.logger.Error(err_.Error())
					break
				}
			}
		}
		c.input.logger.Infof("Ended handling connection from %s", c.conn.RemoteAddr().String())
	}()
}

func (c *forwardClient) shutdown() {
	err := c.conn.Close()
	if err != nil {
		c.input.logger.Infof("Error during closing connection: %s", err.Error())
	}
}

func newForwardClient(input *ForwardInput, logger *logrus.Logger, conn *net.TCPConn, _codec *codec.MsgpackHandle) *forwardClient {
	c := &forwardClient{
		input:  input,
		logger: logger,
		conn:   conn,
		codec:  _codec,
		dec:    codec.NewDecoder(bufio.NewReader(conn), _codec),
	}
	input.markCharged(c)
	return c
}

func (input *ForwardInput) spawnAcceptor() {
	input.logger.Info("Spawning acceptor")
	input.wg.Add(1)
	go func() {
		defer func() {
			close(input.acceptChan)
			input.wg.Done()
		}()
		input.logger.Info("Acceptor started")
		for {
			conn, err := input.listener.AcceptTCP()
			if err != nil {
				input.logger.Info(err.Error())
				break
			}
			if conn != nil {
				input.logger.Infof("Connected from %s", conn.RemoteAddr().String())
				input.acceptChan <- conn
			} else {
				input.logger.Info("AcceptTCP returned nil; something went wrong")
				break
			}
		}
		input.logger.Info("Acceptor ended")
	}()
}

func (input *ForwardInput) spawnDaemon() {
	input.logger.Info("Spawning daemon")
	input.wg.Add(1)
	go func() {
		defer func() {
			close(input.shutdownChan)
			input.wg.Done()
		}()
		input.logger.Info("Daemon started")
	loop:
		for {
			select {
			case conn := <-input.acceptChan:
				if conn != nil {
					input.logger.Info("Got conn from acceptChan")
					newForwardClient(input, input.logger, conn, input.codec).startHandling()
				}
			case <-input.shutdownChan:
				input.listener.Close()
				for _, client := range input.clients {
					client.shutdown()
				}
				break loop
			}
		}
		input.logger.Info("Daemon ended")
	}()
}

func (input *ForwardInput) markCharged(c *forwardClient) {
	input.clientsMtx.Lock()
	defer input.clientsMtx.Unlock()
	input.clients[c.conn] = c
}

func (input *ForwardInput) markDischarged(c *forwardClient) {
	input.clientsMtx.Lock()
	defer input.clientsMtx.Unlock()
	delete(input.clients, c.conn)
}

func (input *ForwardInput) String() string {
	return "input"
}

func (input *ForwardInput) Start() {
	input.spawnAcceptor()
	input.spawnDaemon()
}

func (input *ForwardInput) WaitForShutdown() {
	input.wg.Wait()
}

func (input *ForwardInput) Stop() {
	if atomic.CompareAndSwapUintptr(&input.isShuttingDown, uintptr(0), uintptr(1)) {
		input.shutdownChan <- struct{}{}
	}
}

func NewForwardInput(logger *logrus.Logger, bind string, port shared.InputListener) (*ForwardInput, error) {
	_codec := codec.MsgpackHandle{}
	_codec.MapType = reflect.TypeOf(map[string]interface{}(nil))
	_codec.RawToString = false
	addr, err := net.ResolveTCPAddr("tcp", bind)
	if err != nil {
		logger.Error(err.Error())
		return nil, err
	}
	listener, err := net.ListenTCP("tcp", addr)
	if err != nil {
		logger.Error(err.Error())
		return nil, err
	}
	return &ForwardInput{
		port:           port,
		logger:         logger,
		bind:           bind,
		listener:       listener,
		codec:          &_codec,
		clients:        make(map[*net.TCPConn]*forwardClient),
		clientsMtx:     sync.Mutex{},
		entries:        0,
		wg:             sync.WaitGroup{},
		acceptChan:     make(chan *net.TCPConn),
		shutdownChan:   make(chan struct{}),
		isShuttingDown: uintptr(0),
	}, nil
}
