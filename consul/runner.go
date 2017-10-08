package consul

import (
	"time"

	"github.com/Sirupsen/logrus"
)

type Runner struct {
	logger *logrus.Logger
	stopCh chan bool
	doneCh chan bool
}

func (r *Runner) spawnRunner() {
	r.logger.Info("Spawning runner")
	go func() {
		if err := loadNodesFromAllDatacenters(); err != nil {
			r.logger.Error(err)
		}
		for {
			select {
			case <-r.stopCh:
				break
			case <-time.After(1 * time.Minute):
				if err := loadNodesFromAllDatacenters(); err != nil {
					r.logger.Error(err)
				}
			}
		}
		r.logger.Info("Runner ended")
		r.doneCh <- true
	}()
}

func (r *Runner) String() string {
	return "runner"
}

func (r *Runner) Start() {
	r.spawnRunner()
}

func (r *Runner) Stop() {
	r.stopCh <- true
	close(r.stopCh)
}

func (r *Runner) WaitForShutdown() {
	<-r.doneCh
}

func NewRunner(logger *logrus.Logger) *Runner {
	return &Runner{
		logger: logger,
		stopCh: make(chan bool, 1),
		doneCh: make(chan bool, 1),
	}
}
