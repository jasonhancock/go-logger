package logger

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/go-kit/log"
	"github.com/go-stack/stack"
	"github.com/hashicorp/go-multierror"
)

// Constants defining various output formats.
const (
	FormatLogFmt = "logfmt"
	FormatJSON   = "json"
)

// Level represents the logging level
type Level int8

// String returns the string representation of a level.
func (l Level) String() string {
	name, ok := levelNames[l]
	if !ok {
		return levelNames[All]
	}
	return name
}

var levelNames = map[Level]string{
	All:   "all",
	Fatal: "fatal",
	Err:   "err",
	Warn:  "warn",
	Info:  "info",
	Debug: "debug",
}

// Level constants
const (
	All Level = iota
	Fatal
	Err
	Warn
	Info
	Debug
)

// ParseLevel parses the string into a Level.
func ParseLevel(s string) Level {
	for l, name := range levelNames {
		if strings.HasPrefix(name, strings.ToLower(s)) {
			return l
		}
	}
	return All
}

// L is the logger implementation
type L struct {
	logger log.Logger
	Level  Level
	src    []string
}

// New initializes a new logger. If w is nil, logs will be sent to stdout.
func New(w io.Writer, name, level, format string, keyvals ...interface{}) *L {
	if w == nil {
		w = os.Stdout
	}

	var l log.Logger

	switch strings.ToLower(format) {
	case FormatJSON:
		l = log.NewJSONLogger(log.NewSyncWriter(w))
	default:
		l = log.NewLogfmtLogger(log.NewSyncWriter(w))
	}

	l = log.With(
		l,
		"ts", log.DefaultTimestampUTC,
		"caller", caller(5),
	)

	if len(keyvals) > 0 {
		l = log.With(l, keyvals...)
	}

	return &L{
		logger: l,
		Level:  ParseLevel(level),
		src:    []string{name},
	}
}

// Caller returns a log.Valuer that returns a file and line from a specified depth
// in the callstack.
func caller(depth int) log.Valuer {
	return func() interface{} {
		c := stack.Caller(depth)
		// The format string here has special meaning. See
		// https://godoc.org/github.com/go-stack/stack#Call.Format
		return fmt.Sprintf("%+k/%s:%d", c, c, c)
	}
}

// New returns a sub-logger with the name appended to the existing logger's source
func (l *L) New(name string) *L {
	return &L{
		src:    append(l.src, name),
		Level:  l.Level,
		logger: l.logger,
	}
}

// With returns a logger with the keyvals appended to the existing logger
func (l *L) With(keyvals ...interface{}) *L {
	return &L{
		src:    l.src,
		Level:  l.Level,
		logger: log.With(l.logger, keyvals...),
	}
}

// Debug logs a message at the debug level
func (l *L) Debug(msg interface{}, keyvals ...interface{}) {
	l.log(Debug, log.With(l.logger, "src", l.source(), "level", Debug.String(), "msg", msg), keyvals...)
}

// Info logs a message at the info level
func (l *L) Info(msg interface{}, keyvals ...interface{}) {
	l.log(Info, log.With(l.logger, "src", l.source(), "level", Info.String(), "msg", msg), keyvals...)
}

// Warn logs a message at the warning level
func (l *L) Warn(msg interface{}, keyvals ...interface{}) {
	l.log(Warn, log.With(l.logger, "src", l.source(), "level", Warn.String(), "msg", msg), keyvals...)
}

// Err logs a message at the error level
func (l *L) Err(msg interface{}, keyvals ...interface{}) {
	l.log(Err, log.With(l.logger, "src", l.source(), "level", Err.String(), "msg", msg), keyvals...)
}

// Fatal logs a message at the fatal level and also exits the program by calling
// os.Exit
func (l *L) Fatal(msg interface{}, keyvals ...interface{}) {
	l.log(Fatal, log.With(l.logger, "src", l.source(), "level", Fatal.String(), "msg", msg), keyvals...)
	os.Exit(1)
}

func (l *L) source() string {
	return strings.Join(l.src, ".")
}

func (l *L) log(level Level, lvl log.Logger, keyvals ...interface{}) {
	if l == nil {
		return
	}
	if level > l.Level && l.Level != All {
		return
	}

	_ = lvl.Log(keyvals...)
}

// Default returns a default logger implementation
func Default() *L {
	return New(nil, "default", "", FormatLogFmt)
}

// Silence returns a logger that writes everything to /dev/null. Useful for
// silencing log output from tests
func Silence() *L {
	return New(io.Discard, "discard", "", FormatLogFmt)
}

// LogError logs an error. It automatically unwinds multi-errors (not recursively...yet).
func (l *L) LogError(msg string, err error) {
	mErr, ok := err.(*multierror.Error)
	if !ok {
		l.Err(msg, "error", err.Error())
		return
	}

	keyvals := make([]interface{}, 0, len(mErr.Errors)*2)

	for i, e := range mErr.Errors {
		keyvals = append(
			keyvals,
			fmt.Sprintf("error_%02d", i),
			e.Error(),
		)
	}

	l.Err(msg, keyvals...)
}
