package gqlwsclient

import (
	"errors"
	"net/url"
	"strings"
	"time"

	gqlwsmessage "github.com/onichandame/gql-ws/message"
)

type Config struct {
	URL                  string
	ConnectionAckTimeout time.Duration
	GraceClosePeriod     time.Duration
	// maximum retry attempts before a connection is established
	ReconnectAttempts uint32
	// OnConnecting called on connection init
	// returns the payload to send in the init request
	OnConnecting        func() interface{}
	OnPing              func(*gqlwsmessage.Message) interface{}
	OnPong, OnConnected func(*gqlwsmessage.Message)
}

func (c *Config) init() {
	if u, err := url.Parse(c.URL); err != nil {
		panic(err)
	} else {
		if !strings.HasPrefix(u.Scheme, "ws") {
			panic(errors.New(`gql-ws must be configured to a websocket endpoint`))
		}
	}
	if c.ConnectionAckTimeout <= 0 {
		c.ConnectionAckTimeout = time.Second * 30
	}
	if c.GraceClosePeriod <= 0 {
		c.GraceClosePeriod = time.Second * 5
	}
	if c.OnConnecting == nil {
		c.OnConnecting = func() interface{} { return nil }
	}
	if c.OnConnected == nil {
		c.OnConnected = func(m *gqlwsmessage.Message) {}
	}
	if c.OnPing == nil {
		c.OnPing = func(m *gqlwsmessage.Message) interface{} { return nil }
	}
	if c.OnPong == nil {
		c.OnPong = func(m *gqlwsmessage.Message) {}
	}
}
