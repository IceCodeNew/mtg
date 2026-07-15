//go:build !windows

package utils

import (
	"context"
	"errors"
	"net"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type failingListener struct{}

func (failingListener) Accept() (net.Conn, error) { return nil, errors.New("accept failed") }
func (failingListener) Close() error              { return nil }
func (failingListener) Addr() net.Addr            { return &net.TCPAddr{} }

func TestListenerAcceptError(t *testing.T) {
	_, err := (Listener{Listener: failingListener{}}).Accept()
	require.ErrorContains(t, err, "accept failed")
}

func TestNewListener(t *testing.T) {
	_, err := NewListener("not-an-address", 0)
	require.Error(t, err)

	listener, err := NewListener("127.0.0.1:0", 0)
	require.NoError(t, err)
	t.Cleanup(func() { listener.Close() }) //nolint:errcheck

	accepted := make(chan net.Conn, 1)
	errs := make(chan error, 1)
	go func() {
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			errs <- acceptErr

			return
		}
		accepted <- conn
	}()

	client, err := net.Dial("tcp", listener.Addr().String())
	require.NoError(t, err)
	t.Cleanup(func() { client.Close() }) //nolint:errcheck

	select {
	case conn := <-accepted:
		require.NoError(t, conn.Close())
	case err := <-errs:
		t.Fatal(err)
	case <-time.After(time.Second):
		t.Fatal("listener did not accept connection")
	}
}

func TestRootContextHandlesTerminationSignal(t *testing.T) {
	ctx := RootContext()
	require.NoError(t, syscall.Kill(os.Getpid(), syscall.SIGTERM))

	select {
	case <-ctx.Done():
		require.ErrorIs(t, ctx.Err(), context.Canceled)
	case <-time.After(time.Second):
		t.Fatal("root context was not canceled")
	}
}
