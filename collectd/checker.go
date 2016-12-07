package collectd

import (
	"bytes"
	"sync"
	"sync/atomic"

	"github.com/Sirupsen/logrus"

	"github.com/milk/nmp/config"
	"github.com/milk/nmp/shared"
)

type Checker struct {
	logger         *logrus.Logger
	wg             sync.WaitGroup
	emitterChan    chan CollectdRecord
	isShuttingDown uintptr
	checks         map[string][]shared.CheckerRule
	transformer    shared.Transformer
}

func (checker *Checker) checkRecord(record CollectdRecord) ([]shared.CheckResult, error) {
	results := []shared.CheckResult{}
	if rules, ok := checker.checks[record.Plugin]; ok {
		for _, rule := range rules {
			if rule.Check.PluginInstance != "" && rule.Check.PluginInstance != record.PluginInstance {
				continue
			}
			if rule.Check.Type != "" && rule.Check.Type != record.Type {
				continue
			}
			if rule.Check.TypeInstance != "" && rule.Check.TypeInstance != record.TypeInstance {
				continue
			}

			buf := new(bytes.Buffer)
			err := rule.Check.Value.Execute(buf, record)
			if err != nil {
				checker.logger.Error(err)
				continue
			}
			value := buf.String()

			// CRITICAL CHECK
			result, err := rule.Compare(value, rule.Check.Critical)
			if err != nil {
				checker.logger.Error(err)
				continue
			}
			if result {
				results = append(results, shared.CheckResult{
					Code:        2,
					Hostname:    record.Host,
					Type:        "service",
					ServiceName: rule.Name,
					Output:      value,
				})
				continue
			}

			// WARNING CHECK
			result, err = rule.Compare(value, rule.Check.Warning)
			if err != nil {
				checker.logger.Error(err)
				continue
			}
			if result {
				results = append(results, shared.CheckResult{
					Code:        1,
					Hostname:    record.Host,
					Type:        "service",
					ServiceName: rule.Name,
					Output:      value,
				})
				continue
			}

			// SUCCESS
			results = append(results, shared.CheckResult{
				Code:        0,
				Hostname:    record.Host,
				Type:        "service",
				ServiceName: rule.Name,
				Output:      value,
			})
		}
	}
	return results, nil
}

func (checker *Checker) spawnChecker() {
	checker.logger.Info("Spawning checker")
	checker.wg.Add(1)
	go func() {
		defer func() {
			checker.wg.Done()
		}()
		checker.logger.Info("Checker started")
		for record := range checker.emitterChan {
			checkResults, err := checker.checkRecord(record)
			if err != nil {
				checker.logger.Error(err)
				continue
			}
			if len(checkResults) > 0 {
				checker.transformer.Emit(checkResults)
			}
		}
		checker.logger.Info("Checker ended")
	}()
}

func (checker *Checker) Emit(record CollectdRecord) error {
	defer func() {
		recover()
	}()
	checker.emitterChan <- record
	return nil
}

func (checker *Checker) String() string {
	return "checker"
}

func (checker *Checker) Stop() {
	if atomic.CompareAndSwapUintptr(&checker.isShuttingDown, 0, 1) {
		close(checker.emitterChan)
	}
}

func (checker *Checker) WaitForShutdown() {
	checker.wg.Wait()
}

func (checker *Checker) Start() {
	checker.spawnChecker()
}

func NewChecker(logger *logrus.Logger, checks map[string]config.Check, transformer shared.Transformer) (*Checker, error) {
	_checks := map[string][]shared.CheckerRule{}

	for k, v := range checks {
		if _, ok := _checks[v.Plugin]; !ok {
			_checks[v.Plugin] = []shared.CheckerRule{}
		}
		_checks[v.Plugin] = append(_checks[v.Plugin], shared.CheckerRule{Name: k, Check: v})
	}

	checker := &Checker{
		logger:         logger,
		wg:             sync.WaitGroup{},
		emitterChan:    make(chan CollectdRecord),
		isShuttingDown: 0,
		checks:         _checks,
		transformer:    transformer,
	}
	return checker, nil
}
