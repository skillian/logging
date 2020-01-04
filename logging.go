package logging

import (
	"strconv"
	"strings"
	"time"
)

// Level ranks the severity of a logging message
type Level int

const (
	// VerboseLevel includes the most logging information.  As the
	// definition of verbose implies, this level will produce more
	// information than should be needed for all but the most obscure
	// troubleshooting.
	VerboseLevel Level = (10 * iota) - 30

	// DebugLevel is the lowest severity level intended for any practical
	// use.  As its name implies, it should be restricted to debugging.
	DebugLevel

	// InfoLevel is for tracking informational messages that are not actual
	// problems
	InfoLevel

	// WarnLevel is the default severity level and indicates non-critical
	// problems that the program will work around or otherwise recover from
	// but should be made aware to the user.
	WarnLevel

	// ErrorLevel represents errors that the current call-stack / goroutine
	// cannot work around and will terminate its execution but not necessarily
	// the execution of other goroutines.
	ErrorLevel

	// FatalLevel represents an issue that will cause the entire executable to
	// stop running immediately.  It is essentially an assertion.
	FatalLevel

	// WarningLevel is just an alias for the WarnLevel.
	WarningLevel = WarnLevel
)

var (
	levelValueToName = map[Level]string{
		VerboseLevel: "Verbose",
		DebugLevel:   "Debug",
		InfoLevel:    "Info",
		WarnLevel:    "Warning",
		ErrorLevel:   "Error",
	}

	levelNameToValue = map[string]Level{
		"verbose": VerboseLevel,
		"debug":   DebugLevel,
		"info":    InfoLevel,
		"warn":    WarnLevel,
		"warning": WarningLevel,
		"error":   ErrorLevel,
	}
)

// ParseLevel takes an log level name as a string and returns the Level value.
func ParseLevel(name string) (Level, bool) {
	if value, ok := levelNameToValue[strings.ToLower(name)]; ok {
		return value, true
	}
	return 0, false
}

// String returns a string value for the logging level
func (L Level) String() string {
	if s, ok := levelValueToName[L]; ok {
		return s
	}
	return strconv.Itoa(int(L))
}

// Event holds an event that is being handled within logging's internals
// but is publicly available in case it needs to be expanded upon by
// others' packages to wrap this one.
type Event struct {
	// Name tracks the name of the logger that the event was created within.
	// The name field is used by the Formatter and then the Handler to format
	// and write the event, respectively.
	Name string

	// Time stores the time that the event occurred.
	Time time.Time

	// Level stores the event's logging level so that handlers within the
	// logger can chose whether or not to log the event based on their
	// configuration.

	Level Level

	// Msg stores the unformatted message within the event.  Only the
	// formatters used in the handlers that "want" the message format them
	// with the Event's Args.
	Msg string

	// Args holds the formatting parameters to the message in Msg.
	Args []interface{}

	// FuncName holds the name of the function where the event came from.
	// Imagine that.
	FuncName string

	// File holds the filename of the function where the event happened.
	File string

	// Line holds the line number within the file where the error occurred.
	Line int
}
