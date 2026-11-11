package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
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

// ContextResolutionFunc extracts fields from a context.Context to be added to
// each log line. It returns alternating key/value pairs, using the same
// convention as the keyvals passed to the logging methods
// (e.g. []any{"request_id", id, "user", user}). If an odd number of items is
// returned, the trailing key is dropped.
//
// Resolvers are called for every log line that passes the level check,
// including lines logged via the non-Context methods (which pass
// context.Background()), so they must handle contexts that don't carry the
// values they look for, typically by returning nil. A nil ctx passed to a
// logging method is replaced with context.Background(), so resolvers never
// receive nil. Resolvers may be called
// concurrently and should be cheap.
type ContextResolutionFunc func(ctx context.Context) []any

// L is the logger implementation
type L struct {
	slogger                *slog.Logger
	src                    []string
	showCaller             bool
	callerPrefixTrim       string
	contextResolutionFuncs []ContextResolutionFunc
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

	if len(opt.keyvals) > 0 {
		l = l.With(opt.keyvals...)
	}

	return &L{
		slogger:                l,
		src:                    []string{opt.name},
		showCaller:             opt.showCaller,
		callerPrefixTrim:       opt.callerPrefixTrim,
		contextResolutionFuncs: opt.contextResolutionFuncs,
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
		// Clip capacity so append always allocates; otherwise sibling
		// sub-loggers can share (and overwrite) the same backing array.
		src:                    append(slices.Clip(l.src), name),
		slogger:                l.slogger,
		showCaller:             l.showCaller,
		callerPrefixTrim:       l.callerPrefixTrim,
		contextResolutionFuncs: l.contextResolutionFuncs,
	}
}

// With returns a logger with the keyvals appended to the existing logger
func (l *L) With(keyvals ...any) *L {
	return &L{
		src:                    l.src,
		slogger:                l.slogger.With(keyvals...),
		showCaller:             l.showCaller,
		callerPrefixTrim:       l.callerPrefixTrim,
		contextResolutionFuncs: l.contextResolutionFuncs,
	}
}

// Debug logs a message at the debug level
func (l *L) Debug(msg any, keyvals ...any) {
	l.log(context.Background(), slog.LevelDebug, msg, keyvals...)
}

// DebugContext logs a message at the debug level, including any fields
// extracted from ctx by the configured ContextResolutionFuncs.
func (l *L) DebugContext(ctx context.Context, msg any, keyvals ...any) {
	l.log(ctx, slog.LevelDebug, msg, keyvals...)
}

// Info logs a message at the info level
func (l *L) Info(msg any, keyvals ...any) {
	l.log(context.Background(), slog.LevelInfo, msg, keyvals...)
}

// InfoContext logs a message at the info level, including any fields
// extracted from ctx by the configured ContextResolutionFuncs.
func (l *L) InfoContext(ctx context.Context, msg any, keyvals ...any) {
	l.log(ctx, slog.LevelInfo, msg, keyvals...)
}

// Warn logs a message at the warning level
func (l *L) Warn(msg any, keyvals ...any) {
	l.log(context.Background(), slog.LevelWarn, msg, keyvals...)
}

// WarnContext logs a message at the warning level, including any fields
// extracted from ctx by the configured ContextResolutionFuncs.
func (l *L) WarnContext(ctx context.Context, msg any, keyvals ...any) {
	l.log(ctx, slog.LevelWarn, msg, keyvals...)
}

// Err logs a message at the error level
func (l *L) Err(msg any, keyvals ...any) {
	l.log(context.Background(), slog.LevelError, msg, keyvals...)
}

// ErrContext logs a message at the error level, including any fields
// extracted from ctx by the configured ContextResolutionFuncs.
func (l *L) ErrContext(ctx context.Context, msg any, keyvals ...any) {
	l.log(ctx, slog.LevelError, msg, keyvals...)
}

// Fatal logs a message at the fatal level and also exits the program by calling
// os.Exit
func (l *L) Fatal(msg any, keyvals ...any) {
	l.log(context.Background(), LevelFatal, msg, keyvals...)
	os.Exit(1)
}

// FatalContext logs a message at the fatal level, including any fields
// extracted from ctx by the configured ContextResolutionFuncs, and then exits
// the program by calling os.Exit
func (l *L) FatalContext(ctx context.Context, msg any, keyvals ...any) {
	l.log(ctx, LevelFatal, msg, keyvals...)
	os.Exit(1)
}

func (l *L) log(ctx context.Context, lvl slog.Level, msg any, keyvals ...any) {
	if l == nil {
		return
	}

	if ctx == nil {
		ctx = context.Background()
	}

	if !l.slogger.Enabled(ctx, lvl) {
		return
	}

	keyvals = append([]any{slog.String("src", strings.Join(l.src, "."))}, keyvals...)
	if l.showCaller {
		keyvals = append(keyvals, slog.String("caller", caller(3, l.callerPrefixTrim)))
	}

	for _, fn := range l.contextResolutionFuncs {
		kv := fn(ctx)
		if len(kv)%2 != 0 {
			kv = kv[:len(kv)-1]
		}
		keyvals = append(keyvals, kv...)
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
	l.log(context.Background(), slog.LevelError, msg, errorKeyvals(err, keyvals)...)
}

// LogErrorContext is like LogError, but includes any fields extracted from ctx
// by the configured ContextResolutionFuncs.
func (l *L) LogErrorContext(ctx context.Context, msg string, err error, keyvals ...any) {
	l.log(ctx, slog.LevelError, msg, errorKeyvals(err, keyvals)...)
}

// errorKeyvals appends err to keyvals, unwinding multi-errors into
// error_00, error_01, etc.
func errorKeyvals(err error, keyvals []any) []any {
	mErr, ok := err.(multiError)
	if !ok {
		return append(keyvals, slog.String("error", err.Error()))
	}

	for i, e := range mErr.WrappedErrors() {
		keyvals = append(
			keyvals,
			slog.String(
				fmt.Sprintf("error_%02d", i),
				e.Error(),
			),
		)
	}

	return keyvals
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
