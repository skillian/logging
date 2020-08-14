package logging

import "context"

// LoggerFromContext attempts to retrieve a logger associated with a context.
func LoggerFromContext(ctx context.Context) (*Logger, bool) {
	v := ctx.Value(loggerContextKey{})
	L, ok := v.(*Logger)
	return L, ok
}
