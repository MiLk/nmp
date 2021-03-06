package config

import (
	"fmt"
	"io/ioutil"
	"regexp"
	"strconv"
	"text/template"

	"github.com/dustin/go-humanize"
	"github.com/hashicorp/hcl"
	"github.com/hashicorp/hil"
)

type Comparator string

const (
	GreaterThan          Comparator = ">"
	GreaterThanOrEqualTo Comparator = ">="
	LesserThanOrEqualTo  Comparator = "<="
	LesserThan           Comparator = "<"
)

type CheckThreshold struct {
	WarningTpl  string               `hcl:"warning"`
	CriticalTpl string               `hcl:"critical"`
	Priority    int                  `hcl:"priority"`
	Warning     hil.EvaluationResult `hcl:"-"`
	Critical    hil.EvaluationResult `hcl:"-"`
	Regexp      *regexp.Regexp       `hcl:"-"`
}

type CheckThresholdMap map[string]CheckThreshold

func (m CheckThresholdMap) Parse(hilConfig *hil.EvalConfig) (err error) {
	for k, threshold := range m {
		threshold.Critical, err = ParseHIL(threshold.CriticalTpl, hilConfig)
		if err != nil {
			return
		}

		threshold.Warning, err = ParseHIL(threshold.WarningTpl, hilConfig)
		if err != nil {
			return
		}

		m[k] = threshold
	}
	return
}

func (m CheckThresholdMap) CompileRegexp() error {
	for pattern, threshold := range m {
		r, err := regexp.Compile(pattern)
		if err != nil {
			return err
		}
		threshold.Regexp = r
		m[pattern] = threshold
	}
	return nil
}

type Check struct {
	Plugin         string               `hcl:"plugin"`
	PluginInstance string               `hcl:"plugin_instance"`
	Type           string               `hcl:"type"`
	TypeInstance   string               `hcl:"type_instance"`
	Comparator     Comparator           `hcl:"comparator"`
	WarningTpl     string               `hcl:"warning"`
	CriticalTpl    string               `hcl:"critical"`
	Warning        hil.EvaluationResult `hcl:"-"`
	Critical       hil.EvaluationResult `hcl:"-"`
	ValueTpl       string               `hcl:"value"`
	Value          *template.Template   `hcl:"-"`
	HostThresholds CheckThresholdMap    `hcl:"host"`
	MetaThresholds CheckThresholdMap    `hcl:"meta"`
	Humanize       string               `hcl:"humanize"`
}

func (c *Check) FormatOutput(value string) string {
	if c.Humanize == "" {
		return value
	}

	valueF, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return value
	}

	switch c.Humanize {
	case "bytes":
		return humanize.Bytes(uint64(valueF))
	case "ftoa":
		return humanize.Ftoa(valueF)
	}
	return value
}

type Config struct {
	CheckResultsDir string           `hcl:"check_results_dir"`
	Checks          map[string]Check `hcl:"check"`
}

func Read(configFile string) (*Config, error) {
	out, err := loadFileHcl(configFile)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func ParseHIL(input string, hilConfig *hil.EvalConfig) (hil.EvaluationResult, error) {
	tree, err := hil.Parse(input)
	if err != nil {
		return hil.InvalidResult, err
	}

	return hil.Eval(tree, hilConfig)
}

// loadFileHcl is a fileLoaderFunc that knows how to read HCL files.
func loadFileHcl(root string) (*Config, error) {
	// Read the HCL file and prepare for parsing
	d, err := ioutil.ReadFile(root)
	if err != nil {
		return nil, fmt.Errorf("Error reading %s: %s", root, err)
	}

	// Decode it
	var out Config
	if err = hcl.Decode(&out, string(d)); err != nil {
		return nil, fmt.Errorf("Error decoding %s: %s", root, err)
	}

	hilConfig := &hil.EvalConfig{}

	for name, check := range out.Checks {
		if check.Comparator == "" {
			check.Comparator = GreaterThanOrEqualTo
		}

		if check.ValueTpl == "" {
			check.ValueTpl = "{{ (index .Values 0) }}"
		}

		check.Value, err = template.New(name).Parse(check.ValueTpl)
		if err != nil {
			return nil, err
		}
		check.Critical, err = ParseHIL(check.CriticalTpl, hilConfig)
		if err != nil {
			return nil, err
		}
		check.Warning, err = ParseHIL(check.WarningTpl, hilConfig)
		if err != nil {
			return nil, err
		}

		check.HostThresholds.Parse(hilConfig)
		if err := check.HostThresholds.CompileRegexp(); err != nil {
			return nil, err
		}
		check.MetaThresholds.Parse(hilConfig)

		out.Checks[name] = check
	}

	return &out, nil
}
