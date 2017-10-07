package shared

import (
	"fmt"
	"strconv"

	"github.com/hashicorp/hil"

	"github.com/MiLk/nmp/config"
)

type CheckerRule struct {
	Check config.Check
	Name  string
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
