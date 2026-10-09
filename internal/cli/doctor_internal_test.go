package cli

import (
	"errors"
	"net"
	"testing"
	"time"

	"github.com/IceCodeNew/mtg/internal/config"
	"github.com/IceCodeNew/mtg/internal/testlib"
	"github.com/IceCodeNew/mtg/network/v2"
	"github.com/stretchr/testify/require"
)

func TestDoctorCheckDeprecatedConfig(t *testing.T) {
	t.Run("user Given current config When doctor checks deprecated options Then it reports no migration needed", func(t *testing.T) {
		doctor := Doctor{conf: &config.Config{}}
		output := testlib.CaptureStdout(func() {
			require.True(t, doctor.checkDeprecatedConfig())
		})
		require.Contains(t, output, "All good")
	})

	t.Run("user Given deprecated options When doctor checks config Then it reports each migration", func(t *testing.T) {
		conf := &config.Config{}
		conf.DomainFrontingIP.Value = net.ParseIP("192.0.2.1")
		conf.DomainFronting.IP.Value = net.ParseIP("192.0.2.2")
		conf.DomainFrontingPort.Value = 443
		conf.DomainFrontingProxyProtocol.Value = true
		conf.Network.DOHIP.Value = net.ParseIP("192.0.2.53")

		doctor := Doctor{conf: conf}
		output := testlib.CaptureStdout(func() {
			require.False(t, doctor.checkDeprecatedConfig())
		})
		for _, option := range []string{
			"domain-fronting-ip",
			"domain-fronting-port",
			"domain-fronting-proxy-protocol",
			"doh-ip",
		} {
			require.Contains(t, output, option)
		}
	})
}

func TestDoctorCheckNetworkFailures(t *testing.T) {
	t.Run("user Given an offline SOCKS proxy When doctor checks connectivity Then it reports connection failure without contacting Telegram", func(t *testing.T) {
		listener, err := net.Listen("tcp4", "127.0.0.1:0")
		require.NoError(t, err)
		address := listener.Addr().String()
		require.NoError(t, listener.Close())
		conf := &config.Config{}
		proxy := config.TypeProxyURL{}
		require.NoError(t, proxy.Set("socks5://"+address))
		conf.Network.Proxies = []config.TypeProxyURL{proxy}
		require.NoError(t, conf.Network.Timeout.TCP.Set("1s"))
		ntw, err := makeNetwork(conf, "test")
		require.NoError(t, err)
		doctor := Doctor{conf: conf}
		output := testlib.CaptureStdout(func() {
			require.False(t, doctor.checkNetwork(ntw))
		})
		require.Contains(t, output, address)
		require.Contains(t, output, "connection refused")
	})
}

func TestDoctorCheckNetworkAddressesFiltersIPFamilies(t *testing.T) {
	addresses := []string{"192.0.2.1:443", "[2001:db8::1]:443"}

	for _, tt := range []struct {
		preference string
		addresses  []string
	}{
		{"only-ipv4", addresses[1:]},
		{"only-ipv6", addresses[:1]},
	} {
		t.Run("user Given "+tt.preference+" and only opposite-family addresses When doctor checks connectivity Then no suitable address is reported", func(t *testing.T) {
			conf := &config.Config{}
			require.NoError(t, conf.PreferIP.Set(tt.preference))
			doctor := Doctor{conf: conf}

			ntw, err := makeNetwork(conf, "test")
			require.NoError(t, err)
			_, err = doctor.checkNetworkAddresses(ntw, 1, tt.addresses)
			require.ErrorContains(t, err, "no suitable addresses")
		})
	}
}

func TestDoctorCheckFrontingDomain(t *testing.T) {
	t.Run("user Given a TCP-only fronting server When doctor checks it Then TLS validation fails", func(t *testing.T) {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		t.Cleanup(func() {
			if err := listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
				t.Error(err)
			}
		})

		port := uint(listener.Addr().(*net.TCPAddr).Port)
		conf := &config.Config{}
		require.NoError(t, conf.Secret.Set(testSecret))
		require.NoError(t, conf.DomainFronting.Host.Set("127.0.0.1"))
		conf.DomainFronting.Port.Value = port
		doctor := Doctor{conf: conf}
		network := network.New(nil, "test", time.Second, time.Second, 0, network.DefaultKeepAliveConfig, 0)

		accepted := make(chan struct{})
		go func() {
			defer close(accepted)
			for {
				conn, acceptErr := listener.Accept()
				if acceptErr != nil {
					return
				}
				if err := conn.Close(); err != nil {
					t.Error(err)
				}
			}
		}()
		output := testlib.CaptureStdout(func() {
			require.False(t, doctor.checkFrontingDomain(network))
		})
		require.Contains(t, output, "is reachable")
		require.Contains(t, output, "TLS certificate for google.com is invalid")
		require.NoError(t, listener.Close())
		<-accepted
		require.False(t, doctor.checkFrontingDomain(network))
	})
}
