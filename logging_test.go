package logging

import (
	"testing"
)

func getLogger() *Logger {
	logger := GetLogger("logging")
	handler := &ConsoleHandler{}
	handler.SetFormatter(DefaultFormatter{})
	handler.SetLevel(DebugLevel)
	logger.AddHandler(handler)
	logger.SetLevel(DebugLevel)
	return logger
}

func TestLogging(t *testing.T) {
	t.Parallel()

	L := getLogger()

	L.Debug0("debug without parameters")
	L.Info1("info with 1 parameter: %d", 1)
	L.Warn2("Warning with 2 parameters: %d, %d", 1, 2)
	L.Error3("Error with 3 parameters: %d, %d, %d", 1, 2, 3)
	L.Info4("Important info with 4 parameters: %d, %d, %d, %d", 1, 2, 3, 4)
	L.Info("Important info with a parameter slice: %d", 42)
}

