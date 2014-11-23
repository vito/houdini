package main

import (
	"flag"
	"os"
	"time"
)

const USAGE = `usage:

	iomux spawn [-timeout timeout] [-tty] <socket> <path> <args...>:
		spawn a subprocess, making its stdio and exit status available via
		the given socket

	iomux link <socket>:
		attach to a process via the given socket
`

// TODO actually do this
var timeout = flag.Duration(
	"timeout",
	10*time.Second,
	"time duration to wait on an initial link before giving up",
)

var tty = flag.Bool(
	"tty",
	false,
	"spawn child process with a tty",
)

var windowColumns = flag.Int(
	"windowColumns",
	80,
	"initial window columns for the process's tty",
)

var windowRows = flag.Int(
	"windowRows",
	24,
	"initial window rows for the process's tty",
)

var debug = flag.Bool(
	"debug",
	false,
	"emit debugging information beside socket file as .trace",
)

func main() {
	flag.Parse()

	args := flag.Args()

	switch args[0] {
	case "spawn":
		if len(args) < 3 {
			usage()
		}

		spawn(args[1], args[2:], *timeout, *tty, *windowColumns, *windowRows, *debug)

	case "link":
		if len(args) < 2 {
			usage()
		}

		link(args[1])

	default:
		usage()
	}
}

func usage() {
	println(USAGE)
	os.Exit(1)
}
