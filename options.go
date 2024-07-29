package logger

import (
	"io"
	"runtime/debug"
	"strings"
	"time"
)

type options struct {
	format           string
	name             string
	keyvals          []interface{}
	level            string
	destination      io.Writer
	showCaller       bool
	callerPrefixTrim string
	timeFormatter    TimeFormatterFunc
}

type TimeFormatterFunc func(time.Time) string

// Option is used to customize the logger.
type Option func(*options)

// WithFormat sets the format to log in.
func WithFormat(format string) Option {
	return func(o *options) {
		o.format = format
	}
}

// With adds key value pairs to the logger.
func With(keyvals ...interface{}) Option {
	return func(o *options) {
		o.keyvals = keyvals
	}
}

// WithLevel sets the logging level of the logger.
func WithLevel(level string) Option {
	return func(o *options) {
		o.level = level
	}
}

// WithDestination sets the target for where the output of the logger should be
// written.
func WithDestination(w io.Writer) Option {
	return func(o *options) {
		o.destination = w
	}
}

// WithName specifies the name of the application. If not specified, the current
// processes name will be used.
func WithName(name string) Option {
	return func(o *options) {
		o.name = name
	}
}

// WithCaller sets whether or not to include the source file and line number of
// where the message originated.
func WithCaller(showCaller bool) Option {
	return func(o *options) {
		o.showCaller = showCaller
	}
}

// WithTimeLocation specifies the locale to log the time in.
func WithTimeLocation(loc *time.Location) Option {
	return func(o *options) {
		o.timeFormatter = func(ts time.Time) string {
			return ts.In(loc).Format(time.RFC3339Nano)
		}
	}
}

// WithCallerPrefixTrim manually specifies a path to trim from the caller value
// of each log message.
func WithCallerPrefixTrim(str string) Option {
	return func(o *options) {
		if str != "" {
			if !strings.HasSuffix(str, "/") {
				str += "/"
			}
			o.callerPrefixTrim = str
		}
	}
}

// WithAutoCallerPrefixTrim intelligently figures out the prefix to trim from the
// caller value of each log message.
func WithAutoCallerPrefixTrim() Option {
	bi, ok := debug.ReadBuildInfo()
	if !ok || bi == nil {
		return func(o *options) {}
	}

	return WithCallerPrefixTrim(bi.Main.Path)
}
