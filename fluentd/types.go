package fluentd

type FluentRecord struct {
	Tag       string
	Timestamp uint64
	Data      map[string]interface{}
}
