package logging

import (
	"runtime"
	"sync"
	"time"
)

type logPool struct {
	// ArgsPools holds Pools of Event Args slices of specific sizes to
	// reduce allocations.
	argsPools [4]sync.Pool
	logEventPool sync.Pool
}

type Logger struct {
	name string
	level Level
	handlers []Handler
	pools logPool
}

var (
	gLoggersLock = sync.Mutex{}
	gLoggers = make(map[string]*Logger)
)

func GetLogger(name string) *Logger {
	gLoggersLock.Lock()
	defer gLoggersLock.Unlock()
	if L, ok := gLoggers[name]; ok {
		return L
	} else {
		L = createLogger(name)
		gLoggers[name] = L
		return L
	}
}

func createLogger(name string) *Logger {
	L := &Logger{name: name}
	L.pools.logEventPool.New = logEventAllocator
	for i := 0; i < len(L.pools.argsPools); i++ {
		L.pools.argsPools[i].New = getInterfaceSliceAllocator(i + 1)
	}
	return L
}

// AddHandler adds a logging handler to the logger.
// Handlers cannot be removed from a logger.
func (L *Logger) AddHandler(h Handler) {
	L.handlers = append(L.handlers, h)
}

// Level gets the logger's logging level.  Events below the
// logger's Level threshold won't be logged to the Handlers.
func (L *Logger) Level() Level {
	return L.level
}

// SetLevel sets the logger's logging level.
func (L *Logger) SetLevel(level Level) {
	L.level = level
}

// LogEvent emits the event to its handlers and then consumes the event.
// The event must not be used after a call to LogEvent; it is pooled for
// future use and its values will be overwritten.
func (L *Logger) LogEvent(event *Event) {
	if event.Level >= L.level {
		for _, h := range L.handlers {
			h.Emit(event)
		}
	}
	// Pooling the event might not be a good idea if this logger didn't
	// handle it.  For now, you should not try to log the same event
	// to multiple loggers
	L.poolEvent(event)
}

//
// internal log calls:
//

func (L *Logger) log(level Level, msg string, args []interface{}) {
	L.LogEvent(L.createEventFromCaller(level, msg, args, 2))
}

func (L *Logger) log0(level Level, msg string) {
	L.LogEvent(L.createEvent0FromCaller(level, msg, 2))
}

func (L *Logger) log1(level Level, msg string, arg0 interface{}) {
	L.LogEvent(L.createEvent1FromCaller(level, msg, arg0, 2))
}

func (L *Logger) log2(level Level, msg string, arg0, arg1 interface{}) {
	L.LogEvent(L.createEvent2FromCaller(level, msg, arg0, arg1, 2))
}

func (L *Logger) log3(level Level, msg string, arg0, arg1, arg2 interface{}) {
	L.LogEvent(L.createEvent3FromCaller(level, msg, arg0, arg1, arg2, 2))
}

func (L *Logger) log4(level Level, msg string, arg0, arg1, arg2, arg3 interface{}) {
	L.LogEvent(L.createEvent4FromCaller(level, msg, arg0, arg1, arg2, arg3, 2))
}

//
// Logs:
//

func (L *Logger) Log(level Level, msg string, args... interface{}) {
	L.log(level, msg, args)
}

func (L *Logger) Log0(level Level, msg string) {
	L.log0(level, msg)
}

func (L *Logger) Log1(level Level, msg string, arg0 interface{}) {
	L.log1(level, msg, arg0)
}

func (L *Logger) Log2(level Level, msg string, arg0, arg1 interface{}) {
	L.log2(level, msg, arg0, arg1)
}

func (L *Logger) Log3(level Level, msg string, arg0, arg1, arg2 interface{}) {
	L.log3(level, msg, arg0, arg1, arg2)
}

func (L *Logger) Log4(level Level, msg string, arg0, arg1, arg2, arg3 interface{}) {
	L.log4(level, msg, arg0, arg1, arg2, arg3)
}

//
// Debugs:
//

func (L *Logger) Debug(msg string, args... interface{}) {
	L.log(DebugLevel, msg, args)
}

func (L *Logger) Debug0(msg string) {
	L.log0(DebugLevel, msg)
}

func (L *Logger) Debug1(msg string, arg0 interface{}) {
	L.log1(DebugLevel, msg, arg0)
}

func (L *Logger) Debug2(msg string, arg0, arg1 interface{}) {
	L.log2(DebugLevel, msg, arg0, arg1)
}

func (L *Logger) Debug3(msg string, arg0, arg1, arg2 interface{}) {
	L.log3(DebugLevel, msg, arg0, arg1, arg2)
}

func (L *Logger) Debug4(msg string, arg0, arg1, arg2, arg3 interface{}) {
	L.log4(DebugLevel, msg, arg0, arg1, arg2, arg3)
}

//
// Infos:
//

func (L *Logger) Info(msg string, args... interface{}) {
	L.log(InfoLevel, msg, args)
}

func (L *Logger) Info0(msg string) {
	L.log0(InfoLevel, msg)
}

func (L *Logger) Info1(msg string, arg0 interface{}) {
	L.log1(InfoLevel, msg, arg0)
}

func (L *Logger) Info2(msg string, arg0, arg1 interface{}) {
	L.log2(InfoLevel, msg, arg0, arg1)
}

func (L *Logger) Info3(msg string, arg0, arg1, arg2 interface{}) {
	L.log3(InfoLevel, msg, arg0, arg1, arg2)
}

func (L *Logger) Info4(msg string, arg0, arg1, arg2, arg3 interface{}) {
	L.log4(InfoLevel, msg, arg0, arg1, arg2, arg3)
}

//
// Warns:
//

func (L *Logger) Warn(msg string, args... interface{}) {
	L.log(WarnLevel, msg, args)
}

func (L *Logger) Warn0(msg string) {
	L.log0(WarnLevel, msg)
}

func (L *Logger) Warn1(msg string, arg0 interface{}) {
	L.log1(WarnLevel, msg, arg0)
}

func (L *Logger) Warn2(msg string, arg0, arg1 interface{}) {
	L.log2(WarnLevel, msg, arg0, arg1)
}

func (L *Logger) Warn3(msg string, arg0, arg1, arg2 interface{}) {
	L.log3(WarnLevel, msg, arg0, arg1, arg2)
}

func (L *Logger) Warn4(msg string, arg0, arg1, arg2, arg3 interface{}) {
	L.log4(WarnLevel, msg, arg0, arg1, arg2, arg3)
}

//
// Errors:
//

func (L *Logger) Error(msg string, args... interface{}) {
	L.log(ErrorLevel, msg, args)
}

func (L *Logger) Error0(msg string) {
	L.log0(ErrorLevel, msg)
}

func (L *Logger) Error1(msg string, arg0 interface{}) {
	L.log1(ErrorLevel, msg, arg0)
}

func (L *Logger) Error2(msg string, arg0, arg1 interface{}) {
	L.log2(ErrorLevel, msg, arg0, arg1)
}

func (L *Logger) Error3(msg string, arg0, arg1, arg2 interface{}) {
	L.log3(ErrorLevel, msg, arg0, arg1, arg2)
}

func (L *Logger) Error4(msg string, arg0, arg1, arg2, arg3 interface{}) {
	L.log4(ErrorLevel, msg, arg0, arg1, arg2, arg3)
}

//
// CreateEvents
//

func (L *Logger) createEventFromCaller(level Level, msg string, args []interface{}, caller int) *Event {
	pc, file, line, _ := runtime.Caller(caller + 1)
	f := runtime.FuncForPC(pc)
	var funcname string
	if f == nil {
		funcname = ""
	} else {
		funcname = f.Name()
	}
	return L.CreateEvent(time.Now(), level, msg, args, funcname, file, line)
}

func (L *Logger) CreateEvent(time_ time.Time, level Level, msg string, args []interface{}, funcname, file string, line int) *Event {
	event := L.getOrCreateEvent()
	event.Name = L.name
	event.Time = time_
	event.Level = level
	event.Msg = msg
	event.Args = args
	event.FuncName = funcname
	event.File = file
	event.Line = line
	return event
}

func (L *Logger) createEvent0FromCaller(level Level, msg string, caller int) *Event {
	return L.createEventFromCaller(level, msg, nil, caller + 1)
}

func (L *Logger) createEvent1FromCaller(level Level, msg string, arg0 interface{}, caller int) *Event {
	s := L.getArgsSlice(1)
	s[0] = arg0
	return L.createEventFromCaller(level, msg, s, caller + 1)
}

func (L *Logger) createEvent2FromCaller(level Level, msg string, arg0, arg1 interface{}, caller int) *Event {
	s := L.getArgsSlice(2)
	s[0] = arg0
	s[1] = arg1
	return L.createEventFromCaller(level, msg, s, caller + 1)
}

func (L *Logger) createEvent3FromCaller(level Level, msg string, arg0, arg1, arg2 interface{}, caller int) *Event {
	s := L.getArgsSlice(3)
	s[0] = arg0
	s[1] = arg1
	s[2] = arg2
	return L.createEventFromCaller(level, msg, s, caller + 1)
}

func (L *Logger) createEvent4FromCaller(level Level, msg string, arg0, arg1, arg2, arg3 interface{}, caller int) *Event {
	s := L.getArgsSlice(4)
	s[0] = arg0
	s[1] = arg1
	s[2] = arg2
	s[3] = arg3
	return L.createEventFromCaller(level, msg, s, caller + 1)
}

func (L *Logger) getOrCreateEvent() *Event {
	// TODO: look into not using sync.Pool.New
	return L.pools.logEventPool.Get().(*Event)
}

func (L *Logger) getArgsSlice(length int) []interface{} {
	index := length - 1
	if index < len(L.pools.argsPools) {
		return L.pools.argsPools[index].Get().([]interface{})
	} else {
		return make([]interface{}, length)
	}
}

func (L *Logger) poolEvent(event *Event) {
	args := event.Args
	event.Args = nil
	L.pools.logEventPool.Put(event)
	L.poolArgsSlice(args)
}

func (L *Logger) poolArgsSlice(s []interface{}) {
	if len(s) > 0 {
		index := len(s) - 1
		if index < len(L.pools.argsPools) {
			L.pools.argsPools[index].Put(s)
		}
	}
}

//
// Allocators
//

func logEventAllocator() interface{} {
	return new(Event)
}

func getInterfaceSliceAllocator(size int) func () interface{} {
	return func () interface{} {
		return make([]interface{}, size)
	}
}
