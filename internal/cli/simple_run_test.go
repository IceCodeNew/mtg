package cli

import (
	"net"
	"strings"
	"testing"
	"time"

	"github.com/IceCodeNew/mtg/internal/testlib"
	"github.com/stretchr/testify/require"
)

const testSecret = "7oe1GqLy6TBc38CV3jx7q09nb29nbGUuY29t"

func validSimpleRun() SimpleRun {
	return SimpleRun{
		BindTo:              "127.0.0.1:3128",
		Secret:              testSecret,
		Concurrency:         1,
		PreferIP:            "prefer-ipv6",
		DomainFrontingPort:  443,
		DOHIP:               net.ParseIP("192.0.2.1"),
		Timeout:             time.Second,
		AntiReplayCacheSize: "1MB",
	}
}

func TestSimpleRunValidationErrors(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*SimpleRun)
		want   string
	}{
		{"bind", func(s *SimpleRun) { s.BindTo = "bad" }, "incorrect bind-to"},
		{"secret", func(s *SimpleRun) { s.Secret = "bad" }, "incorrect secret"},
		{"concurrency", func(s *SimpleRun) { s.Concurrency = 65536 }, "incorrect concurrency"},
		{"prefer ip", func(s *SimpleRun) { s.PreferIP = "sometimes" }, "incorrect prefer-ip"},
		{"fronting port", func(s *SimpleRun) { s.DomainFrontingPort = 65536 }, "incorrect domain-fronting-port"},
		{"fronting host", func(s *SimpleRun) { s.DomainFrontingHost = "host:443" }, "incorrect domain-fronting-host"},
		{"deprecated fronting ip", func(s *SimpleRun) { s.DomainFrontingIP = "invalid" }, "incorrect domain-fronting-ip"},
		{"doh ip", func(s *SimpleRun) { s.DOHIP = nil }, "incorrect doh-ip"},
		{"timeout", func(s *SimpleRun) { s.Timeout = -time.Second }, "incorrect timeout"},
		{"antireplay", func(s *SimpleRun) { s.AntiReplayCacheSize = "many" }, "incorrect antireplay-cache-size"},
		{"proxy", func(s *SimpleRun) { s.Socks5Proxies = []string{"http://proxy"} }, "incorrect socks5 proxy URL"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			command := validSimpleRun()
			tt.mutate(&command)

			err := command.Run(&CLI{}, "test")
			require.ErrorContains(t, err, tt.want)
		})
	}
}

func TestGenerateSecret(t *testing.T) {
	for _, hex := range []bool{false, true} {
		t.Run(map[bool]string{false: "base64", true: "hex"}[hex], func(t *testing.T) {
			command := GenerateSecret{HostName: "example.com", Hex: hex}
			cli := &CLI{GenerateSecret: command}

			output := testlib.CaptureStdout(func() {
				require.NoError(t, command.Run(cli, "test"))
			})
			require.NotEmpty(t, strings.TrimSpace(output))
		})
	}
}
