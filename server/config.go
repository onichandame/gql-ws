package gqlwsserver

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/graphql-go/graphql"
	gqlwsmessage "github.com/onichandame/gql-ws/message"
)

type Config struct {
	Response http.ResponseWriter
	Request  *http.Request
	Schema   *graphql.Schema

	GraceClosePeriod, ConnectionInitTimeout time.Duration

	OnConnectionInit, OnPing func(*gqlwsmessage.Message) gqlwsmessage.Payload
	OnPong                   func(*gqlwsmessage.Message)
	// Context is passed to resolvers. can be used to pass context-related values
	Context context.Context
}

var defaultConfig = Config{
	GraceClosePeriod:      time.Second * 5,
	ConnectionInitTimeout: time.Second * 30,
	Context:               context.Background(),
}

func (c *Config) init() {
	if c.GraceClosePeriod <= 0 {
		c.GraceClosePeriod = time.Second * 5
	}
	if c.ConnectionInitTimeout <= 0 {
		c.ConnectionInitTimeout = time.Second * 30
	}
	if c.Context == nil {
		c.Context = defaultConfig.Context
	}
	if c.Response == nil ||
		c.Request == nil ||
		c.Schema == nil {
		panic(errors.New(`gql-ws socket received invalid parameters`))
	}
	if c.OnConnectionInit == nil {
		c.OnConnectionInit = func(m *gqlwsmessage.Message) gqlwsmessage.Payload { return nil }
	}
	if c.OnPing == nil {
		c.OnPing = func(m *gqlwsmessage.Message) gqlwsmessage.Payload { return nil }
	}
	if c.OnPong == nil {
		c.OnPong = func(m *gqlwsmessage.Message) {}
	}
}
