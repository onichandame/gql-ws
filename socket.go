package gqlws

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/graphql-go/graphql"
	"github.com/graphql-go/graphql/language/ast"
	goutils "github.com/onichandame/go-utils"
	"github.com/onichandame/gql-ws/message"
)

type Socket struct {
	// not nullable
	Response http.ResponseWriter
	Request  *http.Request
	Schema   *graphql.Schema

	GraceClosePeriod      time.Duration
	ConnectionInitTimeout time.Duration
	// OnConnectionInit is called if a valid ConnectionInit message is received.
	// returns the payload to return to the client
	OnConnectionInit func(*message.Message) message.Payload
	OnPing           func(*message.Message) message.Payload
	OnPong           func(*message.Message)
	// Context is passed to resolvers
	Context context.Context

	reader  chan *message.Message
	writer  chan *message.Message
	breaker chan error
	// state if ConnectionInit message is received
	inited int32
	// err the final error
	err error
	// the connection parameters negotiated on ConnectionInit
	// will inject into every graphql resolver. can be retrieved by context.Value(reflect.Typeof(ConnectionParams{}))
	connectionParams ConnectionParams

	sm *subscriptionManager
}

// Listen starts the event loop until error
func (sock *Socket) Listen() {
	conn := sock.dial()
	defer conn.Close()
	sock.validateSchema()
	sock.sm = newSubscriptionManager()

	sock.reader = make(chan *message.Message)
	sock.writer = make(chan *message.Message)
	sock.breaker = make(chan error)
	sock.inited = 0
	finished := make(chan int)
	// reader
	go func() {
	LOOP:
		for {
			var msg message.Message
			if err := conn.ReadJSON(&msg); err == nil {
				sock.reader <- &msg
			} else {
				defer close(sock.reader)
				go func() {
					defer goutils.RecoverToErr(new(error))
					sock.breaker <- NewFatalError(4400, `message invalid`)
				}()
				break LOOP
			}
		}
	}()
	// writer
	go func() {
	LOOP:
		for {
			select {
			case msg := <-sock.writer:
				if err := conn.WriteJSON(msg); err != nil {
					sock.breaker <- NewFatalError(4400, `failed to send message`)
				}
			case err := <-sock.breaker:
				defer close(sock.writer)
				defer close(sock.breaker)
				sock.err = err
				timeout := sock.GraceClosePeriod
				if timeout < time.Second*2 {
					timeout = time.Second * 2
				}
				if err := conn.WriteControl(websocket.CloseMessage, []byte(err.Error()), time.Now().Add(timeout)); err == nil {
					time.Sleep(sock.GraceClosePeriod)
				}
				close(finished)
				break LOOP
			}
		}
	}()
	// timeout
	go func() {
		time.Sleep(sock.ConnectionInitTimeout)
		if sock.inited == 0 {
			sock.breaker <- NewFatalError(4408, `Connection initialisation timeout`)
		}
	}()
	// listener
	for {
		req, ok := <-sock.reader
		if !ok {
			break
		}
		go sock.handleRequest(req)
	}
	<-finished
}
func (sock *Socket) Error() error { return sock.err }

func (sock *Socket) dial() *websocket.Conn {
	if sock.Response == nil || sock.Request == nil {
		panic(errors.New(`response and request must be present`))
	}
	upgrader := websocket.Upgrader{
		ReadBufferSize:   1024,
		WriteBufferSize:  1024,
		CheckOrigin:      func(r *http.Request) bool { return true },
		HandshakeTimeout: time.Second * 5,
		Subprotocols:     []string{`graphql-transport-ws`}, // internalize
	}
	conn, err := upgrader.Upgrade(sock.Response, sock.Request, nil)
	goutils.Assert(err)
	if conn.Subprotocol() != `graphql-transport-ws` {
		panic(errors.New(`subprotocol must be graphql-transport-ws`))
	}
	return conn
}
func (sock *Socket) validateSchema() { //TODO: more validation
	if sock.Schema == nil {
		panic(errors.New(`graphql schema must be present`))
	}
}
func (sock *Socket) handleRequest(msg *message.Message) {
	var err error
	defer func() {
		defer goutils.RecoverToErr(new(error)) // do not panic on failure
		if err != nil {
			if he, ok := err.(*HandleError); ok {
				sock.writer <- he.getMessage()
			} else {
				sock.breaker <- err
			}
		}
	}()
	defer goutils.RecoverToErr(&err)
	switch msg.Type {
	case message.ConnectionInit:
		if sock.inited != 0 {
			panic(NewFatalError(4429, `Too many initialisation requests`))
		} else {
			atomic.CompareAndSwapInt32(&sock.inited, 0, 1)
			sock.connectionParams = msg.Payload
			var payload message.Payload
			if sock.OnConnectionInit != nil {
				payload = sock.OnConnectionInit(msg)
			}
			sock.writer <- &message.Message{Type: message.ConnectionAck, Payload: payload}
		}
	case message.Ping:
		var payload message.Payload
		if sock.OnPing != nil {
			payload = sock.OnPing(msg)
		}
		sock.writer <- &message.Message{Type: message.Pong, Payload: payload}
	case message.Pong:
		if sock.OnPong != nil {
			sock.OnPong(msg)
		}
	case message.Subscribe:
		if sock.inited == 0 {
			panic(NewFatalError(4401, `Unauthorized`))
		}
		if msg.ID == nil {
			panic(NewFatalError(4400, `subscription must come with an id`))
		}
		if sock.sm.has(*msg.ID) {
			panic(NewFatalError(4409, fmt.Sprintf(`Subscriber for %v already exists`, *msg.ID)))
		}
		var query message.SubscribePayload
		if pmap, ok := msg.Payload.(map[string]interface{}); !ok {
			panic(NewFatalError(4400, `payload of subscribe must not be empty`))
		} else {
			if err := goutils.Try(func() { goutils.UnmarshalJSONFromMap(pmap, &query) }); err != nil {
				panic(NewFatalError(4400, `payload of subscribe request invalid`))
			}
		}
		sock.sm.add(*msg.ID)
		defer sock.sm.del(*msg.ID)
		if getOperationTypeOfReq(query.Query) == ast.OperationTypeSubscription {
			reschan := graphql.Subscribe(*sock.getGqlParams(&query))
			for res := range reschan {
				sock.writer <- &message.Message{Type: message.Next, Payload: res, ID: msg.ID}
			}
		} else {
			sock.writer <- &message.Message{Type: message.Next, ID: msg.ID, Payload: graphql.Do(*sock.getGqlParams(&query))}
		}
		sock.writer <- &message.Message{Type: message.Complete, ID: msg.ID}
	default:
		panic(NewFatalError(4400, fmt.Sprintf(`message type %v not supported`, msg.Type)))
	}
}
func (sock *Socket) getGqlParams(q *message.SubscribePayload) *graphql.Params {
	ctx := sock.Context
	if ctx == nil {
		ctx = context.Background()
	}
	return &graphql.Params{
		Schema:         *sock.Schema,
		RequestString:  q.Query,
		VariableValues: q.Variables,
		OperationName:  q.OperationName,
		Context:        context.WithValue(ctx, connParamsKey, sock.connectionParams),
	}
}
