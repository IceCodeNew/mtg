package proxyprotocol

import (
	"net"
	"testing"
	"time"

	"github.com/pires/go-proxyproto"
	"github.com/stretchr/testify/require"
)

func TestListenerAdapterAcceptError(t *testing.T) {
	t.Run("user Given a closed TCP listener When accepting through the PROXY adapter Then the closed-listener error is preserved", func(t *testing.T) {
		base, err := net.Listen("tcp4", "127.0.0.1:0")
		require.NoError(t, err)
		require.NoError(t, base.Close())
		listener := &ListenerAdapter{Listener: proxyproto.Listener{Listener: base}}
		_, err = listener.Accept()
		require.ErrorIs(t, err, net.ErrClosed)
	})
}

func TestListenerAdapterAcceptTCP(t *testing.T) {
	t.Run("user Given a TCP connection When accepting through the PROXY adapter Then both TCP half-close operations are available", func(t *testing.T) {
		base, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, base.Close()) })

		listener := &ListenerAdapter{Listener: proxyproto.Listener{
			Listener:          base,
			ReadHeaderTimeout: -1,
		}}

		client, err := net.DialTimeout("tcp", base.Addr().String(), time.Second)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, client.Close()) })

		conn, err := listener.Accept()
		require.NoError(t, err)
		require.NoError(t, conn.(interface{ CloseRead() error }).CloseRead())
		require.NoError(t, conn.(interface{ CloseWrite() error }).CloseWrite())
		require.NoError(t, conn.Close())
	})
}

func TestConnWrapperRejectsNonTCPConnections(t *testing.T) {
	t.Run("user Given a non-TCP connection When requesting TCP half-close Then the unsupported connection is rejected", func(t *testing.T) {
		left, right := net.Pipe()
		t.Cleanup(func() { require.NoError(t, left.Close()) })
		t.Cleanup(func() { require.NoError(t, right.Close()) })

		conn := connWrapper{Conn: proxyproto.NewConn(left)}
		require.Panics(t, func() { _ = conn.CloseRead() })
		require.Panics(t, func() { _ = conn.CloseWrite() })
	})
}
