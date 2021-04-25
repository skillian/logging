package logging

import (
	"fmt"

	//	"path"
	"path/filepath"
	"testing"
)

type testingHandler struct {
	HandlerCommon
	Testing *testing.T
}

func (h *testingHandler) Emit(e *Event) {
	h.Testing.Helper()
	msg := h.Formatter().Format(e)
	var fn func(args ...interface{})
	switch {
	case e.Level < ErrorLevel:
		fn = h.Testing.Log
	case e.Level < FatalLevel:
		fn = h.Testing.Error
	default:
		fn = h.Testing.Fatal
	}
	fn(msg)
}

// TestingHandler lets you register the given testing.T with the logger
// so that debug messages are written to the testing.T.  It returns a
// function that when called, no longer tries to log to the testing.T.
func TestingHandler(logger *Logger, t *testing.T, options ...HandlerOption) func() {
	h := new(testingHandler)
	for _, opt := range options {
		if err := opt(h); err != nil {
			logger.Error("error initializing %v: %v", h, err)
			return func() {}
		}
	}
	if h.Formatter() == nil {
		h.SetFormatter(testingFormatter{})
	}
	h.Testing = t
	logger.AddHandler(h)
	return func() {
		logger.RemoveHandlers(h)
	}
}

type testingFormatter struct{}

func (testingFormatter) Format(e *Event) string {
	//funcname := path.Base(e.FuncName)
	filename := filepath.Base(e.File)
	return fmt.Sprintf(
		"%s:%d:\t%s", filename, e.Line,
		fmt.Sprintf(e.Msg, e.Args...),
	)
}
