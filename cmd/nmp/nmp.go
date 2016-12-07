package main

import (
	"github.com/Sirupsen/logrus"

	"github.com/milk/nmp"
	"github.com/milk/nmp/collectd"
	"github.com/milk/nmp/config"
	"github.com/milk/nmp/fluentd"
	"github.com/milk/nmp/nagios"
)

func main() {
	log := logrus.New()

	_config, err := config.Read()
	if err != nil {
		log.Fatal(err.Error())
		return
	}

	workerSet := nmp.NewWorkerSet()

	writer, err := nagios.NewWriter(log, _config.CheckResultsDir)
	if err != nil {
		log.Fatal(err.Error())
		return
	}
	workerSet.Add(writer)

	transformer, err := nagios.NewTransformer(log, writer)
	if err != nil {
		log.Fatal(err.Error())
		return
	}
	workerSet.Add(transformer)

	checker, err := collectd.NewChecker(log, _config.Checks, transformer)
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

	writer.Start()
	transformer.Start()
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
