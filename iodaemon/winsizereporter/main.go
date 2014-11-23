package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/kr/pty"
)

func main() {
	winsize := make(chan os.Signal, 1)

	signal.Notify(winsize, syscall.SIGWINCH)

	printSize()

	<-winsize

	printSize()

	os.Exit(0)
}

func printSize() {
	rows, cols, err := pty.Getsize(os.Stdin)
	if err != nil {
		log.Fatalln("failed to get window size:", err)
	}

	fmt.Printf("rows: %d, cols: %d\n", rows, cols)
}
