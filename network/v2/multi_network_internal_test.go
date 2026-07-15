package network

import (
	"context"
	"errors"
	"net"
	"net/http"
	"testing"

	"github.com/IceCodeNew/mtg/essentials"
	"github.com/stretchr/testify/require"
)

type stubNetwork struct {
	dialErr    error
	dialCalls  int
	native     *net.Dialer
	httpClient *http.Client
}

func (n *stubNetwork) Dial(network, address string) (essentials.Conn, error) {
	return n.DialContext(context.Background(), network, address)
}

func (n *stubNetwork) DialContext(context.Context, string, string) (essentials.Conn, error) {
	n.dialCalls++

	return nil, n.dialErr
}

func (n *stubNetwork) NativeDialer() *net.Dialer { return n.native }

func (n *stubNetwork) MakeHTTPClient(
	func(context.Context, string, string) (essentials.Conn, error),
) *http.Client {
	return n.httpClient
}

func TestMultiNetwork(t *testing.T) {
	_, err := Join()
	require.Error(t, err)

	firstErr := errors.New("first")
	secondErr := errors.New("second")
	first := &stubNetwork{dialErr: firstErr, native: &net.Dialer{}, httpClient: &http.Client{}}
	second := &stubNetwork{dialErr: secondErr}
	joined, err := Join(first, second)
	require.NoError(t, err)

	_, err = joined.Dial("tcp", "example.invalid:443")
	require.ErrorIs(t, err, ErrCannotDial)
	require.ErrorIs(t, err, firstErr)
	require.ErrorIs(t, err, secondErr)
	require.Equal(t, 1, first.dialCalls)
	require.Equal(t, 1, second.dialCalls)
	require.Same(t, first.native, joined.NativeDialer())
	require.Same(t, first.httpClient, joined.MakeHTTPClient(nil))
}

type stubContextDialer struct {
	conn net.Conn
	err  error
}

func (d stubContextDialer) Dial(string, string) (net.Conn, error) { return d.conn, d.err }

func (d stubContextDialer) DialContext(context.Context, string, string) (net.Conn, error) {
	return d.conn, d.err
}

func TestProxyNetworkDial(t *testing.T) {
	base := &stubNetwork{httpClient: &http.Client{}}
	proxy := proxyNetwork{Network: base, client: stubContextDialer{err: errors.New("proxy offline")}}
	_, err := proxy.Dial("tcp", "example.invalid:443")
	require.ErrorContains(t, err, "proxy offline")

	left, right := net.Pipe()
	t.Cleanup(func() { left.Close() })  //nolint:errcheck
	t.Cleanup(func() { right.Close() }) //nolint:errcheck
	proxy.client = stubContextDialer{conn: left}
	conn, err := proxy.DialContext(context.Background(), "tcp", "example.invalid:443")
	require.NoError(t, err)
	require.NotNil(t, conn)
	require.Same(t, base.httpClient, proxy.MakeHTTPClient(nil))
}
