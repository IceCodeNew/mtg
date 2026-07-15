package proxyprotocol

import (
	"errors"
	"net"
	"testing"
	"time"

	"github.com/pires/go-proxyproto"
	"github.com/stretchr/testify/require"
)

type failingListener struct{}

func (failingListener) Accept() (net.Conn, error) { return nil, errors.New("accept failed") }
func (failingListener) Close() error              { return nil }
func (failingListener) Addr() net.Addr            { return &net.TCPAddr{} }

func TestListenerAdapterAcceptError(t *testing.T) {
	listener := &ListenerAdapter{Listener: proxyproto.Listener{Listener: failingListener{}}}
	_, err := listener.Accept()
	require.ErrorContains(t, err, "accept failed")
}

func TestListenerAdapterAcceptTCP(t *testing.T) {
	base, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { base.Close() }) //nolint:errcheck

	listener := &ListenerAdapter{Listener: proxyproto.Listener{
		Listener:          base,
		ReadHeaderTimeout: -1,
	}}

	client, err := net.DialTimeout("tcp", base.Addr().String(), time.Second)
	require.NoError(t, err)
	t.Cleanup(func() { client.Close() }) //nolint:errcheck

	conn, err := listener.Accept()
	require.NoError(t, err)
	require.NoError(t, conn.(interface{ CloseRead() error }).CloseRead())
	require.NoError(t, conn.(interface{ CloseWrite() error }).CloseWrite())
	require.NoError(t, conn.Close())
}

func TestConnWrapperRejectsNonTCPConnections(t *testing.T) {
	left, right := net.Pipe()
	t.Cleanup(func() { left.Close() })  //nolint:errcheck
	t.Cleanup(func() { right.Close() }) //nolint:errcheck

	conn := connWrapper{Conn: proxyproto.NewConn(left)}
	require.Panics(t, func() { _ = conn.CloseRead() })
	require.Panics(t, func() { _ = conn.CloseWrite() })
}
