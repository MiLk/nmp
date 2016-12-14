package main

import (
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"

	"github.com/Sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/milk/nmp"
	"github.com/milk/nmp/collectd"
	"github.com/milk/nmp/config"
	"github.com/milk/nmp/fluentd"
	"github.com/milk/nmp/nagios"
)

func startProfiler() {
	http.ListenAndServe(":6060", http.DefaultServeMux)
}

var RootCmd = &cobra.Command{
	Use:   "nmp",
	Short: "Nagios Metrics Processor",
	Long:  `NMP (Nagios Metrics Processor) is a simple metrics collector for use with Nagios.`,
	Run:   nmpCommand,
}

func init() {
	RootCmd.PersistentFlags().StringP("config", "c", "config.hcl", "Path to the configuration file to use")
	RootCmd.PersistentFlags().Bool("pprof", false, "Enable profiling with pkg/net/pprof")
}

func main() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
}

func nmpCommand(cmd *cobra.Command, args []string) {
	log := logrus.New()

	pprof, err := cmd.Flags().GetBool("pprof")
	if err != nil {
		log.Fatal(err.Error())
		return
	}

	configFile, err := cmd.Flags().GetString("config")
	if err != nil {
		log.Fatal(err.Error())
		return
	}

	if pprof {
		go startProfiler()
	}

	_config, err := config.Read(configFile)
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
