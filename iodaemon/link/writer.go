package link

import (
	"encoding/gob"
	"net"
	"syscall"

	"github.com/cloudfoundry-incubator/garden"
)

type Input struct {
	Data       []byte
	EOF        bool
	WindowSize *WindowSize
	Signal     *int
}

type WindowSize struct {
	Columns int
	Rows    int
}

type Writer struct {
	enc *gob.Encoder
}

func NewWriter(conn net.Conn) *Writer {
	return &Writer{enc: gob.NewEncoder(conn)}
}

func (w *Writer) Write(d []byte) (int, error) {
	err := w.enc.Encode(Input{Data: d})
	if err != nil {
		return 0, err
	}

	return len(d), nil
}

func (w *Writer) Close() error {
	return w.enc.Encode(Input{EOF: true})
}

func (w *Writer) SetWindowSize(cols, rows int) error {
	return w.enc.Encode(Input{
		WindowSize: &WindowSize{
			Columns: cols,
			Rows:    rows,
		},
	})
}

func (w *Writer) SendSignal(signal garden.Signal) error {
	var sig int

	switch signal {
	case garden.SignalTerminate:
		sig = int(syscall.SIGTERM)
	case garden.SignalKill:
		sig = int(syscall.SIGKILL)
	default:
		sig = 42
	}

	return w.enc.Encode(Input{
		Signal: &sig,
	})
}
