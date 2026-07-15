package cli

import (
	"context"
	"errors"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/IceCodeNew/mtg/essentials"
	"github.com/IceCodeNew/mtg/internal/config"
	"github.com/IceCodeNew/mtg/internal/testlib"
	"github.com/IceCodeNew/mtg/mtglib"
	"github.com/stretchr/testify/require"
)

type doctorNetwork struct {
	dialer *net.Dialer
	err    error
}

func (n doctorNetwork) Dial(string, string) (essentials.Conn, error) {
	return nil, n.err
}

func (n doctorNetwork) DialContext(context.Context, string, string) (essentials.Conn, error) {
	return nil, n.err
}

func (n doctorNetwork) NativeDialer() *net.Dialer { return n.dialer }

func (n doctorNetwork) MakeHTTPClient(
	func(context.Context, string, string) (essentials.Conn, error),
) *http.Client {
	return &http.Client{}
}

var _ mtglib.Network = doctorNetwork{}

func TestDoctorCheckDeprecatedConfig(t *testing.T) {
	t.Run("current config", func(t *testing.T) {
		doctor := Doctor{conf: &config.Config{}}
		output := testlib.CaptureStdout(func() {
			require.True(t, doctor.checkDeprecatedConfig())
		})
		require.Contains(t, output, "All good")
	})

	t.Run("all deprecated options", func(t *testing.T) {
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
	doctor := Doctor{conf: &config.Config{}}
	output := testlib.CaptureStdout(func() {
		require.False(t, doctor.checkNetwork(doctorNetwork{err: errors.New("offline")}))
	})
	require.Contains(t, output, "offline")
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
		t.Run(tt.preference, func(t *testing.T) {
			conf := &config.Config{}
			require.NoError(t, conf.PreferIP.Set(tt.preference))
			doctor := Doctor{conf: conf}

			_, err := doctor.checkNetworkAddresses(
				doctorNetwork{err: errors.New("should not dial")}, 1, tt.addresses,
			)
			require.ErrorContains(t, err, "no suitable addresses")
		})
	}
}

func TestDoctorCheckFrontingDomain(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	port := uint(listener.Addr().(*net.TCPAddr).Port)
	conf := &config.Config{}
	require.NoError(t, conf.Secret.Set(testSecret))
	require.NoError(t, conf.DomainFronting.Host.Set("127.0.0.1"))
	conf.DomainFronting.Port.Value = port
	doctor := Doctor{conf: conf}
	network := doctorNetwork{dialer: &net.Dialer{Timeout: time.Second}}

	accepted := make(chan struct{})
	go func() {
		conn, acceptErr := listener.Accept()
		if acceptErr == nil {
			conn.Close() //nolint:errcheck
		}
		close(accepted)
	}()
	require.True(t, doctor.checkFrontingDomain(network))
	<-accepted
	require.NoError(t, listener.Close())
	require.False(t, doctor.checkFrontingDomain(network))
}
