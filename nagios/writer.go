package nagios

import (
	"fmt"
	"io/ioutil"
	"sync"
	"sync/atomic"

	"github.com/Sirupsen/logrus"
	"os"
)

type Writer struct {
	logger          *logrus.Logger
	wg              sync.WaitGroup
	writerChan      chan string
	isShuttingDown  uintptr
	checkResultsDir string
}

func (writer *Writer) writeToFile(checkResult string) error {
	tmpfile, err := TempFile(writer.checkResultsDir, "c", 6)
	if err != nil {
		return err
	}
	if _, err := tmpfile.Write([]byte(checkResult)); err != nil {
		return err
	}
	if err := tmpfile.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpfile.Name(), 0770); err != nil {
		return err
	}

	filename := fmt.Sprintf("%s.ok", tmpfile.Name())
	err = ioutil.WriteFile(filename, []byte{}, 0770)
	if err != nil {
		return nil
	}

	return nil
}

func (writer *Writer) spawnWriter() {
	writer.logger.Info("Spawning writer")
	writer.wg.Add(1)
	go func() {
		defer func() {
			writer.wg.Done()
		}()
		writer.logger.Info("Writer started")

		for checkResult := range writer.writerChan {
			err := writer.writeToFile(checkResult)
			if err != nil {
				writer.logger.Error(err)
			}
		}
		writer.logger.Info("Writer ended")
	}()
}

func (writer *Writer) Emit(resultCheck string) error {
	defer func() {
		recover()
	}()
	writer.writerChan <- resultCheck
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

func NewWriter(logger *logrus.Logger, checkResultsDir string) (*Writer, error) {

	writer := &Writer{
		logger:          logger,
		wg:              sync.WaitGroup{},
		writerChan:      make(chan string),
		isShuttingDown:  0,
		checkResultsDir: checkResultsDir,
	}
	return writer, nil
}
