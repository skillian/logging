package logging

import (
	"context"
	"math/bits"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"golang.org/x/sys/cpu"

	"github.com/skillian/errors"
	"github.com/skillian/unsafereflect"
)

// Logger objects expose methods to log events to added handlers if the event
// exceeds the logger's log level.
type Logger struct {
	parent         *Logger
	handlersUnsafe *[]Handler
	preCallFunc    func()
	flags          logFlags
	name           string
	pools          logPool
}

type logFlags int32

const (
	// noPropagateFlag turns off propagation to parent loggers.
	noPropagateFlag logFlags = 1 << (iota + (8 * unsafe.Sizeof(Level(0))))
	temporaryFlag
)

func (fs *logFlags) cas(old, new logFlags) bool {
	return atomic.CompareAndSwapInt32(
		(*int32)(fs),
		int32(old),
		int32(new),
	)
}

func (fs *logFlags) load() logFlags {
	return logFlags(atomic.LoadInt32((*int32)(fs)))
}

func (fs *logFlags) set(v logFlags) {
	for {
		old := fs.load()
		if fs.cas(old, old|v) {
			return
		}
	}
}

func (fs *logFlags) unset(v logFlags) {
	for {
		old := fs.load()
		if fs.cas(old, old&(^v)) {
			return
		}
	}
}

type logPool struct {
	mu         sync.Mutex
	freeArgs   [][]interface{}
	freeEvents []*Event
}

const cacheLinePadSize = unsafe.Sizeof(cpu.CacheLinePad{})

func (p *logPool) getArgs() (args []interface{}) {
	p.mu.Lock()
	if len(p.freeArgs) == 0 {
		p.freeArgs = append(p.freeArgs[:cap(p.freeArgs)], nil)
		p.freeArgs = p.freeArgs[:cap(p.freeArgs)]
		// ifacePerCacheLine is 8 on 32-bit systems and 4 on
		// 64-bit systems.  We use this later when we
		// allocate a slice of args to align the args to a cache
		// line so that events can be created in parallel and
		// avoid "false sharing."
		const ifacePerCacheLine = int(cacheLinePadSize / (bits.UintSize >> 2))
		// add one so we can manually align to a cache line
		cache := make([]interface{}, len(p.freeArgs)+(ifacePerCacheLine-1))
		pinner := runtime.Pinner{}
		pinner.Pin(&cache[0])
		{
			sd := unsafereflect.SliceDataOf(unsafe.Pointer(&cache))
			data := uintptr(sd.Data)
			inc := uintptr(cacheLinePadSize-(data%cacheLinePadSize)) % cacheLinePadSize
			sd.Data = unsafe.Add(sd.Data, inc)
			sd.Len = len(p.freeArgs)
			sd.Len = len(p.freeArgs)
		}
		pinner.Unpin()
		for i := range p.freeArgs {
			start := i * ifacePerCacheLine
			end := start + ifacePerCacheLine
			p.freeArgs[i] = cache[start:start:end]
		}
	}
	args = p.freeArgs[len(p.freeArgs)-1]
	p.freeArgs = p.freeArgs[:len(p.freeArgs)-1]
	p.mu.Unlock()
	return
}

func (p *logPool) getEvent() (ev *Event) {
	p.mu.Lock()
	{
		if len(p.freeEvents) == 0 {
			p.freeEvents = append(p.freeEvents[:cap(p.freeEvents)], nil)
			p.freeEvents = p.freeEvents[:cap(p.freeEvents)]
			cache := make([]Event, len(p.freeEvents))
			for i := range p.freeEvents {
				p.freeEvents[i] = &cache[i]
			}
		}
		ev = p.freeEvents[len(p.freeEvents)-1]
		p.freeEvents = p.freeEvents[:len(p.freeEvents)-1]
	}
	p.mu.Unlock()
	return
}

func (p *logPool) putEvent(ev *Event) {
	p.mu.Lock()
	{
		for i := range ev.Args {
			ev.Args[i] = nil
		}
		ev.Args = ev.Args[:0]
		p.freeArgs = append(p.freeArgs, ev.Args)
		ev.Args = nil
		p.freeEvents = append(p.freeEvents, ev)
	}
	p.mu.Unlock()
}

var (
	//gLoggersLock = sync.Mutex{}
	//gLoggers     = make(map[string]*Logger)
	loggers = sync.Map{}

	defaultPreCallFunc = func() {}
)

// LoggerOption configures a logger
type LoggerOption func(L *Logger) error

// LoggerHandler adds a handler to the logger.
func LoggerHandler(h Handler, options ...HandlerOption) LoggerOption {
	return func(L *Logger) error {
		for _, o := range options {
			if err := o(h); err != nil {
				return err
			}
		}
		L.AddHandlers(h)
		return nil
	}
}

// LoggerLevel returns a LoggerOption that configures the logger level.
func LoggerLevel(level Level) LoggerOption {
	return func(L *Logger) error {
		L.SetLevel(level)
		return nil
	}
}

// LoggerTemporary configures a logger to be temporary and not cached.
func LoggerTemporary() LoggerOption {
	return func(L *Logger) error {
		if L.name != "" {
			return nil
		}
		L.name = strings.Join([]string{
			L.name,
			"<0x",
			strconv.FormatUint(uint64(uintptr(unsafe.Pointer(L))), 16),
			">",
		}, "")
		return nil
	}
}

// loggerTemporaryData represents LoggerTemporary as a uintptr so that
// GetLogger can check to see if any of its options are LoggerTemporary
var loggerTemporaryData = func() unsafe.Pointer {
	f := LoggerTemporary()
	return *((*unsafe.Pointer)(unsafe.Pointer(&f)))
}()

// LoggerPropagate returns a logger option that configures the logger's
// propagation flag to the given value.
func LoggerPropagate(propagate bool) LoggerOption {
	return func(L *Logger) error {
		L.SetPropagate(propagate)
		return nil
	}
}

// GetLogger retrieves a logger with the given name.  If a logger with that name
// doesn't exist, one is created.  This function is protected by a mutex and
// can be called concurrently.
func GetLogger(name string, options ...LoggerOption) *Logger {
	var k interface{} = name
	var L *Logger
	v, loaded := loggers.Load(k)
	if loaded {
		L = v.(*Logger)
	} else {
		L = createLogger(name)
	}
	var errs error
	temp := false
	for _, opt := range options {
		if err := opt(L); err != nil {
			errs = errors.CreateError(err, nil, errs, 0)
		}
		if !temp {
			optData := *((*unsafe.Pointer)(unsafe.Pointer(&opt)))
			temp = optData == loggerTemporaryData
		}
	}
	if !temp {
		v = L
		v, loaded = loggers.LoadOrStore(k, v)
		if loaded {
			L = v.(*Logger)
		}
	}
	if errs != nil {
		L.LogErr(errs)
	}
	return L
}

// LoggerFromContext gets the logger associated with the given context.  If
// no logger is associated, returns nil (just like how ctx.Value returns nil)
func LoggerFromContext(ctx context.Context) (*Logger, bool) {
	L, ok := ctx.Value((*Logger)(nil)).(*Logger)
	return L, ok
}

func createLogger(name string) *Logger {
	splitAt := strings.LastIndexByte(name, '/')
	var parent *Logger
	if splitAt != -1 {
		parent = GetLogger(name[:splitAt])
	}
	L := &Logger{
		parent:         parent,
		preCallFunc:    defaultPreCallFunc,
		name:           name,
		handlersUnsafe: new([]Handler),
	}
	return L
}

// AddToContext adds the given Logger to the context and returns that new
// context.  If the logger is already in the context, that existing context is
// returned as-is.
func (L *Logger) AddToContext(ctx context.Context) context.Context {
	L2, ok := LoggerFromContext(ctx)
	if ok && L2 == L {
		return ctx
	}
	return context.WithValue(ctx, (*Logger)(nil), L)
}

// AddHandler adds a single logging handler to the logger.
func (L *Logger) AddHandler(h Handler) { L.AddHandlers(h) }

// AddHandlers adds handlers to the logger.  This function is threadsafe.
func (L *Logger) AddHandlers(hs ...Handler) {
	if len(hs) == 0 {
		return
	}
	newHandlers := new([]Handler)
	for {
		oldHandlers := L.handlersPtr()
		newLength := len(*oldHandlers) + len(hs)
		if len(*newHandlers) < newLength {
			*newHandlers = make([]Handler, newLength)
		} else {
			*newHandlers = (*newHandlers)[:newLength]
		}
		copy(*newHandlers, *oldHandlers)
		copy((*newHandlers)[len(*oldHandlers):], hs)
		if L.casHandlers(oldHandlers, newHandlers) {
			return
		}
	}
}

// Handlers returns all the handlers registered with the logger at the time
// this call was made.  Note that by the time the function returns, the set of
// handlers may have changed.  This is meant for debugging.
func (L *Logger) Handlers() []Handler {
	p := L.handlersPtr()
	hs := make([]Handler, len(*p))
	copy(hs, *p)
	return hs
}

func (L *Logger) handlersPtr() *[]Handler {
	addr := (*unsafe.Pointer)(unsafe.Pointer(&L.handlersUnsafe))
	return (*[]Handler)(atomic.LoadPointer(addr))
}

func (L *Logger) casHandlers(old, new *[]Handler) bool {
	addr := (*unsafe.Pointer)(unsafe.Pointer(&L.handlersUnsafe))
	return atomic.CompareAndSwapPointer(
		addr,
		unsafe.Pointer(old),
		unsafe.Pointer(new),
	)
}

// RemoveHandlers removes the given list of handlers from the logger.
func (L *Logger) RemoveHandlers(hs ...Handler) {
	if len(hs) == 0 {
		return
	}
	oldHandlers := L.handlersPtr()
	if len(*oldHandlers) == 0 {
		return
	}
	newHandlers := new([]Handler)
	for {
		*newHandlers = make([]Handler, 0, len(*oldHandlers))
	oldLoop:
		for _, oldH := range *oldHandlers {
			for _, newH := range hs {
				if oldH == newH {
					continue oldLoop
				}
			}
			*newHandlers = append(*newHandlers, oldH)
		}
		if L.casHandlers(oldHandlers, newHandlers) {
			return
		}
		oldHandlers = L.handlersPtr()
	}
}

// EffectiveLevel gets the minimum level of this logger and any of the parents
// it can propagate events to.  Use this if in order to log something, you need
// to do extra work to build some representation of it and you don't want to
// do that unless it's actually going to be logged:
//
//	if logger.EffectiveLevel() <= logging.VerboseLevel {
//		nvps := make([]NameValuePair, len(names))
//		for i, n := range names {
//			nvps[i] = NameValuePair{Name: n, Value: values[i]}
//		}
//		logger.Verbose1("doing work with names and values: %#v", nvps)
//	}
func (L *Logger) EffectiveLevel() Level {
	minLevel := L.Level()
	for L.Propagate() {
		L = L.parent
		if L == nil {
			break
		}
		parentLevel := L.Level()
		if parentLevel < minLevel {
			minLevel = parentLevel
		}
	}
	return minLevel
}

// Level gets the logger's level.
func (L *Logger) Level() Level { return Level(L.flags & levelMask) }

// Name gets the logger's name.
func (L *Logger) Name() string { return L.name }

// SetLevel sets the logging level of the logger.
func (L *Logger) SetLevel(level Level) {
	for {
		oldFlags := L.flags.load()
		newFlags := (oldFlags & ^levelMask) | logFlags(level)
		if L.flags.cas(oldFlags, newFlags) {
			return
		}
	}
}

// Propagate events to the parent logger(s).
func (L *Logger) Propagate() bool {
	return L.flags.load()&noPropagateFlag == 0
}

// SetPropagate toggles propagating events to parent logger(s).
func (L *Logger) SetPropagate(v bool) {
	if v {
		L.flags.unset(noPropagateFlag)
	} else {
		L.flags.set(noPropagateFlag)
	}
}

// LogEvent emits the event to its handlers and then consumes the event.
// The event must not be used after a call to LogEvent; it is pooled for
// future use and its values will be overwritten.
func (L *Logger) LogEvent(event *Event) {
	L.preCallFunc()
	L.doLogEvent(event)
	L.pools.putEvent(event)
}

// doLogEvent is the actual work behind LogEvent.  It is separate from LogEvent
// so parent loggers "know" the event is not theirs to put back into their
// pool(s).
func (L *Logger) doLogEvent(e *Event) {
	L.preCallFunc()
	if e.Level >= L.Level() {
		for _, h := range *L.handlersPtr() {
			h.Emit(e)
		}
	}
	if L.parent != nil && L.Propagate() {
		L.parent.doLogEvent(e)
	}
}

//
// internal log calls:
//

func (L *Logger) log(level Level, msg string, args []interface{}) {
	L.preCallFunc()
	L.LogEvent(L.createEventFromCaller(level, msg, args, 2))
}

func (L *Logger) log0(level Level, msg string) {
	L.preCallFunc()
	L.LogEvent(L.createEvent0FromCaller(level, msg, 2))
}

func (L *Logger) log1(level Level, msg string, arg0 interface{}) {
	L.preCallFunc()
	L.LogEvent(L.createEvent1FromCaller(level, msg, arg0, 2))
}

func (L *Logger) log2(level Level, msg string, arg0, arg1 interface{}) {
	L.preCallFunc()
	L.LogEvent(L.createEvent2FromCaller(level, msg, arg0, arg1, 2))
}

func (L *Logger) log3(level Level, msg string, arg0, arg1, arg2 interface{}) {
	L.preCallFunc()
	L.LogEvent(L.createEvent3FromCaller(level, msg, arg0, arg1, arg2, 2))
}

func (L *Logger) log4(level Level, msg string, arg0, arg1, arg2, arg3 interface{}) {
	L.preCallFunc()
	L.LogEvent(L.createEvent4FromCaller(level, msg, arg0, arg1, arg2, arg3, 2))
}

//
// Logs:
//

// Log an event to the logger.
func (L *Logger) Log(level Level, msg string, args ...interface{}) {
	L.preCallFunc()
	L.log(level, msg, args)
}

// Log0 logs an event with no arguments to the logger.
func (L *Logger) Log0(level Level, msg string) {
	L.preCallFunc()
	L.log0(level, msg)
}

// Log1 logs an event with a single argument to the logger.
func (L *Logger) Log1(level Level, msg string, arg0 interface{}) {
	L.preCallFunc()
	L.log1(level, msg, arg0)
}

// Log2 logs an event with two arguments to the logger.
func (L *Logger) Log2(level Level, msg string, arg0, arg1 interface{}) {
	L.preCallFunc()
	L.log2(level, msg, arg0, arg1)
}

// Log3 logs an event with three arguments to the logger.
func (L *Logger) Log3(level Level, msg string, arg0, arg1, arg2 interface{}) {
	L.preCallFunc()
	L.log3(level, msg, arg0, arg1, arg2)
}

// Log4 logs an event with four arguments to the logger.
func (L *Logger) Log4(level Level, msg string, arg0, arg1, arg2, arg3 interface{}) {
	L.preCallFunc()
	L.log4(level, msg, arg0, arg1, arg2, arg3)
}

//
// Verboses:
//

// Verbose calls Log with the VerboseLevel level.
func (L *Logger) Verbose(msg string, args ...interface{}) {
	L.preCallFunc()
	L.log(VerboseLevel, msg, args)
}

// Verbose0 calls Log0 with the VerboseLevel level.
func (L *Logger) Verbose0(msg string) {
	L.preCallFunc()
	L.log0(VerboseLevel, msg)
}

// Verbose1 calls Log1 with the VerboseLevel level.
func (L *Logger) Verbose1(msg string, arg0 interface{}) {
	L.preCallFunc()
	L.log1(VerboseLevel, msg, arg0)
}

// Verbose2 calls Log2 with the VerboseLevel level.
func (L *Logger) Verbose2(msg string, arg0, arg1 interface{}) {
	L.preCallFunc()
	L.log2(VerboseLevel, msg, arg0, arg1)
}

// Verbose3 calls Log3 with the VerboseLevel level.
func (L *Logger) Verbose3(msg string, arg0, arg1, arg2 interface{}) {
	L.preCallFunc()
	L.log3(VerboseLevel, msg, arg0, arg1, arg2)
}

// Verbose4 calls Log4 with the VerboseLevel level.
func (L *Logger) Verbose4(msg string, arg0, arg1, arg2, arg3 interface{}) {
	L.preCallFunc()
	L.log4(VerboseLevel, msg, arg0, arg1, arg2, arg3)
}

//
// Debugs:
//

// Debug calls Log with the DebugLevel level.
func (L *Logger) Debug(msg string, args ...interface{}) {
	L.preCallFunc()
	L.log(DebugLevel, msg, args)
}

// Debug0 calls Log0 with the DebugLevel level.
func (L *Logger) Debug0(msg string) {
	L.preCallFunc()
	L.log0(DebugLevel, msg)
}

// Debug1 calls Log1 with the DebugLevel level.
func (L *Logger) Debug1(msg string, arg0 interface{}) {
	L.preCallFunc()
	L.log1(DebugLevel, msg, arg0)
}

// Debug2 calls Log2 with the DebugLevel level.
func (L *Logger) Debug2(msg string, arg0, arg1 interface{}) {
	L.log2(DebugLevel, msg, arg0, arg1)
}

// Debug3 calls Log3 with the DebugLevel level.
func (L *Logger) Debug3(msg string, arg0, arg1, arg2 interface{}) {
	L.preCallFunc()
	L.log3(DebugLevel, msg, arg0, arg1, arg2)
}

// Debug4 calls Log4 with the DebugLevel level.
func (L *Logger) Debug4(msg string, arg0, arg1, arg2, arg3 interface{}) {
	L.preCallFunc()
	L.log4(DebugLevel, msg, arg0, arg1, arg2, arg3)
}

//
// Infos:
//

// Info calls Log with the InfoLevel level.
func (L *Logger) Info(msg string, args ...interface{}) {
	L.preCallFunc()
	L.log(InfoLevel, msg, args)
}

// Info0 calls Log0 with the InfoLevel level.
func (L *Logger) Info0(msg string) {
	L.preCallFunc()
	L.log0(InfoLevel, msg)
}

// Info1 calls Log1 with the InfoLevel level.
func (L *Logger) Info1(msg string, arg0 interface{}) {
	L.preCallFunc()
	L.log1(InfoLevel, msg, arg0)
}

// Info2 calls Log2 with the InfoLevel level.
func (L *Logger) Info2(msg string, arg0, arg1 interface{}) {
	L.preCallFunc()
	L.log2(InfoLevel, msg, arg0, arg1)
}

// Info3 calls Log3 with the InfoLevel level.
func (L *Logger) Info3(msg string, arg0, arg1, arg2 interface{}) {
	L.preCallFunc()
	L.log3(InfoLevel, msg, arg0, arg1, arg2)
}

// Info4 calls Log4 with the InfoLevel level.
func (L *Logger) Info4(msg string, arg0, arg1, arg2, arg3 interface{}) {
	L.preCallFunc()
	L.log4(InfoLevel, msg, arg0, arg1, arg2, arg3)
}

//
// Warns:
//

// Warn calls Log with the WarnLevel level.
func (L *Logger) Warn(msg string, args ...interface{}) {
	L.preCallFunc()
	L.log(WarnLevel, msg, args)
}

// Warn0 calls Log0 with the WarnLevel level.
func (L *Logger) Warn0(msg string) {
	L.preCallFunc()
	L.log0(WarnLevel, msg)
}

// Warn1 calls Log1 with the WarnLevel level.
func (L *Logger) Warn1(msg string, arg0 interface{}) {
	L.preCallFunc()
	L.log1(WarnLevel, msg, arg0)
}

// Warn2 calls Log2 with the WarnLevel level.
func (L *Logger) Warn2(msg string, arg0, arg1 interface{}) {
	L.preCallFunc()
	L.log2(WarnLevel, msg, arg0, arg1)
}

// Warn3 calls Log3 with the WarnLevel level.
func (L *Logger) Warn3(msg string, arg0, arg1, arg2 interface{}) {
	L.preCallFunc()
	L.log3(WarnLevel, msg, arg0, arg1, arg2)
}

// Warn4 calls Log4 with the WarnLevel level.
func (L *Logger) Warn4(msg string, arg0, arg1, arg2, arg3 interface{}) {
	L.preCallFunc()
	L.log4(WarnLevel, msg, arg0, arg1, arg2, arg3)
}

//
// Errors:
//

// Error calls Log with the ErrorLevel level.
func (L *Logger) Error(msg string, args ...interface{}) {
	L.preCallFunc()
	L.log(ErrorLevel, msg, args)
}

// Error0 calls Log0 with the ErrorLevel level.
func (L *Logger) Error0(msg string) {
	L.preCallFunc()
	L.log0(ErrorLevel, msg)
}

// Error1 calls Log1 with the ErrorLevel level.
func (L *Logger) Error1(msg string, arg0 interface{}) {
	L.preCallFunc()
	L.log1(ErrorLevel, msg, arg0)
}

// Error2 calls Log2 with the ErrorLevel level.
func (L *Logger) Error2(msg string, arg0, arg1 interface{}) {
	L.preCallFunc()
	L.log2(ErrorLevel, msg, arg0, arg1)
}

// Error3 calls Log3 with the ErrorLevel level.
func (L *Logger) Error3(msg string, arg0, arg1, arg2 interface{}) {
	L.preCallFunc()
	L.log3(ErrorLevel, msg, arg0, arg1, arg2)
}

// Error4 calls Log4 with the ErrorLevel level.
func (L *Logger) Error4(msg string, arg0, arg1, arg2, arg3 interface{}) {
	L.preCallFunc()
	L.log4(ErrorLevel, msg, arg0, arg1, arg2, arg3)
}

// LogErr logs the given error at ErrorLevel
func (L *Logger) LogErr(err error) {
	L.preCallFunc()
	L.log0(ErrorLevel, err.Error())
}

//
// CreateEvents
//

// createEventFromCaller creates an Event object given a level, message, and
// its arguments.  The caller value specifies how many stack frames to skip
// before getting the function and file name and line number of the caller.
func (L *Logger) createEventFromCaller(level Level, msg string, args []interface{}, caller int) *Event {
	pc, file, line, _ := runtime.Caller(caller + 1)
	f := runtime.FuncForPC(pc)
	var funcname string
	if f != nil {
		funcname = f.Name()
	}
	return L.CreateEvent(time.Now(), level, msg, args, funcname, file, line)
}

// CreateEvent doesn't always actually create an event but will reuse an event
// that's been added to the event pool (to reduce allocations).
func (L *Logger) CreateEvent(time time.Time, level Level, msg string, args []interface{}, funcname, file string, line int) *Event {
	event := L.pools.getEvent()
	event.Name = L.name
	event.Time = time
	event.Level = level
	event.Msg = msg
	event.Args = args
	event.FuncName = funcname
	event.File = file
	event.Line = line
	return event
}

// CreateEventNow works similarly to CreateEvent but it assumes the time argument is time.Now().
func (L *Logger) CreateEventNow(level Level, msg string, args []interface{}, funcname, filename string, line int) *Event {
	return L.CreateEvent(time.Now(), level, msg, args, funcname, filename, line)
}

func (L *Logger) createEvent0FromCaller(level Level, msg string, caller int) *Event {
	return L.createEventFromCaller(level, msg, nil, caller+1)
}

func (L *Logger) createEvent1FromCaller(level Level, msg string, arg0 interface{}, caller int) *Event {
	s := append(L.pools.getArgs(), arg0)
	return L.createEventFromCaller(level, msg, s, caller+1)
}

func (L *Logger) createEvent2FromCaller(level Level, msg string, arg0, arg1 interface{}, caller int) *Event {
	s := append(L.pools.getArgs(), arg0, arg1)
	return L.createEventFromCaller(level, msg, s, caller+1)
}

func (L *Logger) createEvent3FromCaller(level Level, msg string, arg0, arg1, arg2 interface{}, caller int) *Event {
	s := append(L.pools.getArgs(), arg0, arg1, arg2)
	return L.createEventFromCaller(level, msg, s, caller+1)
}

func (L *Logger) createEvent4FromCaller(level Level, msg string, arg0, arg1, arg2, arg3 interface{}, caller int) *Event {
	s := append(L.pools.getArgs(), arg0, arg1, arg2, arg3)
	return L.createEventFromCaller(level, msg, s, caller+1)
}
