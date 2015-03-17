package main

import (
	"flag"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/cloudfoundry-incubator/garden/server"
	"github.com/pivotal-golang/lager"
	"github.com/vito/houdini"
)

var listenNetwork = flag.String(
	"listenNetwork",
	"tcp",
	"how to listen on the address (unix, tcp, etc.)",
)

var listenAddr = flag.String(
	"listenAddr",
	"0.0.0.0:7777",
	"address to listen on",
)

var containerGraceTime = flag.Duration(
	"containerGraceTime",
	0,
	"time after which to destroy idle containers",
)

var containersDir = flag.String(
	"depot",
	"./containers",
	"directory in which to store containers",
)

var skeletonDir = flag.String(
	"skeleton",
	"./skeleton",
	"directory containing container skeleton",
)

func main() {
	flag.Parse()

	logger := lager.NewLogger("houdini")
	logger.RegisterSink(lager.NewWriterSink(os.Stdout, lager.DEBUG))

	depot, err := filepath.Abs(*containersDir)
	if err != nil {
		logger.Fatal("failed-to-determine-depot-dir", err)
	}

	skeleton, err := filepath.Abs(*skeletonDir)
	if err != nil {
		logger.Fatal("failed-to-determine-skeleton-dir", err)
	}

	backend := houdini.NewBackend(depot, skeleton)

	gardenServer := server.New(*listenNetwork, *listenAddr, *containerGraceTime, backend, logger)

	err = gardenServer.Start()
	if err != nil {
		logger.Fatal("failed-to-start-server", err)
	}

	logger.Info("started", lager.Data{
		"network": *listenNetwork,
		"addr":    *listenAddr,
	})

	signals := make(chan os.Signal, 1)

	go func() {
		<-signals
		gardenServer.Stop()
		os.Exit(0)
	}()

	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	select {}
}
