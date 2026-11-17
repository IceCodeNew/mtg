package network_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/IceCodeNew/mtg/network"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

const externalHTTPBinHost = "httpbin.io"

type headerValues []string

func (values *headerValues) UnmarshalJSON(data []byte) error {
	var multiple []string
	if err := json.Unmarshal(data, &multiple); err == nil {
		*values = multiple

		return nil
	}

	var single string
	if err := json.Unmarshal(data, &single); err != nil {
		return err
	}

	*values = []string{single}

	return nil
}

func TestHeaderValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		json     string
		expected headerValues
	}{
		{name: "single", json: `"itsme"`, expected: headerValues{"itsme"}},
		{name: "array", json: `["itsme"]`, expected: headerValues{"itsme"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var actual headerValues
			require.NoError(t, json.Unmarshal([]byte(test.json), &actual))
			assert.Equal(t, test.expected, actual)
		})
	}
}

type NetworkTestSuite struct {
	suite.Suite
	HTTPServerTestSuite

	dialer network.Dialer
}

func (suite *NetworkTestSuite) SetupTest() {
	dialer, err := network.NewDefaultDialer(0, 0)
	suite.NoError(err)

	suite.dialer = dialer
}

func (suite *NetworkTestSuite) TestLocalHTTPRequest() {
	ntw, err := network.NewNetwork(suite.dialer, "itsme", "1.1.1.1", 0)
	suite.NoError(err)

	client := ntw.MakeHTTPClient(nil)

	resp, err := client.Get(suite.httpServer.URL + "/headers") //nolint: noctx
	suite.NoError(err)

	defer resp.Body.Close() //nolint: errcheck

	data, err := io.ReadAll(resp.Body)
	suite.NoError(err)
	suite.Equal(http.StatusOK, resp.StatusCode)

	jsonStruct := struct {
		Headers struct {
			UserAgent []string `json:"User-Agent"` //nolint: tagliatelle
		} `json:"headers"`
	}{}

	suite.NoError(json.Unmarshal(data, &jsonStruct))
	suite.Equal([]string{"itsme"}, jsonStruct.Headers.UserAgent)
}

func (suite *NetworkTestSuite) TestRealHTTPRequest() {
	ntw, err := network.NewNetwork(suite.dialer, "itsme", "1.1.1.1", 0)
	suite.NoError(err)

	client := ntw.MakeHTTPClient(nil)

	externalURL := (&url.URL{
		Scheme: "https",
		Host:   externalHTTPBinHost,
		Path:   "/headers",
	}).String()

	var resp *http.Response
	suite.Require().Eventually(func() bool {
		candidate, err := client.Get(externalURL) //nolint: noctx
		if err != nil {
			if candidate != nil {
				candidate.Body.Close() //nolint: errcheck
			}

			return false
		}
		if candidate.StatusCode == http.StatusOK {
			resp = candidate

			return true
		}

		candidate.Body.Close() //nolint: errcheck

		return false
	}, 30*time.Second, time.Second)

	defer resp.Body.Close() //nolint: errcheck
	suite.Require().Equal(http.StatusOK, resp.StatusCode)

	data, err := io.ReadAll(resp.Body)
	suite.NoError(err)
	jsonStruct := struct {
		Headers struct {
			UserAgent headerValues `json:"User-Agent"` //nolint: tagliatelle
		} `json:"headers"`
	}{}

	suite.NoError(json.Unmarshal(data, &jsonStruct))
	suite.Equal(headerValues{"itsme"}, jsonStruct.Headers.UserAgent)
}

func (suite *NetworkTestSuite) TestIncorrectTimeout() {
	_, err := network.NewNetwork(suite.dialer, "itsme", "1.1.1.1", -time.Second)
	suite.Error(err)
}

func (suite *NetworkTestSuite) TestIncorrectDOHHostname() {
	_, err := network.NewNetwork(suite.dialer, "itsme", "doh.com", 0)
	suite.Error(err)
}

func TestNetwork(t *testing.T) {
	t.Parallel()
	suite.Run(t, &NetworkTestSuite{})
}
