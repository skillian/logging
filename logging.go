package logging

import (
	"strconv"
	"time"
)

type Level int

const (
	DebugLevel Level = 10 * (1 + iota)
	InfoLevel
	WarnLevel
	ErrorLevel
	FatalLevel

	WarningLevel = WarnLevel
)

var (
	levelnames = map[Level]string{
		DebugLevel: "Debug",
		InfoLevel: "Info",
		WarnLevel: "Warning",
		ErrorLevel: "Error",
	}
)

func (L Level) String() string {
	if s, ok := levelnames[L]; ok {
		return s
	} else {
		return strconv.Itoa(int(L))
	}
}

type Event struct {
	Name string
	Time time.Time
	Level Level
	Msg string
	Args []interface{}
	FuncName string
	File string
	Line int
}

type Handler interface {
	SetFormatter(formatter Formatter)
	SetLevel(level Level)
	Emit(event *Event)
}

type Formatter interface {
	Format(event *Event) string
}
