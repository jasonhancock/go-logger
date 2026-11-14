package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLogger(t *testing.T) {
	var buf bytes.Buffer

	l := New(
		WithDestination(&buf),
		WithName("somelogger"),
		WithLevel("info"),
		WithFormat(FormatLogFmt),
		With("key1", "value1"),
	)

	t.Run("info", func(t *testing.T) {
		defer buf.Reset()

		l.Info("foo", "key2", "value2")

		require.Contains(t, buf.String(), "key1=value1")
		require.Contains(t, buf.String(), "key2=value2")
		require.Contains(t, buf.String(), "caller=github.com/jasonhancock/go-logger/logger_test.go")
		require.Contains(t, buf.String(), "ts="+fmt.Sprintf("%d", time.Now().Year()))
		require.Contains(t, buf.String(), "src=somelogger")
		require.Contains(t, buf.String(), "level=info")
		require.Contains(t, buf.String(), "msg=foo")

	})

	t.Run("check-level-filtering", func(t *testing.T) {
		defer buf.Reset()

		l.Debug("debug_message", "keyDebug", "valueDebug")
		require.NotContains(t, buf.String(), "debug_message")
	})

	t.Run("sub-logger", func(t *testing.T) {
		defer buf.Reset()

		sub := l.New("sublogger")
		sub.Info("sub", "key3", "value3")

		require.Contains(t, buf.String(), "key1=value1")
		require.Contains(t, buf.String(), "key3=value3")
		require.Contains(t, buf.String(), "src=somelogger.sublogger")
	})

	t.Run("sub-log-with-vals", func(t *testing.T) {
		defer buf.Reset()

		sub := l.New("sublogger2").With("key4", "value4")
		sub.Info("sub", "key5", "value5")

		require.Contains(t, buf.String(), "key1=value1")
		require.Contains(t, buf.String(), "key4=value4")
		require.Contains(t, buf.String(), "key5=value5")
		require.Contains(t, buf.String(), "src=somelogger.sublogger2")
		require.NotContains(t, buf.String(), "src=somelogger ")
	})

	t.Run("LogError", func(t *testing.T) {
		t.Run("single", func(t *testing.T) {
			defer buf.Reset()

			l.LogError("some error", errors.New("some error message"))

			require.Contains(t, buf.String(), `msg="some error"`)
			require.Contains(t, buf.String(), `error="some error message"`)
		})

		t.Run("single with kv", func(t *testing.T) {
			defer buf.Reset()

			l.LogError("some error", errors.New("some error message"), "key1", "value1")

			require.Contains(t, buf.String(), `msg="some error"`)
			require.Contains(t, buf.String(), `error="some error message"`)
			require.Contains(t, buf.String(), "key1=value1")
		})

		t.Run("multi", func(t *testing.T) {
			defer buf.Reset()

			err := &myMulti{
				errs: []error{
					errors.New("some err1"),
					errors.New("some err2"),
				},
			}
			l.LogError("some error", err)

			require.Contains(t, buf.String(), `msg="some error"`)
			require.Contains(t, buf.String(), `error_00="some err1"`)
			require.Contains(t, buf.String(), `error_01="some err2"`)
		})

		t.Run("multi with kv", func(t *testing.T) {
			defer buf.Reset()

			err := &myMulti{
				errs: []error{
					errors.New("some err1"),
					errors.New("some err2"),
				},
			}
			l.LogError("some error", err, "key1", "value1")

			require.Contains(t, buf.String(), `msg="some error"`)
			require.Contains(t, buf.String(), `error_00="some err1"`)
			require.Contains(t, buf.String(), `error_01="some err2"`)
			require.Contains(t, buf.String(), "key1=value1")
		})
	})

	t.Run("caller-trim", func(t *testing.T) {
		defer buf.Reset()

		l := New(
			WithDestination(&buf),
			WithName("somelogger"),
			WithLevel("info"),
			WithFormat(FormatLogFmt),
			With("key1", "value1"),
			WithCallerPrefixTrim("github.com/jasonhancock/go-logger"),
		)

		l.Info("foo", "key2", "value2")

		require.Contains(t, buf.String(), "key1=value1")
		require.Contains(t, buf.String(), "key2=value2")
		require.Contains(t, buf.String(), "caller=logger_test.go")
		require.Contains(t, buf.String(), "ts="+fmt.Sprintf("%d", time.Now().Year()))
		require.Contains(t, buf.String(), "src=somelogger")
		require.Contains(t, buf.String(), "level=info")
		require.Contains(t, buf.String(), "msg=foo")

	})
}

func TestLoggerJSON(t *testing.T) {
	var buf bytes.Buffer

	l := New(
		WithDestination(&buf),
		WithName("somelogger"),
		WithLevel("info"),
		WithFormat(FormatJSON),
		With("key1", "value1"),
	)

	l.Info("foo", "key2", "value2")
	var data map[string]string
	require.NoError(t, json.NewDecoder(&buf).Decode(&data))
	require.Equal(t, "value1", data["key1"])
	require.Equal(t, "value2", data["key2"])
	require.Contains(t, data["caller"], "github.com/jasonhancock/go-logger/logger_test.go")
	require.Contains(t, data["ts"], fmt.Sprintf("%d", time.Now().Year()))
	require.Equal(t, "somelogger", data["src"])
	require.Equal(t, "info", data["level"])
	require.Equal(t, "foo", data["msg"])
}

func TestToString(t *testing.T) {
	tests := []struct {
		desc     string
		input    any
		expected string
	}{
		{"string", "string", "string"},
		{"error", errors.New("error"), "error"},
		{"custom error", &myError{}, "my error"},
		{"stringer", &myStringer{}, "my string"},
		{"invalid string conversion", &notErrorStringer{}, "unable to convert type *logger.notErrorStringer to string"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			require.Equal(t, tt.expected, toString(tt.input))
		})
	}
}

type myError struct{}

func (e *myError) Error() string { return "my error" }

type myStringer struct{}

func (s *myStringer) String() string { return "my string" }

type notErrorStringer struct{}

type myMulti struct {
	errs []error
}

func (m *myMulti) WrappedErrors() []error {
	return m.errs
}

func (m *myMulti) Error() string {
	return errors.Join(m.errs...).Error()
}

func TestDynamicLogger(t *testing.T) {
	var buf bytes.Buffer

	logLeveler := NewDynamicLeveler("info")
	l := New(
		WithDestination(&buf),
		WithName("somelogger"),
		WithLeveler(logLeveler),
		WithFormat(FormatLogFmt),
	)

	var lvl = new(atomic.Value)

	lvl.Store(slog.LevelInfo)

	// create a sub logger so we can verify sub-loggers also get the dynamic leveler.
	sub := l.New("sub")

	sub.Info("hello world")
	sub.Debug("hello world 2")

	// Change the level
	logLeveler.SetLevel("debug")

	sub.Debug("hello world 3")
	sub.Info("hello world 4")

	require.Contains(t, buf.String(), `msg="hello world"`)
	require.NotContains(t, buf.String(), `msg="hello world 2"`)
	require.Contains(t, buf.String(), `msg="hello world 3"`)
	require.Contains(t, buf.String(), `msg="hello world 4"`)
}

type ctxKey struct{}

func TestContextResolution(t *testing.T) {
	var buf bytes.Buffer
	var calls atomic.Int32

	l := New(
		WithDestination(&buf),
		WithName("ctxlogger"),
		WithLevel("info"),
		WithFormat(FormatLogFmt),
		WithContextResolutionFunc(func(ctx context.Context) []any {
			calls.Add(1)
			if v, ok := ctx.Value(ctxKey{}).(string); ok {
				return []any{"request_id", v}
			}
			return nil
		}),
		WithContextResolutionFunc(func(ctx context.Context) []any {
			return []any{"odd1", "v1", "dangling"}
		}),
	)

	ctx := context.WithValue(context.Background(), ctxKey{}, "abc123")

	tests := []struct {
		name string
		fn   func()
	}{
		{"info", func() { l.InfoContext(ctx, "msg") }},
		{"warn", func() { l.WarnContext(ctx, "msg") }},
		{"err", func() { l.ErrContext(ctx, "msg") }},
		{"log-error", func() { l.LogErrorContext(ctx, "msg", errors.New("boom")) }},
		{"sub-logger", func() { l.New("sub").InfoContext(ctx, "msg") }},
		{"with", func() { l.With("k", "v").InfoContext(ctx, "msg") }},
		{"with-then-new", func() { l.With("k", "v").New("sub").InfoContext(ctx, "msg") }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer buf.Reset()
			tt.fn()
			out := buf.String()
			require.Contains(t, out, "request_id=abc123")
			require.Contains(t, out, "odd1=v1")
			require.NotContains(t, out, "dangling")
			require.NotContains(t, out, "BADKEY")
			require.Contains(t, out, "caller=github.com/jasonhancock/go-logger/logger_test.go")
		})
	}

	t.Run("no-value-in-context", func(t *testing.T) {
		defer buf.Reset()
		l.Info("msg")
		require.NotContains(t, buf.String(), "request_id")
		require.Contains(t, buf.String(), "msg=msg")
	})

	t.Run("nil-context", func(t *testing.T) {
		defer buf.Reset()
		//nolint:staticcheck // intentionally passing a nil context
		l.InfoContext(nil, "msg")
		require.Contains(t, buf.String(), "msg=msg")
		require.NotContains(t, buf.String(), "request_id")
	})

	t.Run("disabled-level-skips-resolvers", func(t *testing.T) {
		defer buf.Reset()
		before := calls.Load()
		l.DebugContext(ctx, "debug_message")
		require.Empty(t, buf.String())
		require.Equal(t, before, calls.Load())
	})
}

func TestSubLoggerSiblingsDoNotShareSrc(t *testing.T) {
	var buf bytes.Buffer

	l := New(
		WithDestination(&buf),
		WithName("root"),
		WithFormat(FormatLogFmt),
	)

	// Nest deep enough that the src slice has spare capacity, which is when
	// append would otherwise share the backing array between siblings.
	parent := l.New("a").New("b")
	first := parent.New("first")
	second := parent.New("second")

	first.Info("msg")
	require.Contains(t, buf.String(), "src=root.a.b.first")
	buf.Reset()

	second.Info("msg")
	require.Contains(t, buf.String(), "src=root.a.b.second")
	buf.Reset()

	// Logging via first again must not have been clobbered by creating second.
	first.Info("msg")
	require.Contains(t, buf.String(), "src=root.a.b.first")
}

// logViaHelper logs through one wrapper frame, like an adapter would. It
// returns the line of its l.Info call.
func logViaHelper(l *L, msg string) int {
	_, _, line, _ := runtime.Caller(0)
	l.Info(msg) // must stay on the line after runtime.Caller
	return line + 1
}

// logViaTwoHelpers logs through two wrapper frames.
func logViaTwoHelpers(l *L, msg string) {
	logViaHelper(l, msg)
}

func TestWithCallerSkip(t *testing.T) {
	var buf bytes.Buffer
	l := New(
		WithDestination(&buf),
		WithFormat(FormatLogFmt),
		WithCallerPrefixTrim("github.com/jasonhancock/go-logger/"),
	)

	t.Run("without skip reports the helper", func(t *testing.T) {
		buf.Reset()
		helperLine := logViaHelper(l, "foo")
		require.Contains(t, buf.String(), fmt.Sprintf("caller=logger_test.go:%d\n", helperLine))
	})

	t.Run("skip one reports the helper's caller", func(t *testing.T) {
		buf.Reset()
		_, _, line, _ := runtime.Caller(0)
		logViaHelper(l.WithCallerSkip(1), "foo")
		require.Contains(t, buf.String(), fmt.Sprintf("caller=logger_test.go:%d\n", line+1))
	})

	t.Run("skips accumulate and survive New and With", func(t *testing.T) {
		sub := l.WithCallerSkip(1).New("sub").With("k", "v").WithCallerSkip(1)
		buf.Reset()
		_, _, line, _ := runtime.Caller(0)
		logViaTwoHelpers(sub, "foo")
		require.Contains(t, buf.String(), fmt.Sprintf("caller=logger_test.go:%d\n", line+1))
		require.Contains(t, buf.String(), "k=v")
		require.Contains(t, buf.String(), "src=go-logger.test.sub")
	})

	t.Run("original logger is unaffected", func(t *testing.T) {
		_ = l.WithCallerSkip(5)
		buf.Reset()
		helperLine := logViaHelper(l, "foo")
		require.Contains(t, buf.String(), fmt.Sprintf("caller=logger_test.go:%d\n", helperLine))
	})
}
