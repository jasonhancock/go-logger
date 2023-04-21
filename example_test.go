package logger_test

import (
	"os"

	"github.com/jasonhancock/go-logger"
)

func ExampleNew_customized() {
	l := logger.New(
		logger.WithDestination(os.Stdout),
		logger.WithName("myapp"),
		logger.WithLevel("info"),
		logger.WithFormat(logger.FormatLogFmt),
		logger.With("somekey", "someval"),
	)

	l.Info("some message", "anotherkey", "another value")

	// Output would resemble:
	// ts=2023-04-13T17:38:13.516398Z caller=github.com/jasonhancock/go-logger_test/example_test.go:11 somekey=someval src=myapp level=info msg="some message" anotherkey="another value"
}

func ExampleNew_json() {
	l := logger.New(
		logger.WithFormat(logger.FormatJSON),
		logger.WithName("myapp"),
	)

	l.Info("some message", "anotherkey", "another value")

	// Output would resemble:
	// {"anotherkey":"another value","caller":"github.com/jasonhancock/go-logger_test/example_test.go:32","level":"info","msg":"some message","src":"myapp","ts":"2023-04-21T16:08:24.224597Z"}
}

func ExampleNew_minimum() {
	l := logger.New()

	l.Info("some message", "anotherkey", "another value")

	// Output would resemble:
	// ts=2023-04-21T16:09:00.026753Z caller=github.com/jasonhancock/go-logger_test/example_test.go:43 src=go-logger.test level=info msg="some message" anotherkey="another value"
}

func ExampleDefault() {
	l := logger.Default()
	l.Info("some message")

	// Output would resemble:
	// ts=2023-04-21T16:09:28.653472Z caller=github.com/jasonhancock/go-logger_test/example_test.go:51 src=default level=info msg="some message"
}
