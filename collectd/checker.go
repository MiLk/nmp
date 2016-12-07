package collectd

import (
	"bytes"
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/hashicorp/hil"
	"github.com/milk/nmp/config"
	"strconv"
	"sync"
	"sync/atomic"
)

type CheckerRule struct {
	Check config.Check
	Name  string
}

type CheckResult struct {
	Code   uint8
	Record *CollectdRecord
	Rule   *CheckerRule
}

func (result *CheckResult) IsEmpty() bool {
	return result.Code == 0 && result.Record == nil && result.Rule == nil
}

func (rule *CheckerRule) CompareFloat64(value float64, threshold float64) (bool, error) {
	switch rule.Check.Comparator {
	case config.GreaterThan:
		return value > threshold, nil
	case config.GreaterThanOrEqualTo:
		return value >= threshold, nil
	case config.LesserThanOrEqualTo:
		return value <= threshold, nil
	case config.LesserThan:
		return value < threshold, nil
	}
	return false, fmt.Errorf("Invalid comparator for rule %+v", rule)
}

func (rule *CheckerRule) Compare(value string, threshold hil.EvaluationResult) (bool, error) {
	if threshold.Type != hil.TypeString {
		return false, fmt.Errorf("Invalid configuration for rule %+v", rule)
	}

	thresholdF, err := strconv.ParseFloat(threshold.Value.(string), 64)
	if err != nil {
		return false, err
	}
	valueF, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return false, err
	}
	return rule.CompareFloat64(valueF, thresholdF)
}

type Checker struct {
	logger         *logrus.Logger
	wg             sync.WaitGroup
	emitterChan    chan CollectdRecord
	isShuttingDown uintptr
	checks         map[string][]CheckerRule
}

func (checker *Checker) checkRecord(record CollectdRecord) ([]CheckResult, error) {
	results := []CheckResult{}
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

			// CRITICAL CHECK
			result, err := rule.Compare(buf.String(), rule.Check.Critical)
			if err != nil {
				checker.logger.Error(err)
				continue
			}
			if result {
				results = append(results, CheckResult{Code: 2, Rule: &rule, Record: &record})
				continue
			}

			// WARNING CHECK
			result, err = rule.Compare(buf.String(), rule.Check.Warning)
			if err != nil {
				checker.logger.Error(err)
				continue
			}
			if result {
				results = append(results, CheckResult{Code: 1, Rule: &rule, Record: &record})
				continue
			}

			// SUCCESS
			results = append(results, CheckResult{Code: 0, Rule: &rule, Record: &record})
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
				fmt.Printf("Check results: %+v\n", checkResults)
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

func NewChecker(logger *logrus.Logger, checks map[string]config.Check) (*Checker, error) {
	_checks := map[string][]CheckerRule{}

	for k, v := range checks {
		if _, ok := _checks[v.Plugin]; !ok {
			_checks[v.Plugin] = []CheckerRule{}
		}
		_checks[v.Plugin] = append(_checks[v.Plugin], CheckerRule{Name: k, Check: v})
	}

	checker := &Checker{
		logger:         logger,
		wg:             sync.WaitGroup{},
		emitterChan:    make(chan CollectdRecord),
		isShuttingDown: 0,
		checks:         _checks,
	}
	return checker, nil
}
