package essentials_test

import (
	"io"
	"net"
	"testing"
	"time"

	"github.com/IceCodeNew/mtg/essentials"
	"github.com/stretchr/testify/require"
)

func TestWrapNetConnDelegatesHalfClose(t *testing.T) {
	for _, closeRead := range []bool{false, true} {
		t.Run("user Given a TCP connection When closing the "+map[bool]string{false: "write", true: "read"}[closeRead]+" half Then the opposite half remains usable", func(t *testing.T) {
			listener, err := net.Listen("tcp4", "127.0.0.1:0")
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, listener.Close()) })
			base, err := net.DialTimeout("tcp4", listener.Addr().String(), time.Second)
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, base.Close()) })
			peer, err := listener.Accept()
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, peer.Close()) })
			require.NoError(t, base.SetDeadline(time.Now().Add(time.Second)))
			require.NoError(t, peer.SetDeadline(time.Now().Add(time.Second)))
			conn := essentials.WrapNetConn(base)
			buf := make([]byte, 1)
			if closeRead {
				require.NoError(t, conn.CloseRead())
				_, err = conn.Read(buf)
				require.ErrorIs(t, err, io.EOF)
				_, err = conn.Write([]byte("x"))
				require.NoError(t, err)
				_, err = io.ReadFull(peer, buf)
			} else {
				require.NoError(t, conn.CloseWrite())
				_, err = peer.Read(buf)
				require.ErrorIs(t, err, io.EOF)
				_, err = peer.Write([]byte("x"))
				require.NoError(t, err)
				_, err = io.ReadFull(conn, buf)
			}
			require.NoError(t, err)
			require.Equal(t, []byte("x"), buf)
		})
	}
}

func TestWrapNetConnFallsBackToClose(t *testing.T) {
	t.Run("user Given a pipe without half-close When closing its read half Then the peer cannot write", func(t *testing.T) {
		left, right := net.Pipe()
		t.Cleanup(func() { require.NoError(t, right.Close()) })

		require.NoError(t, essentials.WrapNetConn(left).CloseRead())
		_, err := right.Write([]byte("closed"))
		require.Error(t, err)
	})

	t.Run("user Given a pipe without half-close When closing its write half Then the peer cannot write", func(t *testing.T) {
		left, right := net.Pipe()
		t.Cleanup(func() { require.NoError(t, right.Close()) })

		require.NoError(t, essentials.WrapNetConn(left).CloseWrite())
		_, err := right.Write([]byte("closed"))
		require.Error(t, err)
	})
}
