package main

import (
	"flag"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

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

var debugListenAddress = flag.String(
	"debugListenAddress",
	"127.0.0.1",
	"address for the pprof debugger listen on",
)

var debugListenPort = flag.Int(
	"debugListenPort",
	7776,
	"port for the pprof debugger to listen on",
)

var containerGraceTime = flag.Duration(
	"containerGraceTime",
	5*time.Minute,
	"time after which to destroy idle containers",
)

var containersDir = flag.String(
	"depot",
	"./containers",
	"directory in which to store containers",
)

func main() {
	flag.Parse()

	logger := lager.NewLogger("houdini")
	logger.RegisterSink(lager.NewWriterSink(os.Stdout, lager.DEBUG))

	depot, err := filepath.Abs(*containersDir)
	if err != nil {
		logger.Fatal("failed-to-determine-depot-dir", err)
	}

	backend := houdini.NewBackend(depot)

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

	debugListenAddr := fmt.Sprintf("%s:%d", *debugListenAddress, *debugListenPort)

	err = http.ListenAndServe(debugListenAddr, nil)
	if err != nil {
		logger.Fatal("failed-to-start-debug-server", err)
	}

	select {}
}
