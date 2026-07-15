package cli

import (
	"context"
	"net/url"
	"testing"

	"github.com/IceCodeNew/mtg/internal/config"
	"github.com/IceCodeNew/mtg/ipblocklist"
	"github.com/IceCodeNew/mtg/logger"
	"github.com/stretchr/testify/require"
)

func TestMakeLogger(t *testing.T) {
	require.NotNil(t, makeLogger(&config.Config{}))
	conf := &config.Config{}
	conf.Debug.Value = true
	require.NotNil(t, makeLogger(conf))
}

func TestMakeNetwork(t *testing.T) {
	t.Run("direct", func(t *testing.T) {
		network, err := makeNetwork(&config.Config{}, "test")
		require.NoError(t, err)
		require.NotNil(t, network)
	})

	t.Run("one proxy", func(t *testing.T) {
		conf := &config.Config{}
		proxy := config.TypeProxyURL{}
		require.NoError(t, proxy.Set("socks5://127.0.0.1:1080"))
		conf.Network.Proxies = []config.TypeProxyURL{proxy}

		network, err := makeNetwork(conf, "test")
		require.NoError(t, err)
		require.NotNil(t, network)
	})

	t.Run("multiple proxies", func(t *testing.T) {
		conf := &config.Config{}
		for _, endpoint := range []string{"socks5://127.0.0.1:1080", "socks5://127.0.0.1:1081"} {
			proxy := config.TypeProxyURL{}
			require.NoError(t, proxy.Set(endpoint))
			conf.Network.Proxies = append(conf.Network.Proxies, proxy)
		}

		network, err := makeNetwork(conf, "test")
		require.NoError(t, err)
		require.NotNil(t, network)
	})

	t.Run("invalid proxy", func(t *testing.T) {
		conf := &config.Config{}
		conf.Network.Proxies = []config.TypeProxyURL{{
			Value: &url.URL{Scheme: "http", Host: "127.0.0.1"},
		}}

		_, err := makeNetwork(conf, "test")
		require.ErrorContains(t, err, "cannot use")
	})
}

func TestMakeAntiReplayCache(t *testing.T) {
	conf := &config.Config{}
	require.NotNil(t, makeAntiReplayCache(conf))

	conf.Defense.AntiReplay.Enabled.Value = true
	require.NoError(t, conf.Defense.AntiReplay.MaxSize.Set("1KB"))
	require.NoError(t, conf.Defense.AntiReplay.ErrorRate.Set("0.01"))
	require.NotNil(t, makeAntiReplayCache(conf))
}

func TestMakeIPLists(t *testing.T) {
	log := logger.NewNoopLogger()
	network, err := makeNetwork(&config.Config{}, "test")
	require.NoError(t, err)
	callback := ipblocklist.FireholUpdateCallback(func(context.Context, int) {})

	blocklist, err := makeIPBlocklist(config.ListConfig{}, log, network, callback)
	require.NoError(t, err)
	require.NotNil(t, blocklist)
	defer blocklist.Shutdown()

	allowlist, err := makeIPAllowlist(config.ListConfig{}, log, network, callback)
	require.NoError(t, err)
	require.NotNil(t, allowlist)
	defer allowlist.Shutdown()
}

func TestMakeEventStream(t *testing.T) {
	log := logger.NewNoopLogger()

	stream, err := makeEventStream(&config.Config{}, log)
	require.NoError(t, err)
	require.NotNil(t, stream)

	t.Run("invalid statsd", func(t *testing.T) {
		conf := &config.Config{}
		conf.Stats.StatsD.Enabled.Value = true
		conf.Stats.StatsD.TagFormat.Value = "invalid"
		_, err := makeEventStream(conf, log)
		require.ErrorContains(t, err, "statsd")
	})

	t.Run("invalid prometheus listener", func(t *testing.T) {
		conf := &config.Config{}
		conf.Stats.Prometheus.Enabled.Value = true
		conf.Stats.Prometheus.BindTo.Value = "invalid address"
		_, err := makeEventStream(conf, log)
		require.ErrorContains(t, err, "prometheus")
	})
}

func TestWarningFastPaths(t *testing.T) {
	conf := &config.Config{}
	warnSNIMismatch(conf, nil, logger.NewNoopLogger())

	conf.DomainFrontingIP.Value = []byte{192, 0, 2, 1}
	conf.DomainFronting.IP.Value = []byte{192, 0, 2, 2}
	warnDeprecatedDomainFronting(conf, logger.NewNoopLogger())
}
