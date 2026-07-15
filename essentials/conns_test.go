package essentials_test

import (
	"net"
	"testing"

	"github.com/IceCodeNew/mtg/essentials"
	"github.com/stretchr/testify/require"
)

type halfCloseConn struct {
	net.Conn
	readClosed  bool
	writeClosed bool
}

func (c *halfCloseConn) CloseRead() error {
	c.readClosed = true

	return nil
}

func (c *halfCloseConn) CloseWrite() error {
	c.writeClosed = true

	return nil
}

func TestWrapNetConnDelegatesHalfClose(t *testing.T) {
	left, right := net.Pipe()
	t.Cleanup(func() { left.Close() })  //nolint:errcheck
	t.Cleanup(func() { right.Close() }) //nolint:errcheck

	base := &halfCloseConn{Conn: left}
	conn := essentials.WrapNetConn(base)

	require.NoError(t, conn.CloseRead())
	require.NoError(t, conn.CloseWrite())
	require.True(t, base.readClosed)
	require.True(t, base.writeClosed)
}

func TestWrapNetConnFallsBackToClose(t *testing.T) {
	t.Run("read", func(t *testing.T) {
		left, right := net.Pipe()
		t.Cleanup(func() { right.Close() }) //nolint:errcheck

		require.NoError(t, essentials.WrapNetConn(left).CloseRead())
		_, err := right.Write([]byte("closed"))
		require.Error(t, err)
	})

	t.Run("write", func(t *testing.T) {
		left, right := net.Pipe()
		t.Cleanup(func() { right.Close() }) //nolint:errcheck

		require.NoError(t, essentials.WrapNetConn(left).CloseWrite())
		_, err := right.Write([]byte("closed"))
		require.Error(t, err)
	})
}
