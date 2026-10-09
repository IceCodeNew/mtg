package cli

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/IceCodeNew/mtg/internal/config"
	"github.com/IceCodeNew/mtg/internal/testlib"
	"github.com/IceCodeNew/mtg/ipblocklist"
	"github.com/IceCodeNew/mtg/logger"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/things-go/go-socks5"
)

func TestMakeLogger(t *testing.T) {
	level := zerolog.GlobalLevel()
	t.Cleanup(func() { zerolog.SetGlobalLevel(level) })
	for _, debug := range []bool{false, true} {
		t.Run("user Given debug "+map[bool]string{false: "disabled", true: "enabled"}[debug]+" When logging Then warnings remain visible and debug follows configuration", func(t *testing.T) {
			conf := &config.Config{}
			conf.Debug.Value = debug
			output := testlib.CaptureStdout(func() {
				log := makeLogger(conf)
				log.Debug("debug probe")
				log.Warning("warning probe")
			})
			require.Contains(t, output, `"level":"warn"`)
			require.Contains(t, output, "warning probe")
			if debug {
				require.Contains(t, output, `"level":"debug"`)
				require.Contains(t, output, "debug probe")
			} else {
				require.NotContains(t, output, "debug probe")
			}
		})
	}
}

func TestMakeNetwork(t *testing.T) {
	t.Run("user Given no proxy When requesting loopback HTTP Then the destination responds", func(t *testing.T) {
		network, err := makeNetwork(&config.Config{}, "test")
		require.NoError(t, err)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, writeErr := io.WriteString(w, "direct destination")
			if writeErr != nil {
				t.Error(writeErr)
			}
		}))
		t.Cleanup(server.Close)
		client := network.MakeHTTPClient(nil)
		t.Cleanup(client.CloseIdleConnections)
		request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, server.URL, nil)
		require.NoError(t, err)
		response, err := client.Do(request)
		require.NoError(t, err)
		body, err := io.ReadAll(response.Body)
		require.NoError(t, response.Body.Close())
		require.NoError(t, err)
		require.Equal(t, "direct destination", string(body))
	})

	for _, multiple := range []bool{false, true} {
		t.Run("user Given "+map[bool]string{false: "one live SOCKS proxy", true: "a live and an offline SOCKS proxy"}[multiple]+" When dialing and requesting HTTP Then traffic uses the live proxy", func(t *testing.T) {
			conf := &config.Config{}
			require.NoError(t, conf.Network.Timeout.TCP.Set("1s"))
			echo, err := net.Listen("tcp4", "127.0.0.1:0")
			require.NoError(t, err)
			var wg sync.WaitGroup
			wg.Go(func() {
				conn, acceptErr := echo.Accept()
				if acceptErr != nil {
					return
				}
				if deadlineErr := conn.SetDeadline(time.Now().Add(5 * time.Second)); deadlineErr != nil {
					t.Error(deadlineErr)
				}
				if _, copyErr := io.Copy(conn, conn); copyErr != nil {
					t.Error(copyErr)
				}
				if closeErr := conn.Close(); closeErr != nil {
					t.Error(closeErr)
				}
			})
			t.Cleanup(func() { require.NoError(t, echo.Close()); wg.Wait() })
			httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if _, writeErr := io.WriteString(w, "SOCKS destination"); writeErr != nil {
					t.Error(writeErr)
				}
			}))
			t.Cleanup(httpServer.Close)
			listener, err := net.Listen("tcp4", "127.0.0.1:0")
			require.NoError(t, err)
			server := socks5.NewServer(socks5.WithDial(func(ctx context.Context, protocol, address string) (net.Conn, error) {
				// These loopback-only aliases cannot succeed if the configured proxy is bypassed.
				switch address {
				case "127.0.0.1:1":
					address = echo.Addr().String()
				case "127.0.0.1:2":
					address = httpServer.Listener.Addr().String()
				default:
					return nil, net.InvalidAddrError(address)
				}
				return (&net.Dialer{Timeout: time.Second}).DialContext(ctx, protocol, address)
			}))
			done := make(chan error, 1)
			go func() { done <- server.Serve(listener) }()
			t.Cleanup(func() {
				require.NoError(t, listener.Close())
				require.ErrorIs(t, <-done, net.ErrClosed)
			})
			proxy := config.TypeProxyURL{}
			require.NoError(t, proxy.Set("socks5://"+listener.Addr().String()))
			conf.Network.Proxies = append(conf.Network.Proxies, proxy)
			if multiple {
				offline, err := net.Listen("tcp4", "127.0.0.1:0")
				require.NoError(t, err)
				require.NoError(t, offline.Close())
				other := config.TypeProxyURL{}
				require.NoError(t, other.Set("socks5://"+offline.Addr().String()))
				conf.Network.Proxies = append([]config.TypeProxyURL{other}, conf.Network.Proxies...)
			}
			network, err := makeNetwork(conf, "test")
			require.NoError(t, err)
			conn, err := network.DialContext(t.Context(), "tcp", "127.0.0.1:1")
			require.NoError(t, err)
			require.NoError(t, conn.SetDeadline(time.Now().Add(5*time.Second)))
			_, err = conn.Write([]byte("proxy echo"))
			require.NoError(t, err)
			data := make([]byte, len("proxy echo"))
			_, err = io.ReadFull(conn, data)
			require.NoError(t, conn.Close())
			require.NoError(t, err)
			require.Equal(t, "proxy echo", string(data))
			client := network.MakeHTTPClient(nil)
			t.Cleanup(client.CloseIdleConnections)
			request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://127.0.0.1:2", nil)
			require.NoError(t, err)
			response, err := client.Do(request)
			require.NoError(t, err)
			body, err := io.ReadAll(response.Body)
			require.NoError(t, response.Body.Close())
			require.NoError(t, err)
			require.Equal(t, "SOCKS destination", string(body))
		})
	}

	t.Run("user Given an unsupported proxy scheme When configuring the network Then configuration is rejected", func(t *testing.T) {
		conf := &config.Config{}
		conf.Network.Proxies = []config.TypeProxyURL{{
			Value: &url.URL{Scheme: "http", Host: "127.0.0.1"},
		}}

		_, err := makeNetwork(conf, "test")
		require.ErrorContains(t, err, "cannot use")
	})
}

func TestMakeAntiReplayCache(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		t.Run("user Given anti-replay "+map[bool]string{false: "disabled", true: "enabled"}[enabled]+" When submitting identical bytes twice Then only enabled defense rejects the replay", func(t *testing.T) {
			conf := &config.Config{}
			conf.Defense.AntiReplay.Enabled.Value = enabled
			require.NoError(t, conf.Defense.AntiReplay.MaxSize.Set("1KB"))
			require.NoError(t, conf.Defense.AntiReplay.ErrorRate.Set("0.01"))
			cache := makeAntiReplayCache(conf)
			require.False(t, cache.SeenBefore([]byte("handshake bytes")))
			require.Equal(t, enabled, cache.SeenBefore([]byte("handshake bytes")))
		})
	}
}

func TestMakeIPLists(t *testing.T) {
	t.Run("user Given disabled IP defenses When building lists Then IPv4 and IPv6 are unblocked and allowed", func(t *testing.T) {
		log := logger.NewNoopLogger()
		network, err := makeNetwork(&config.Config{}, "test")
		require.NoError(t, err)
		updated := make(chan struct{}, 1)
		callback := ipblocklist.FireholUpdateCallback(func(context.Context, int) { updated <- struct{}{} })

		blocklist, err := makeIPBlocklist(config.ListConfig{}, log, network, callback)
		require.NoError(t, err)
		require.False(t, blocklist.Contains(net.ParseIP("192.0.2.1")))
		require.False(t, blocklist.Contains(net.ParseIP("2001:db8::1")))
		defer blocklist.Shutdown()

		allowlist, err := makeIPAllowlist(config.ListConfig{}, log, network, callback)
		require.NoError(t, err)
		defer allowlist.Shutdown()
		select {
		case <-updated:
		case <-time.After(time.Second):
			t.Fatal("allowlist did not load")
		}
		require.True(t, allowlist.Contains(net.ParseIP("192.0.2.1")))
		require.True(t, allowlist.Contains(net.ParseIP("2001:db8::1")))
	})
}

func TestMakeEventStream(t *testing.T) {
	log := logger.NewNoopLogger()

	t.Run("user Given an invalid StatsD tag format When configuring events Then the invalid observer is rejected", func(t *testing.T) {
		conf := &config.Config{}
		conf.Stats.StatsD.Enabled.Value = true
		conf.Stats.StatsD.TagFormat.Value = "invalid"
		_, err := makeEventStream(conf, log)
		require.ErrorContains(t, err, "statsd")
	})

	t.Run("user Given an invalid Prometheus bind address When configuring events Then the listener error is reported", func(t *testing.T) {
		conf := &config.Config{}
		conf.Stats.Prometheus.Enabled.Value = true
		conf.Stats.Prometheus.BindTo.Value = "invalid address"
		_, err := makeEventStream(conf, log)
		require.ErrorContains(t, err, "prometheus")
	})
}

func TestWarningFastPaths(t *testing.T) {
	level := zerolog.GlobalLevel()
	t.Cleanup(func() { zerolog.SetGlobalLevel(level) })
	t.Run("user Given no secret host or deprecated options When checking warnings Then no warning is emitted", func(t *testing.T) {
		output := testlib.CaptureStdout(func() {
			conf := &config.Config{}
			log := makeLogger(conf)
			warnSNIMismatch(conf, nil, log)
			warnDeprecatedDomainFronting(conf, log)
		})
		require.Empty(t, output)
	})
	t.Run("user Given deprecated domain fronting IP options When checking warnings Then both migration warnings are emitted", func(t *testing.T) {
		output := testlib.CaptureStdout(func() {
			conf := &config.Config{}
			conf.DomainFrontingIP.Value = net.ParseIP("192.0.2.1")
			conf.DomainFronting.IP.Value = net.ParseIP("192.0.2.2")
			warnDeprecatedDomainFronting(conf, makeLogger(conf))
		})
		require.Contains(t, output, `\"domain-fronting-ip\" is deprecated and ignored`)
		require.Contains(t, output, `\"ip\" in [domain-fronting] is deprecated and ignored`)
		require.Contains(t, output, `"level":"warn"`)
	})
}
