package collectd

import (
	"bytes"
	"regexp"
	"sync"
	"sync/atomic"

	"github.com/Sirupsen/logrus"

	"github.com/hashicorp/hil"
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

func (checker *Checker) checkThreshold(rule shared.CheckerRule, value string, threshold hil.EvaluationResult, code uint8, hostname string) (*shared.CheckResult, error) {
	result, err := rule.Compare(value, rule.Check.Critical)
	if err != nil {
		return nil, err
	}
	if result {
		return &shared.CheckResult{
			Code:        code,
			Hostname:    hostname,
			Type:        "service",
			ServiceName: rule.Name,
			Output:      value,
		}, nil
	}
	return nil, nil
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

			// Load host specific thresholds
			critical := rule.Check.Critical
			warning := rule.Check.Warning
			for pattern, threshold := range rule.Check.Thresholds {
				matched, err := regexp.MatchString(pattern, record.Host)
				if err != nil {
					checker.logger.Error(err)
					continue
				}
				if matched {
					critical = threshold.Critical
					warning = threshold.Warning
					break
				}
			}

			// CRITICAL CHECK
			result, err := checker.checkThreshold(rule, value, critical, 2, record.Host)
			if err != nil {
				checker.logger.Error(err)
				continue
			}
			if result != nil {
				checker.logger.Infof("CRITICAL: %s - %s | %+v\n", record.Host, rule.Name, result)
				results = append(results, *result)
				continue
			}

			// WARNING CHECK
			result, err = checker.checkThreshold(rule, value, warning, 1, record.Host)
			if err != nil {
				checker.logger.Error(err)
				continue
			}
			if result != nil {
				checker.logger.Infof("WARNING: %s - %s | %+v\n", record.Host, rule.Name, result)
				results = append(results, *result)
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
