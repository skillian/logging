package logging

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Formatter objects are used by their Handlers to format events into a single
// string that the Handler can write (or not write) somewhere else.  The same
// Formatter object should not be used by multiple Handlers.
type Formatter interface {
	// Format takes the event object and formats it into a string for its
	// Handler to do something with.
	Format(event *Event) string
}

// FormatterFunc implements the Formatter interface through a single function.
type FormatterFunc func(e *Event) string

// Format implements Formatter by calling the format message.
func (f FormatterFunc) Format(e *Event) string { return f(e) }

// DefaultFormatter Sprintf's all of the information within its provided Event
// in an arbitrarily decided format that *I* just happen to like.
// Your mileage may vary.
type DefaultFormatter struct{}

// Format returns the event with the following layout:
//
//    yyyy-mm-dd HH:MM:SS:  Level:  LoggerName:  at FuncName in File, line Line:  fmt.Sprintf(Msg, Args...)
func (f DefaultFormatter) Format(event *Event) string {
	year, month, day := event.Time.Date()
	hour, minute, second := event.Time.Clock()
	levelString := event.Level.String()
	rightAlignedLevel := strings.Repeat(" ", 8-len(levelString)) + levelString
	return fmt.Sprintf(
		"%d-%02d-%02d %02d:%02d:%02d:  %s:  %s:  at %s in %s, line %d:\n\t%s\n",
		year, month, day, hour, minute, second,
		rightAlignedLevel, event.Name, event.FuncName,
		filepath.Base(event.File), event.Line,
		fmt.Sprintf(event.Msg, event.Args...))
}
