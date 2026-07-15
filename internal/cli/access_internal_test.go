package cli

import (
	"encoding/json"
	"net"
	"net/url"
	"testing"

	"github.com/IceCodeNew/mtg/internal/config"
	"github.com/IceCodeNew/mtg/internal/testlib"
	"github.com/stretchr/testify/require"
)

func TestAccessMakeURLs(t *testing.T) {
	conf := &config.Config{}
	require.NoError(t, conf.Secret.Set(testSecret))
	require.NoError(t, conf.BindTo.Set("127.0.0.1:3128"))

	require.Nil(t, (&Access{}).makeURLs(conf, nil))

	base64URLs := (&Access{}).makeURLs(conf, net.ParseIP("192.0.2.1"))
	require.Equal(t, uint(3128), base64URLs.Port)
	require.Equal(t, "192.0.2.1", base64URLs.IP.String())
	require.Contains(t, base64URLs.TgQrCode, url.QueryEscape(base64URLs.TgURL))
	require.Contains(t, base64URLs.TmeQrCode, url.QueryEscape(base64URLs.TmeURL))
	require.Contains(t, base64URLs.TgURL, url.QueryEscape(conf.Secret.Base64()))

	hexURLs := (&Access{Port: 8443, Hex: true}).makeURLs(conf, net.ParseIP("2001:db8::1"))
	require.Equal(t, uint(8443), hexURLs.Port)
	require.Contains(t, hexURLs.TgURL, conf.Secret.Hex())
}

func TestAccessRunWithExplicitPublicAddresses(t *testing.T) {
	command := Access{
		ConfigPath: "../config/testdata/minimal.toml",
		PublicIPv4: net.ParseIP("192.0.2.1"),
		PublicIPv6: net.ParseIP("2001:db8::1"),
		Port:       8443,
	}

	output := testlib.CaptureStdout(func() {
		require.NoError(t, command.Run(&CLI{}, "test"))
	})
	response := &accessResponse{}
	require.NoError(t, json.Unmarshal([]byte(output), response))
	require.Equal(t, "192.0.2.1", response.IPv4.IP.String())
	require.Equal(t, "2001:db8::1", response.IPv6.IP.String())
	require.Equal(t, uint(8443), response.IPv4.Port)
	require.NotEmpty(t, response.Secret.Base64)
	require.NotEmpty(t, response.Secret.Hex)
}

func TestAccessRunRejectsUnreadableConfig(t *testing.T) {
	err := (&Access{ConfigPath: "missing.toml"}).Run(&CLI{}, "test")
	require.ErrorContains(t, err, "cannot init config")
}
