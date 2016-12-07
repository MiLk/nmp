package shared

type TinyRecord struct {
	Timestamp uint64
	Data      map[string]interface{}
}

type RecordSet struct {
	Tag     string
	Records []TinyRecord
}

type InputListener interface {
	Emit(recordSets []RecordSet) error
}

type CheckResult struct {
	Hostname    string
	Type        string
	ServiceName string
	Code        uint8
	Output      string
}

type Writer interface {
	Emit(checkResults []CheckResult) error
}
