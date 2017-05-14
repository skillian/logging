package logging

import (
	"fmt"
	"io"
	"os"
)

//
// Handlers:
//

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
	BaseHandler
}

func (wh WriterHandler) Emit(event *Event) {
	if event.Level >= wh.level {
		fmt.Fprintf(wh.w, wh.formatter.Format(event))
	}
}
