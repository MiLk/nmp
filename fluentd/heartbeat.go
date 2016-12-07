package fluentd

import (
	"net"
	"sync"
	"sync/atomic"

	"github.com/Sirupsen/logrus"
)

type HeartbeatInput struct {
	logger         *logrus.Logger
	listener       *net.UDPConn
	wg             sync.WaitGroup
	isShuttingDown uintptr
}

func (input *HeartbeatInput) spawnDaemon() {
	input.logger.Info("Spawning Fluentd Heartbeat Daemon")
	input.wg.Add(1)
	go func() {
		defer func() {
			input.wg.Done()
		}()
		input.logger.Info("Fluentd Heartbeat Daemon started")

		buf := make([]byte, 1024)
		for input.isShuttingDown == 0 {
			_, addr, err := input.listener.ReadFromUDP(buf)
			if err != nil {
				input.logger.Error(err.Error())
			}

			_, err = input.listener.WriteTo(buf, addr)
			if err != nil {
				input.logger.Error(err.Error())
			}
		}
		input.logger.Info("Fluentd Heartbeat Daemon ended")
	}()
}

func (input *HeartbeatInput) Start() {
	input.spawnDaemon()
}

func (input *HeartbeatInput) WaitForShutdown() {
	input.wg.Wait()
}

func (input *HeartbeatInput) Stop() {
	atomic.CompareAndSwapUintptr(&input.isShuttingDown, uintptr(0), uintptr(1))
}

func (input *HeartbeatInput) String() string {
	return "heartbeat"
}

func NewForwardHeartbeatInput(logger *logrus.Logger, bind string) (*HeartbeatInput, error) {
	addr, err := net.ResolveUDPAddr("udp", bind)
	if err != nil {
		logger.Error(err.Error())
		return nil, err
	}
	listener, err := net.ListenUDP("udp", addr)
	if err != nil {
		logger.Error(err.Error())
		return nil, err
	}

	return &HeartbeatInput{
		logger:   logger,
		listener: listener,
		wg:       sync.WaitGroup{},
	}, nil
}
