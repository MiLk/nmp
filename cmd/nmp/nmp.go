package main

import (
	"github.com/milk/nmp"
	"github.com/milk/nmp/fluentd"
	"github.com/Sirupsen/logrus"
    "github.com/milk/nmp/collectd"
    "github.com/milk/nmp/config"
)

func main() {
    log := logrus.New()

    _config, err := config.Read()
    if err != nil {
        log.Fatal(err.Error())
        return
    }

	workerSet := nmp.NewWorkerSet()

    checker, err := collectd.NewChecker(log, _config.Checks)
    if err != nil {
        log.Fatal(err.Error())
        return
    }
    workerSet.Add(checker)

    collectdTransformer, err := collectd.NewTransformer(log, []string{"collectd"}, checker)
    if err != nil {
        log.Fatal(err.Error())
        return
    }
    workerSet.Add(collectdTransformer)

	fluentdForwarderInput, err := fluentd.NewForwardInput(log, "0.0.0.0:24224", collectdTransformer)
	if err != nil {
		log.Fatal(err.Error())
		return
	}
	workerSet.Add(fluentdForwarderInput)

	fluentdHeartbeatInput, err := fluentd.NewForwardHeartbeatInput(log, "0.0.0.0:24224")
	if err != nil {
		log.Fatal(err.Error())
		return
	}
	workerSet.Add(fluentdHeartbeatInput)

	signalHandler := nmp.NewSignalHandler(workerSet)

    checker.Start()
    collectdTransformer.Start()
    fluentdForwarderInput.Start()
    fluentdHeartbeatInput.Start()
	signalHandler.Start()

	for _, worker := range workerSet.Slice() {
		worker.WaitForShutdown()
	}
	log.Info("Shutting down...")
}
