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

type BaseHandler struct {
	formatter Formatter
	level Level
}

func (bh BaseHandler) Formatter() Formatter {
	return bh.formatter
}

func (bh *BaseHandler) SetFormatter(formatter Formatter) {
	bh.formatter = formatter
}

func (bh BaseHandler) Level() Level {
	return bh.level
}

func (bh *BaseHandler) SetLevel(level Level) {
	bh.level = level
}

type ConsoleHandler struct {
	BaseHandler
}

func (ch ConsoleHandler) Emit(event *Event) {
	if event.Level >= ch.level {
		var w io.Writer
		if event.Level >= ErrorLevel {
			w = os.Stderr
		} else {
			w = os.Stdout
		}
		fmt.Fprintf(w, ch.formatter.Format(event))
	}
}

type WriterHandler struct {
	w io.Writer
	m sync.Mutex
	BaseHandler
}

func (wh *WriterHandler) Emit(event *Event) {
	wh.m.Lock()
	defer wh.m.Unlock()
	if event.Level >= wh.level {
		fmt.Fprintf(wh.w, wh.formatter.Format(event))
	}
}
