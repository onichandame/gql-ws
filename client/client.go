package gqlwsclient

import (
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/graphql-go/graphql"
	"github.com/graphql-go/graphql/gqlerrors"
	goutils "github.com/onichandame/go-utils"
	gqlwserror "github.com/onichandame/gql-ws/error"
	gqlwsmessage "github.com/onichandame/gql-ws/message"
)

type Client struct {
	*Config

	reader, writer chan *gqlwsmessage.Message
	breaker        chan error
	done           chan interface{}
	init           chan *gqlwsmessage.Message
	sm             *subMan
	inited         bool
	err            error
}

func NewClient(cfg *Config) *Client {
	var c Client
	cfg.init()
	c.Config = cfg
	c.reader = make(chan *gqlwsmessage.Message)
	c.writer = make(chan *gqlwsmessage.Message)
	c.breaker = make(chan error)
	c.done = make(chan interface{})
	c.sm = newSubMan()
	goutils.Retry(func() { c.dial() }, &goutils.RetryConfig{Attempts: uint(cfg.ReconnectAttempts) + 1})
	return &c
}
func (c *Client) Close() {
	defer goutils.RecoverToErr(new(error))
	c.breaker <- errors.New(`terminated by user`)
}
func (c *Client) Wait() {
	<-c.done
}
func (c *Client) Error() error { return c.err }

func (c *Client) dial() {
	conn, _, err := websocket.DefaultDialer.Dial(c.URL, http.Header{"Sec-WebSocket-Protocol": []string{`graphql-transport-ws`}})
	goutils.Assert(err)
	// cleanup
	go func() {
		defer close(c.done)
		defer conn.Close()
		err := <-c.breaker
		c.err = err
		if err != nil {
			if conn.WriteControl(websocket.CloseMessage, []byte(err.Error()), time.Now().Add(c.GraceClosePeriod)) == nil {
				time.Sleep(c.GraceClosePeriod)
			}
		}
	}()
	// reader
	go func() {
		var err error
		defer func() {
			goutils.RecoverToErr(new(error))
			c.breaker <- err
		}()
		defer goutils.RecoverToErr(&err)
		for {
			var msg gqlwsmessage.Message
			goutils.Assert(conn.ReadJSON(&msg))
			c.reader <- &msg
		}
	}()
	// writer
	go func() {
		var err error
		defer func() {
			goutils.RecoverToErr(new(error))
			c.breaker <- err
		}()
		defer goutils.RecoverToErr(&err)
		for {
			msg, ok := <-c.writer
			if !ok {
				return
			}
			goutils.Assert(conn.WriteJSON(msg))
		}
	}()
	// listener
	go func() {
		var err error
		defer func() {
			c.breaker <- err
		}()
		defer goutils.RecoverToErr(&err)
		for res := range c.reader {
			go c.handleResponse(res)
		}
	}()
	// init
	c.init = make(chan *gqlwsmessage.Message)
	func() {
		var err error
		defer func() {
			if err != nil {
				defer goutils.RecoverToErr(new(error))
				c.breaker <- err
			}
		}()
		defer goutils.RecoverToErr(&err)
		c.writer <- &gqlwsmessage.Message{Type: gqlwsmessage.ConnectionInit, Payload: c.OnConnecting()}
		timeout := make(chan interface{})
		go func() {
			time.Sleep(c.ConnectionAckTimeout)
			if !c.inited {
				timeout <- nil
			}
		}()
		select {
		case <-timeout:
			panic(errors.New(`connection ack timeout`))
		case ack := <-c.init:
			c.OnConnected(ack)
			c.inited = true
		}
	}()
}

// returns unsubscribe function
func (c *Client) Subscribe(payload gqlwsmessage.SubscribePayload, handlers Handlers) func() {
	if handlers.OnComplete == nil {
		handlers.OnComplete = func() {}
	}
	if handlers.OnError == nil {
		handlers.OnError = func(fe gqlerrors.FormattedErrors) {}
	}
	if handlers.OnNext == nil {
		handlers.OnNext = func(r *graphql.Result) {}
	}
	id := uuid.NewString()
	c.writer <- &gqlwsmessage.Message{Type: gqlwsmessage.Subscribe, ID: &id, Payload: payload}
	c.sm.set(id, &handlers)
	return func() {
		c.writer <- &gqlwsmessage.Message{Type: gqlwsmessage.Complete, ID: &id}
		c.sm.del(id)
	}
}

func (c *Client) handleResponse(msg *gqlwsmessage.Message) {
	var err error
	defer func() {
		defer goutils.RecoverToErr(new(error))
		if err != nil {
			c.breaker <- err
		}
	}()
	defer goutils.RecoverToErr(&err)
	switch msg.Type {
	case gqlwsmessage.ConnectionAck:
		c.init <- msg
	case gqlwsmessage.Ping:
		c.writer <- &gqlwsmessage.Message{Type: gqlwsmessage.Pong, Payload: c.OnPing(msg)}
	case gqlwsmessage.Pong:
		c.OnPong(msg)
	case gqlwsmessage.Next:
		if msg.ID == nil {
			panic(gqlwserror.NewFatalError(4400, `next gqlwsmessage must come with id`))
		}
		hdl := c.sm.get(*msg.ID)
		if hdl == nil {
			panic(gqlwserror.NewFatalError(4400, `subscription not found`))
		}
		var payload graphql.Result
		if err := goutils.Try(func() { goutils.UnmarshalJSONFromMap(msg.Payload.(map[string]interface{}), &payload) }); err != nil {
			panic(gqlwserror.NewFatalError(4400, `payload of next response invalid`))
		}
		if payload.Errors != nil {
			hdl.OnError(payload.Errors)
		} else {
			hdl.OnNext(&payload)
		}
	case gqlwsmessage.Error:
		if msg.ID == nil {
			panic(gqlwserror.NewFatalError(4400, `error gqlwsmessage must come with id`))
		}
		hdl := c.sm.get(*msg.ID)
		if hdl == nil {
			panic(errors.New(`subscription not found`))
		}
		payload := gqlerrors.FormattedErrors{}
		if err := goutils.Try(func() { goutils.UnmarshalJSONFromMap(msg.Payload.(map[string]interface{}), &payload) }); err != nil {
			panic(errors.New(`payload of error response invalid`))
		}
		hdl.OnError(payload)
	case gqlwsmessage.Complete:
		if msg.ID == nil {
			panic(errors.New(`error gqlwsmessage must come with id`))
		}
		hdl := c.sm.get(*msg.ID)
		if hdl == nil {
			panic(errors.New(`subscription not found`))
		}
		defer c.sm.del(*msg.ID)
		hdl.OnComplete()
	default:
		panic(errors.New(`invalid gqlwsmessage type`))
	}
}
