package network

import (
	"bufio"
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/IceCodeNew/mtg/essentials"
	"github.com/IceCodeNew/mtg/mtglib"
	"github.com/stretchr/testify/require"
	"github.com/things-go/go-socks5"
)

func routingProxy(t *testing.T, destination string) mtglib.Network {
	t.Helper()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	require.NoError(t, err)
	server := socks5.NewServer(socks5.WithDial(func(ctx context.Context, protocol, _ string) (net.Conn, error) {
		return (&net.Dialer{Timeout: time.Second}).DialContext(ctx, protocol, destination)
	}))
	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()
	t.Cleanup(func() {
		require.NoError(t, listener.Close())
		require.ErrorIs(t, <-done, net.ErrClosed)
	})
	endpoint, err := url.Parse("socks5://" + listener.Addr().String())
	require.NoError(t, err)
	proxy, err := NewProxyNetwork(New(nil, "test", time.Second, time.Second, 0, DefaultKeepAliveConfig, 0), endpoint)
	require.NoError(t, err)
	return proxy
}

func closedProxy(t *testing.T) (mtglib.Network, string) {
	t.Helper()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	require.NoError(t, err)
	address := listener.Addr().String()
	require.NoError(t, listener.Close())
	endpoint, err := url.Parse("socks5://" + address)
	require.NoError(t, err)
	proxy, err := NewProxyNetwork(New(nil, "test", time.Second, time.Second, 0, DefaultKeepAliveConfig, 0), endpoint)
	require.NoError(t, err)
	return proxy, address
}

func responseServer(t *testing.T, body string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if _, err := io.WriteString(w, body); err != nil {
			t.Error(err)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func assertHTTPBody(t *testing.T, client *http.Client, address, want string) {
	t.Helper()
	t.Cleanup(client.CloseIdleConnections)
	request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, address, nil)
	require.NoError(t, err)
	response, err := client.Do(request)
	require.NoError(t, err)
	require.NotNil(t, response)
	body, err := io.ReadAll(response.Body)
	require.NoError(t, response.Body.Close())
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, response.StatusCode)
	require.Equal(t, want, string(body))
}

func TestMultiNetwork(t *testing.T) {
	t.Run("user Given no networks When joining Then configuration is rejected", func(t *testing.T) {
		_, err := Join()
		require.Error(t, err)
	})
	t.Run("user Given two offline proxies When dialing Then both connection failures are reported", func(t *testing.T) {
		first, firstAddress := closedProxy(t)
		second, secondAddress := closedProxy(t)
		joined, err := Join(first, second)
		require.NoError(t, err)
		_, err = joined.Dial("tcp", "127.0.0.1:1")
		require.ErrorIs(t, err, ErrCannotDial)
		require.ErrorContains(t, err, firstAddress)
		require.ErrorContains(t, err, secondAddress)
	})
	t.Run("user Given an offline and a live proxy When HTTP uses the default route Then the live destination responds", func(t *testing.T) {
		server := responseServer(t, "joined route")
		offline, _ := closedProxy(t)
		live := routingProxy(t, server.Listener.Addr().String())
		joined, err := Join(offline, live)
		require.NoError(t, err)
		assertHTTPBody(t, joined.MakeHTTPClient(nil), "http://127.0.0.1:1", "joined route")
		conn, err := joined.NativeDialer().DialContext(t.Context(), "tcp", server.Listener.Addr().String())
		require.NoError(t, err)
		require.NoError(t, conn.Close())
	})
	t.Run("user Given a custom HTTP route When requesting Then the supplied route selects a different isolated server", func(t *testing.T) {
		defaultServer := responseServer(t, "default route")
		customServer := responseServer(t, "custom route")
		joined, err := Join(routingProxy(t, defaultServer.Listener.Addr().String()))
		require.NoError(t, err)
		base := New(nil, "test", time.Second, time.Second, 0, DefaultKeepAliveConfig, 0)
		callback := func(ctx context.Context, protocol, _ string) (essentials.Conn, error) {
			return base.DialContext(ctx, protocol, customServer.Listener.Addr().String())
		}
		assertHTTPBody(t, joined.MakeHTTPClient(callback), "http://127.0.0.1:1", "custom route")
	})
}

func TestProxyNetworkDial(t *testing.T) {
	t.Run("user Given an offline proxy When dialing Then the connection error identifies that proxy", func(t *testing.T) {
		proxy, address := closedProxy(t)
		_, err := proxy.Dial("tcp", "127.0.0.1:1")
		require.ErrorContains(t, err, address)
	})
	t.Run("user Given a live proxy When dialing Then the connection carries the destination response", func(t *testing.T) {
		server := responseServer(t, "proxied connection")
		proxy := routingProxy(t, server.Listener.Addr().String())
		conn, err := proxy.DialContext(t.Context(), "tcp", "127.0.0.1:1")
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, conn.Close()) })
		require.NoError(t, conn.SetDeadline(time.Now().Add(time.Second)))
		_, err = io.WriteString(conn, "GET / HTTP/1.1\r\nHost: localhost\r\nConnection: close\r\n\r\n")
		require.NoError(t, err)
		response, err := http.ReadResponse(bufio.NewReader(conn), nil)
		require.NoError(t, err)
		body, err := io.ReadAll(response.Body)
		require.NoError(t, response.Body.Close())
		require.NoError(t, err)
		require.Equal(t, "proxied connection", string(body))
		assertHTTPBody(t, proxy.MakeHTTPClient(nil), "http://127.0.0.1:1", "proxied connection")
	})
}
