# logging
Yet another logging library for Golang.  This one is similar to Python's logging module.

## Usage

### Initialization

To initialize a logger, do this:

```
logger := logging.GetLogger("MyLogger")
handler := new(logging.ConsoleHandler)
handler.SetFormatter(logging.DefaultFormatter{})
handler.SetLevel(logging.DebugLevel)
logger.AddHandler(handler)
logger.SetLevel(logging.DebugLevel)
```

To get an overview of the different types, check out the `logging.go` file; it specifies the Handler and Formatter protocols.

### Logging

Similar to Python's logging, loggers have the following methods:
```
logger.Debug
logger.Info
logger.Warn
logger.Error
```
They do not have an exception call because Go obviously doesn't have exceptions.  All of the methods have numbered versions like this:
```
logger.Debug0
logger.Debug1
logger.Debug2
logger.Debug3
logger.Debug4
```
This is to pass an explicit number of message arguments so an `[]interface{}` slice isn't allocated for every log call (for convenience the non-suffixed version of each method uses a varargs-style `[]interface{}` slice).

I wrote this library with allocations in mind.  I try to effectively use `sync.Pool`s to keep old `Event`s and `Event.Args` `[]interface{}` slices cached to prevent allocations wherever possible.  At this point, the only allocations that I am aware of are the varargs of the non-numbered `Debug`, `Info`, etc. methods (or if using a numbered method but there's no existing slice available in the cache), and the Event objects themselves (again if none are available in the pool).

This is my first Go project that actually does anything, so please let me know if there are any bugs or design antipatterns, flaws, or "non-idiomatic Go" in the design; I would appreciate feedback from anyone with Golang experience!
