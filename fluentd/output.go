package fluentd

import "fmt"

type TestOutput struct{}

func (output *TestOutput) Emit(recordSets []FluentRecordSet) error {
	for _, recordSet := range recordSets {
		fmt.Printf("%+v\n", recordSet)
	}
	return nil
}
