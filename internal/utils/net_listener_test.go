//go:build !windows

package utils

import (
	"context"
	"net"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestListenerAcceptError(t *testing.T) {
	t.Run("user Given a closed TCP listener When accepting Then the closed-listener error is preserved", func(t *testing.T) {
		base, err := net.Listen("tcp4", "127.0.0.1:0")
		require.NoError(t, err)
		require.NoError(t, base.Close())
		_, err = (Listener{Listener: base}).Accept()
		require.ErrorIs(t, err, net.ErrClosed)
	})
}

func TestNewListener(t *testing.T) {
	t.Run("user Given an invalid or loopback bind address When creating a listener Then invalid input fails and loopback accepts connections", func(t *testing.T) {
		_, err := NewListener("not-an-address", 0)
		require.Error(t, err)

		listener, err := NewListener("127.0.0.1:0", 0)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, listener.Close()) })

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
		t.Cleanup(func() { require.NoError(t, client.Close()) })

		select {
		case conn := <-accepted:
			require.NoError(t, conn.Close())
		case err := <-errs:
			t.Fatal(err)
		case <-time.After(time.Second):
			t.Fatal("listener did not accept connection")
		}
	})
}

func TestRootContextHandlesTerminationSignal(t *testing.T) {
	t.Run("user Given the root context When receiving SIGTERM Then shutdown is requested by cancellation", func(t *testing.T) {
		ctx := RootContext()
		require.NoError(t, syscall.Kill(os.Getpid(), syscall.SIGTERM))

		select {
		case <-ctx.Done():
			require.ErrorIs(t, ctx.Err(), context.Canceled)
		case <-time.After(time.Second):
			t.Fatal("root context was not canceled")
		}
	})
}
