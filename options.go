package logger

import "io"

type options struct {
	format      string
	name        string
	keyvals     []interface{}
	level       string
	destination io.Writer
}

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
