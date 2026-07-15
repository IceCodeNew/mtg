package config_test

import (
	"net"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/IceCodeNew/mtg/internal/config"
	"github.com/stretchr/testify/suite"
)

type ConfigTestSuite struct {
	suite.Suite
}

func (suite *ConfigTestSuite) ReadConfig(filename string) []byte {
	data, err := os.ReadFile(filepath.Join("testdata", filename))
	suite.NoError(err)

	return data
}

func (suite *ConfigTestSuite) TestParseEmpty() {
	_, err := config.Parse([]byte{})
	suite.Error(err)
}

func (suite *ConfigTestSuite) TestParseBrokenToml() {
	_, err := config.Parse(suite.ReadConfig("broken.toml"))
	suite.Error(err)
}

func (suite *ConfigTestSuite) TestParseOnlySecret() {
	_, err := config.Parse(suite.ReadConfig("only_secret.toml"))
	suite.Error(err)
}

func (suite *ConfigTestSuite) TestParseMinimalConfig() {
	conf, err := config.Parse(suite.ReadConfig("minimal.toml"))
	suite.NoError(err)
	suite.Equal("7oe1GqLy6TBc38CV3jx7q09nb29nbGUuY29t", conf.Secret.Base64())
	suite.Equal("0.0.0.0:3128", conf.BindTo.String())
}

func (suite *ConfigTestSuite) TestParsePublicIP() {
	conf, err := config.Parse(suite.ReadConfig("public_ip.toml"))
	suite.NoError(err)
	suite.Equal("203.0.113.1", conf.PublicIPv4.Get(nil).String())
	suite.Equal("2001:db8::1", conf.PublicIPv6.Get(nil).String())
}

func (suite *ConfigTestSuite) TestParsePublicIPv4Only() {
	conf, err := config.Parse(suite.ReadConfig("public_ip_v4_only.toml"))
	suite.NoError(err)
	suite.Equal("203.0.113.1", conf.PublicIPv4.Get(nil).String())
	suite.Nil(conf.PublicIPv6.Get(nil))
}

func (suite *ConfigTestSuite) TestParsePublicIPInvalid() {
	_, err := config.Parse(suite.ReadConfig("public_ip_invalid.toml"))
	suite.Error(err)
}

func (suite *ConfigTestSuite) TestParsePublicIPNotSet() {
	conf, err := config.Parse(suite.ReadConfig("minimal.toml"))
	suite.NoError(err)
	suite.Nil(conf.PublicIPv4.Get(nil))
	suite.Nil(conf.PublicIPv6.Get(nil))
}

func (suite *ConfigTestSuite) TestString() {
	conf, err := config.Parse(suite.ReadConfig("minimal.toml"))
	suite.NoError(err)
	suite.NotEmpty(conf.String())
}

func (suite *ConfigTestSuite) TestCompatibilityGetters() {
	conf := &config.Config{}

	suite.Equal(uint(7), conf.GetConcurrency(7))
	suite.Equal(uint(443), conf.GetDomainFrontingPort(443))
	suite.False(conf.GetDomainFrontingProxyProtocol(false))
	suite.Nil(conf.GetDNS())

	suite.Require().NoError(conf.Concurrency.Set("11"))
	suite.Require().NoError(conf.DomainFrontingPort.Set("8443"))
	suite.Require().NoError(conf.DomainFronting.Port.Set("9443"))
	conf.DomainFrontingProxyProtocol.Value = true

	suite.Equal(uint(11), conf.GetConcurrency(7))
	suite.Equal(uint(9443), conf.GetDomainFrontingPort(443))
	suite.True(conf.GetDomainFrontingProxyProtocol(false))

	conf.DomainFronting.Port.Value = 0
	suite.Equal(uint(8443), conf.GetDomainFrontingPort(443))

	conf.DomainFrontingProxyProtocol.Value = false
	conf.DomainFronting.ProxyProtocol.Value = true
	suite.True(conf.GetDomainFrontingProxyProtocol(false))

	conf.Network.DOHIP.Value = net.ParseIP("192.0.2.1")
	suite.Equal(&url.URL{Scheme: "https", Host: "192.0.2.1"}, conf.GetDNS())

	suite.Require().NoError(conf.Network.DNS.Set("udp://192.0.2.53"))
	suite.Equal("udp://192.0.2.53", conf.GetDNS().String())
}

func (suite *ConfigTestSuite) TestValidateErrors() {
	conf := &config.Config{}
	suite.ErrorContains(conf.Validate(), "invalid secret")

	suite.Require().NoError(conf.Secret.Set("7oe1GqLy6TBc38CV3jx7q09nb29nbGUuY29t"))
	suite.ErrorContains(conf.Validate(), "incorrect bind-to")
}

func (suite *ConfigTestSuite) TestDomainFrontingIPIgnoredWhenHostSet() {
	conf, err := config.Parse(suite.ReadConfig("minimal.toml"))
	suite.NoError(err)

	suite.NoError(conf.DomainFronting.Host.Set("fronting-backend"))
	suite.NoError(conf.DomainFronting.IP.Set("10.0.0.10"))
	suite.NoError(conf.Validate())
	suite.Equal("fronting-backend", conf.GetDomainFrontingHost())
}

func (suite *ConfigTestSuite) TestDomainFrontingHostFromTOML() {
	conf, err := config.Parse(suite.ReadConfig("domain_fronting_host.toml"))
	suite.NoError(err)
	suite.NoError(conf.Validate())
	suite.Equal("fronting-backend", conf.GetDomainFrontingHost())
}

func (suite *ConfigTestSuite) TestDomainFrontingHostAcceptsLiteralIP() {
	conf, err := config.Parse(suite.ReadConfig("domain_fronting_host_ip.toml"))
	suite.NoError(err)
	suite.NoError(conf.Validate())
	suite.Equal("10.0.0.1", conf.GetDomainFrontingHost())
}

func (suite *ConfigTestSuite) TestDomainFrontingIPIgnoredFromTOML() {
	conf, err := config.Parse(suite.ReadConfig("domain_fronting_ip.toml"))
	suite.NoError(err)
	suite.NoError(conf.Validate())
	// Deprecated [domain-fronting].ip is parsed but never used to derive
	// the dial target — the user must migrate to [domain-fronting].host.
	suite.NotNil(conf.DomainFronting.IP.Get(nil))
	suite.Equal("", conf.GetDomainFrontingHost())
}

func (suite *ConfigTestSuite) TestDomainFrontingNotSet() {
	conf, err := config.Parse(suite.ReadConfig("minimal.toml"))
	suite.NoError(err)
	suite.NoError(conf.Validate())
	suite.Equal("", conf.GetDomainFrontingHost())
}

func TestConfig(t *testing.T) {
	t.Parallel()
	suite.Run(t, &ConfigTestSuite{})
}
