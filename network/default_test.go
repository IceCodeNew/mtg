package network_test

import (
	"bufio"
	"context"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/IceCodeNew/mtg/network"
	"github.com/stretchr/testify/suite"
)

type DefaultDialerTestSuite struct {
	suite.Suite
	HTTPServerTestSuite

	d network.Dialer
}

func (suite *DefaultDialerTestSuite) SetupSuite() {
	suite.HTTPServerTestSuite.SetupSuite()

	d, err := network.NewDefaultDialer(0, 0)
	suite.NoError(err)

	suite.d = d
}

func (suite *DefaultDialerTestSuite) TestNegativeTimeout() {
	_, err := network.NewDefaultDialer(-1, 0)
	suite.Error(err)
}

func (suite *DefaultDialerTestSuite) TestUnsupportedProtocol() {
	_, err := suite.d.DialContext(context.Background(),
		"udp",
		suite.HTTPServerAddress())
	suite.Error(err)
}

func (suite *DefaultDialerTestSuite) TestCannotDial() {
	_, err := suite.d.DialContext(context.Background(),
		"tcp",
		suite.HTTPServerAddress()+suite.HTTPServerAddress())
	suite.Error(err)
}

func (suite *DefaultDialerTestSuite) TestConnectOk() {
	conn, err := suite.d.DialContext(context.Background(),
		"tcp",
		suite.HTTPServerAddress())
	suite.NoError(err)
	suite.NotNil(conn)

	conn.Close() //nolint: errcheck
}

func (suite *DefaultDialerTestSuite) TestConnectWithoutContext() {
	suite.Run("user Given a loopback HTTP server When dialing without context Then the connection carries a valid HTTP response", func() {
		conn, err := suite.d.Dial("tcp", suite.HTTPServerAddress())
		suite.Require().NoError(err)
		suite.T().Cleanup(func() { suite.NoError(conn.Close()) })
		suite.Require().NoError(conn.SetDeadline(time.Now().Add(time.Second)))
		_, err = io.WriteString(conn, "GET /get HTTP/1.1\r\nHost: localhost\r\nConnection: close\r\n\r\n")
		suite.Require().NoError(err)
		response, err := http.ReadResponse(bufio.NewReader(conn), nil)
		suite.Require().NoError(err)
		_, err = io.ReadAll(response.Body)
		suite.NoError(response.Body.Close())
		suite.NoError(err)
		suite.Equal(http.StatusOK, response.StatusCode)
	})
}

func (suite *DefaultDialerTestSuite) TestHTTPRequest() {
	httpClient := suite.MakeHTTPClient(suite.d)

	resp, err := httpClient.Get(suite.MakeURL("/get")) //nolint: noctx
	if err == nil {
		defer resp.Body.Close() //nolint: errcheck
	}

	suite.NoError(err)
	suite.Equal(http.StatusOK, resp.StatusCode)
}

func TestDefaultDialer(t *testing.T) {
	t.Parallel()
	suite.Run(t, &DefaultDialerTestSuite{})
}
