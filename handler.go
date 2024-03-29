package logging

import (
	"fmt"
	"io"
	"os"
	"sync"
)

// Handler implementers are sent messages by their owning Logger objects to
// handle however they see fit.  They may ignore the message based on their
// level.
type Handler interface {
	// SetFormatter sets the Formatter to be used for this handler.  Handlers
	// only have one formatter at a time.
	SetFormatter(formatter Formatter)
	// SetLevel sets the logging level that this handler is interested in.
	// Handlers are still given every event that gets to the logger, but they
	// can filter events to a certain level within their Emit methods.
	SetLevel(level Level)
	// Emit is how a Logger feeds its handlers.  Ever event that a logger gets
	// is passed into the Emit method of every Handler.  Handlers must not
	// modify the event because it is shared between the other handlers.
	Emit(event *Event)
}

// HandlerCommon is a struct that contains some common Handler state.
type HandlerCommon struct {
	formatter Formatter
	level     Level
}

// Formatter implements the Handler interface.
func (hc HandlerCommon) Formatter() Formatter {
	return hc.formatter
}

// SetFormatter implements the Handler interface.
func (hc *HandlerCommon) SetFormatter(formatter Formatter) {
	hc.formatter = formatter
}

// Level implements the Handler interface.
func (hc HandlerCommon) Level() Level {
	return hc.level
}

// SetLevel implements the Handler interface.
func (hc *HandlerCommon) SetLevel(level Level) {
	hc.level = level
}

// EmitFuncHandler is a Handler that delegates emitting events to a
// EmitFunc
type EmitFuncHandler struct {
	EmitFunc func(e *Event)
	HandlerCommon
}

// HandlerFromEmitFunc creates a handler from an Emit function
func HandlerFromEmitFunc(f func(e *Event)) Handler {
	return &EmitFuncHandler{EmitFunc: f}
}

// Emit delegates to EmitFuncHandler.EmitFunc
func (h EmitFuncHandler) Emit(e *Event) { h.EmitFunc(e) }

// HandlerOption configures a handler
type HandlerOption func(h Handler) error

// HandlerFormatter sets the handler's formatter
func HandlerFormatter(f Formatter) HandlerOption {
	return func(h Handler) error {
		h.SetFormatter(f)
		return nil
	}
}

// HandlerLevel configures a handler's logging level
func HandlerLevel(lvl Level) HandlerOption {
	return func(h Handler) error {
		h.SetLevel(lvl)
		return nil
	}
}

// ConsoleHandler implements the Handler interface by logging events to the
// console.
type ConsoleHandler struct {
	HandlerCommon
}

// Emit implements the Handler interface.
func (ch ConsoleHandler) Emit(event *Event) {
	if event.Level >= ch.level {
		fmt.Fprint(os.Stderr, ch.formatter.Format(event))
	}
}

// WriterHandler implements the Handler interface by writing events into an
// underlying io.Writer implementation.  Access to the writer is synchronized
// with a sync.Locker.
type WriterHandler struct {
	HandlerCommon

	// L is the mutex that protects the writer from concurrent access.
	L sync.Locker
	w io.Writer
}

// LockHandler is protected with a sync.Locker
type LockHandler struct {
	sync.Locker
	Handler
}

// Emit an event to the underlying Handler
func (h LockHandler) Emit(e *Event) {
	h.Lock()
	defer h.Unlock()
	h.Handler.Emit(e)
}

type dummyLock struct{}

func (dummyLock) Lock()   {}
func (dummyLock) Unlock() {}

// NewWriterHandler creates a new WriterHandler with an optional lock to
// synchronize access to the writer.  If nil, no locking on the writer is
// performed.
func NewWriterHandler(w io.Writer, lock sync.Locker) *WriterHandler {
	if lock == nil {
		lock = dummyLock{}
	}
	return &WriterHandler{
		HandlerCommon: HandlerCommon{},
		L:             lock,
		w:             w,
	}
}

// Emit implements the Handler interface.
func (wh *WriterHandler) Emit(event *Event) {
	wh.L.Lock()
	defer wh.L.Unlock()
	if event.Level >= wh.level {
		_, err := fmt.Fprint(wh.w, wh.formatter.Format(event))
		if err != nil {
			panic(err)
		}
	}
}
