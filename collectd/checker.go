package collectd

import (
	"bytes"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/Sirupsen/logrus"
	"github.com/hashicorp/hil"

	"github.com/MiLk/nmp/config"
	"github.com/MiLk/nmp/consul"
	"github.com/MiLk/nmp/shared"
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
	result, err := rule.Compare(value, threshold)
	if err != nil {
		return nil, err
	}
	if result {
		return &shared.CheckResult{
			Code:        code,
			Hostname:    hostname,
			Type:        "service",
			ServiceName: rule.Name,
			Output:      rule.Check.FormatOutput(value),
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

			critical := rule.Check.Critical
			warning := rule.Check.Warning

			matchName := "default"

			// Load meta specific thresholds
			for pattern, threshold := range rule.Check.MetaThresholds {
				node := consul.GetNode(record.Host)
				if node == nil {
					continue
				}

				splitted := strings.SplitN(pattern, ":", 2)
				v, ok := node.Meta[splitted[0]]
				if !ok || v != splitted[1] {
					continue
				}

				matchName = fmt.Sprintf("meta:%s", pattern)

				critical = threshold.Critical
				warning = threshold.Warning
				break
			}

			// Load host specific thresholds
			priority := 0
			for pattern, threshold := range rule.Check.HostThresholds {
				if threshold.Regexp == nil {
					continue
				}

				if threshold.Priority < priority {
					continue
				}

				if !threshold.Regexp.MatchString(record.Host) {
					continue
				}

				matchName = fmt.Sprintf("host:%s", pattern)

				priority = threshold.Priority
				critical = threshold.Critical
				warning = threshold.Warning
			}

			// CRITICAL CHECK
			result, err := checker.checkThreshold(rule, value, critical, 2, record.Host)
			if err != nil {
				checker.logger.Error(err)
				continue
			}
			if result != nil {
				checker.logger.Infof("CRITICAL: %s - %s - %s | %+v | %s %s %s\n", record.Host, rule.Name, matchName, result, value, rule.Check.Comparator, critical)
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
				checker.logger.Infof("WARNING: %s - %s - %s | %+v | %s %s %s\n", record.Host, rule.Name, matchName, result, value, rule.Check.Comparator, warning)
				results = append(results, *result)
				continue
			}

			// SUCCESS
			results = append(results, shared.CheckResult{
				Code:        0,
				Hostname:    record.Host,
				Type:        "service",
				ServiceName: rule.Name,
				Output:      rule.Check.FormatOutput(value),
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
