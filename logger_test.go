package logger

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/go-multierror"
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
		defer buf.Reset()

		var err error
		err = multierror.Append(err, errors.New("some err1"))
		err = multierror.Append(err, errors.New("some err2"))
		l.LogError("some error", err)

		require.Contains(t, buf.String(), `msg="some error"`)
		require.Contains(t, buf.String(), `error_00="some err1"`)
		require.Contains(t, buf.String(), `error_01="some err2"`)
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
