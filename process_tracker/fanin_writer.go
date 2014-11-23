package process_tracker

import (
	"errors"
	"io"
	"sync"
)

type faninWriter struct {
	w      io.WriteCloser
	closed bool
	writeL sync.Mutex

	hasSink chan struct{}
}

func (w *faninWriter) Write(data []byte) (int, error) {
	<-w.hasSink

	w.writeL.Lock()

	if w.closed {
		return 0, errors.New("write after close")
	}

	defer w.writeL.Unlock()

	return w.w.Write(data)
}

func (w *faninWriter) Close() error {
	<-w.hasSink

	w.writeL.Lock()

	if w.closed {
		return errors.New("closed twice")
	}

	w.closed = true

	defer w.writeL.Unlock()

	return w.w.Close()
}

func (w *faninWriter) AddSink(sink io.WriteCloser) {
	w.w = sink
	close(w.hasSink)
}

func (w *faninWriter) AddSource(source io.Reader) {
	go func() {
		_, err := io.Copy(w, source)
		if err == nil {
			w.Close()
		}
	}()
}
