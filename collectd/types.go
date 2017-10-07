package collectd

type TagList map[string]bool

type CollectdRecord struct {
	Tag            string
	Timestamp      uint64
	Raw            map[string]interface{}
	Host           string
	HostShort      string
	Plugin         string
	PluginInstance string
	Type           string
	TypeInstance   string
	Values         []interface{}
	DsTypes        interface{}
	DsNames        interface{}
	Interval       uint8
}

type CollectdCheckerListener interface {
	Emit(record CollectdRecord) error
}
