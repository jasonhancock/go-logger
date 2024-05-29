package logger

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
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
