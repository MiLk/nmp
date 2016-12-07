package nagios

import (
	"bytes"
	"fmt"
	"sync"
	"sync/atomic"
	"text/template"

	"github.com/Sirupsen/logrus"

	"github.com/milk/nmp/shared"
	"time"
)

type Writer struct {
	logger         *logrus.Logger
	wg             sync.WaitGroup
	writerChan     chan []shared.CheckResult
	isShuttingDown uintptr
	template       *template.Template
}

type TemplateData struct {
	CheckResult shared.CheckResult
	FinishTime  int64
	Date        string
}

func (writer *Writer) spawnWriter() {
	writer.logger.Info("Spawning writer")
	writer.wg.Add(1)
	go func() {
		defer func() {
			writer.wg.Done()
		}()
		writer.logger.Info("Writer started")

		buf := new(bytes.Buffer)
		for checks := range writer.writerChan {
			for _, check := range checks {
				now := time.Now()
				err := writer.template.Execute(buf, TemplateData{
					CheckResult: check,
					FinishTime:  now.Unix(),
					Date:        now.Format("Mon Jan 02 15:04:05 -0700 2006"),
				})
				if err != nil {
					writer.logger.Error(err)
					continue
				}
				value := buf.String()
				fmt.Printf("Check: %+v\n", value)
			}
		}
		writer.logger.Info("Writer ended")
	}()
}

func (writer *Writer) Emit(resultChecks []shared.CheckResult) error {
	defer func() {
		recover()
	}()
	writer.writerChan <- resultChecks
	return nil
}

func (writer *Writer) String() string {
	return "writer"
}

func (writer *Writer) Stop() {
	if atomic.CompareAndSwapUintptr(&writer.isShuttingDown, 0, 1) {
		close(writer.writerChan)
	}
}

func (writer *Writer) WaitForShutdown() {
	writer.wg.Wait()
}

func (writer *Writer) Start() {
	writer.spawnWriter()
}

func NewWriter(logger *logrus.Logger) (*Writer, error) {

	checkTemplate := `### NMP Check ###
latency=0
start_time={{ .FinishTime }}.0
finish_time={{ .FinishTime }}.0
# Time: {{ .Date -}}
{{ with .CheckResult }}
host_name={{ .Hostname }}
{{ if (eq .Type "service") }}service_description={{ .ServiceName }}{{ end }}
check_type=1
early_timeout=1
exited_ok=1
return_code={{ .Code }}
output={{ .Output }}
{{ end }}
`
	t, err := template.New("nagios writter").Parse(checkTemplate)
	if err != nil {
		return nil, err
	}

	writer := &Writer{
		logger:         logger,
		wg:             sync.WaitGroup{},
		writerChan:     make(chan []shared.CheckResult),
		isShuttingDown: 0,
		template:       t,
	}
	return writer, nil
}
