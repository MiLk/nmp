package main

import (
	"github.com/milk/nmp"
	"github.com/milk/nmp/fluentd"
	"github.com/Sirupsen/logrus"
)

func main() {
	workerSet := nmp.NewWorkerSet()

	log := logrus.New()

	output := &fluentd.TestOutput{}

	input, err := fluentd.NewForwardInput(log, "0.0.0.0:24224", output)
	if err != nil {
		log.Fatal(err.Error())
		return
	}
	workerSet.Add(input)

	hb, err := fluentd.NewForwardHeartbeatInput(log, "0.0.0.0:24224", output)
	if err != nil {
		log.Fatal(err.Error())
		return
	}
	workerSet.Add(hb)

	signalHandler := NewSignalHandler(workerSet)

	input.Start()
	hb.Start()
	signalHandler.Start()

	for _, worker := range workerSet.Slice() {
		worker.WaitForShutdown()
	}
	log.Info("Shutting down...")
}
