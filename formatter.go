package logging

import (
	"fmt"
	"strings"
)

type DefaultFormatter struct {}

func (f DefaultFormatter) Format(event *Event) string {
	year, month, day := event.Time.Date()
	hour, minute, second := event.Time.Clock()
	levelString := event.Level.String()
	rightAlignedLevel := strings.Repeat(" ", 8 - len(levelString)) + levelString
	return fmt.Sprintf(
		fmt.Sprintf(
			"%d-%02d-%02d %02d:%02d:%02d:  %s:  %s:  at %s in %s, line %d:  %s\n",
			year, month, day, hour, minute, second,
			rightAlignedLevel, event.Name, event.FuncName, event.File, event.Line,
			event.Msg),
		event.Args...)
}