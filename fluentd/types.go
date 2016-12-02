package fluentd

type FluentRecord struct {
	Tag       string
	Timestamp uint64
	Data      map[string]interface{}
}

type TinyFluentRecord struct {
	Timestamp uint64
	Data      map[string]interface{}
}

type FluentRecordSet struct {
	Tag     string
	Records []TinyFluentRecord
}

type Port interface {
	Emit(recordSets []FluentRecordSet) error
}
