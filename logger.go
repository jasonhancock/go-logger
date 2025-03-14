package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/go-stack/stack"
)

// Constants defining various output formats.
const (
	FormatLogFmt = "logfmt"
	FormatJSON   = "json"
)

// AvailableFormats lists the available format types.
var AvailableFormats = []string{
	FormatLogFmt,
	FormatJSON,
}

const (
	LevelAll   = slog.Level(-10)
	LevelFatal = slog.Level(12)
)

var levelNames = map[slog.Leveler]string{
	LevelAll:        "all",
	LevelFatal:      "fatal",
	slog.LevelError: "err",
	slog.LevelWarn:  "warn",
	slog.LevelInfo:  "info",
	slog.LevelDebug: "debug",
}

// ParseLevel parses the string into a Level.
func ParseLevel(s string) slog.Leveler {
	s = strings.ToLower(s)
	for l, name := range levelNames {
		if strings.HasPrefix(name, s) {
			return l
		}
	}
	return LevelAll
}

// L is the logger implementation
type L struct {
	slogger          *slog.Logger
	src              []string
	showCaller       bool
	callerPrefixTrim string
}

// New initializes a new logger. If w is nil, logs will be sent to stdout.
func New(opts ...Option) *L {
	opt := &options{
		destination: os.Stdout,
		name:        filepath.Base(os.Args[0]),
		showCaller:  true,
	}

	for _, o := range opts {
		o(opt)
	}

	if opt.timeFormatter == nil {
		// Detect if the current Location is UTC or not. If not, install the formatter.
		// This is an optimization because servers should be set to UTC.
		ts := time.Now()
		if ts.Format(time.RFC3339Nano) != ts.In(time.UTC).Format(time.RFC3339Nano) {
			opt.timeFormatter = func(ts time.Time) string {
				return ts.In(time.UTC).Format(time.RFC3339Nano)
			}
		}
	}

	var l *slog.Logger

	handlerOpts := slog.HandlerOptions{
		Level: opt.leveler,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			switch a.Key {
			case slog.TimeKey:
				a.Key = "ts"
				if opt.timeFormatter != nil {
					a.Value = slog.StringValue(opt.timeFormatter(a.Value.Time()))
				}
			case slog.LevelKey:
				level := a.Value.Any().(slog.Level)
				levelLabel, exists := levelNames[level]
				if !exists {
					levelLabel = level.String()
				}
				a.Value = slog.StringValue(levelLabel)
			default:
			}

			return a

		},
	}

	switch strings.ToLower(opt.format) {
	case FormatJSON:
		l = slog.New(slog.NewJSONHandler(opt.destination, &handlerOpts))
	default:
		l = slog.New(slog.NewTextHandler(opt.destination, &handlerOpts))
	}

	l = l.With(append(opt.keyvals, slog.String("src", opt.name))...)

	return &L{
		slogger:          l,
		src:              []string{opt.name},
		showCaller:       opt.showCaller,
		callerPrefixTrim: opt.callerPrefixTrim,
	}
}

// caller returns a string that returns a file and line from a specified depth
// in the callstack.
// func caller(depth int) string {
func caller(depth int, prefixTrim string) string {
	c := stack.Caller(depth)
	// The format string here has special meaning. See
	// https://godoc.org/github.com/go-stack/stack#Call.Format
	const format = "%+k/%s:%d"
	if prefixTrim != "" {
		return strings.TrimPrefix(fmt.Sprintf(format, c, c, c), prefixTrim)
	}
	return fmt.Sprintf(format, c, c, c)
}

// New returns a sub-logger with the name appended to the existing logger's source
func (l *L) New(name string) *L {
	return &L{
		src:              append(l.src, name),
		slogger:          l.slogger.With(slog.String("src", strings.Join(append(l.src, name), "."))),
		showCaller:       l.showCaller,
		callerPrefixTrim: l.callerPrefixTrim,
	}
}

// With returns a logger with the keyvals appended to the existing logger
func (l *L) With(keyvals ...any) *L {
	return &L{
		src:              l.src,
		slogger:          l.slogger.With(keyvals...),
		showCaller:       l.showCaller,
		callerPrefixTrim: l.callerPrefixTrim,
	}
}

// Debug logs a message at the debug level
func (l *L) Debug(msg any, keyvals ...any) {
	l.log(context.Background(), slog.LevelDebug, msg, keyvals...)
}

// Info logs a message at the info level
func (l *L) Info(msg any, keyvals ...any) {
	l.log(context.Background(), slog.LevelInfo, msg, keyvals...)
}

// Warn logs a message at the warning level
func (l *L) Warn(msg any, keyvals ...any) {
	l.log(context.Background(), slog.LevelWarn, msg, keyvals...)
}

// Err logs a message at the error level
func (l *L) Err(msg any, keyvals ...any) {
	l.log(context.Background(), slog.LevelError, msg, keyvals...)
}

// Fatal logs a message at the fatal level and also exits the program by calling
// os.Exit
func (l *L) Fatal(msg any, keyvals ...any) {
	l.log(context.Background(), LevelFatal, msg, keyvals...)
	os.Exit(1)
}

func (l *L) log(ctx context.Context, lvl slog.Level, msg any, keyvals ...any) {
	if l == nil {
		return
	}

	if l.showCaller {
		keyvals = append(keyvals, slog.String("caller", caller(3, l.callerPrefixTrim)))
	}

	l.slogger.Log(ctx, lvl, toString(msg), keyvals...)
}

func toString(s any) string {
	switch v := s.(type) {
	case string:
		return v
	case error:
		return v.Error()
	case fmt.Stringer:
		return v.String()
	default:
		return fmt.Sprintf("unable to convert type %T to string", v)
	}
}

// Default returns a default logger implementation
func Default() *L {
	return New(
		WithName("default"),
		WithFormat(FormatLogFmt),
	)
}

// Silence returns a logger that writes everything to /dev/null. Useful for
// silencing log output from tests
func Silence() *L {
	return New(
		WithDestination(io.Discard),
		WithName("discard"),
	)
}

type multiError interface {
	WrappedErrors() []error
}

// LogError logs an error. It automatically unwinds multi-errors (not recursively...yet).
func (l *L) LogError(msg string, err error, keyvals ...any) {
	mErr, ok := err.(multiError)
	if !ok {
		l.log(context.Background(), slog.LevelError, msg, append(keyvals, slog.String("error", err.Error()))...)
		return
	}

	errs := mErr.WrappedErrors()

	for i, e := range errs {
		keyvals = append(
			keyvals,
			slog.String(
				fmt.Sprintf("error_%02d", i),
				e.Error(),
			),
		)
	}

	l.log(context.Background(), slog.LevelError, msg, keyvals...)
}

// DynamicLeveler gives you the ability to adjust the log level of the
// application without having to restart it.
type DynamicLeveler struct {
	level *atomic.Value
}

// NewDynamicLeveler initializes a DynamicLeveler and sets the initial log level
// to initialLevel.
func NewDynamicLeveler(initialLevel string) *DynamicLeveler {
	d := DynamicLeveler{
		level: new(atomic.Value),
	}

	d.level.Store(ParseLevel(initialLevel))

	return &d
}

// SetLevel changes the log level.
func (d *DynamicLeveler) SetLevel(level string) {
	d.level.Store(ParseLevel(level))
}

// Level returns the latest log level.
func (d *DynamicLeveler) Level() slog.Level {
	return d.level.Load().(slog.Level)
}
