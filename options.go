package logger

import (
	"io"
	"time"
)

type options struct {
	format        string
	name          string
	keyvals       []interface{}
	level         string
	destination   io.Writer
	showCaller    bool
	timeFormatter TimeFormatterFunc
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

func With(keyvals ...interface{}) Option {
	return func(o *options) {
		o.keyvals = keyvals
	}
}

func WithLevel(level string) Option {
	return func(o *options) {
		o.level = level
	}
}

func WithDestination(w io.Writer) Option {
	return func(o *options) {
		o.destination = w
	}
}

func WithName(name string) Option {
	return func(o *options) {
		o.name = name
	}
}

func WithCaller(showCaller bool) Option {
	return func(o *options) {
		o.showCaller = showCaller
	}
}

func WithTimeLocation(loc *time.Location) Option {
	return func(o *options) {
		o.timeFormatter = func(ts time.Time) string {
			return ts.In(loc).Format(time.RFC3339Nano)
		}
	}
}
