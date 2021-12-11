package gqlwsserver

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/graphql-go/graphql"
	"github.com/graphql-go/graphql/language/ast"
	goutils "github.com/onichandame/go-utils"
	gqlwserror "github.com/onichandame/gql-ws/error"
	gqlwsmessage "github.com/onichandame/gql-ws/message"
)

type Socket struct {
	*Config

	reader, writer chan *gqlwsmessage.Message
	breaker        chan error
	done           chan interface{}
	init           chan *gqlwsmessage.Message
	inited         bool
	err            error
	// the connection parameters negotiated on ConnectionInit
	// will inject into every graphql resolver. can be retrieved by context.Value(reflect.Typeof(ConnectionParams{}))
	connectionParams ConnectionParams

	sm *subMan
}

func NewSocket(cfg *Config) *Socket {
	var sock Socket
	cfg.init()
	sock.Config = cfg
	sock.reader = make(chan *gqlwsmessage.Message)
	sock.writer = make(chan *gqlwsmessage.Message)
	sock.init = make(chan *gqlwsmessage.Message)
	sock.breaker = make(chan error)
	sock.done = make(chan interface{})
	sock.sm = newSubMan()
	sock.listen()
	return &sock
}

func (sock *Socket) Close() {
	defer goutils.RecoverToErr(new(error))
	sock.breaker <- errors.New(`closed by user`)
}
func (sock *Socket) Wait() {
	<-sock.done
}
func (sock *Socket) Error() error { return sock.err }

func (sock *Socket) listen() {
	conn := sock.getConn()

	// cleanup
	go func() {
		defer close(sock.done)
		defer conn.Close()
		err := <-sock.breaker
		sock.err = err
		if err != nil {
			if err := conn.WriteControl(websocket.CloseMessage, []byte(err.Error()), time.Now().Add(sock.GraceClosePeriod)); err == nil {
				time.Sleep(sock.GraceClosePeriod)
			}
		}
	}()
	// reader
	go func() {
		var err error
		defer func() {
			defer goutils.RecoverToErr(new(error))
			sock.breaker <- err
		}()
		defer goutils.RecoverToErr(&err)
		for {
			var msg gqlwsmessage.Message
			goutils.Assert(conn.ReadJSON(&msg))
			sock.reader <- &msg
		}
	}()
	// writer
	go func() {
		var err error
		defer func() {
			defer goutils.RecoverToErr(new(error))
			sock.breaker <- err
		}()
		defer goutils.RecoverToErr(&err)
		for msg := range sock.writer {
			goutils.Assert(conn.WriteJSON(msg))
		}
	}()
	// listener
	go func() {
		for {
			select {
			case req := <-sock.reader:
				go sock.handleRequest(req)
			case <-sock.done:
				return
			}
		}
	}()
	// init
	func() {
		var err error
		defer func() {
			defer goutils.RecoverToErr(new(error))
			if err != nil {
				sock.breaker <- err
			}
		}()
		defer goutils.RecoverToErr(&err)
		sock.inited = false
		timeout := make(chan int)
		go func() {
			defer goutils.RecoverToErr(new(error))
			time.Sleep(sock.ConnectionInitTimeout)
			if !sock.inited {
				timeout <- 0
			}
		}()
		select {
		case <-timeout:
			panic(gqlwserror.NewFatalError(4408, `Connection initialisation timeout`))
		case init := <-sock.init:
			sock.OnConnectionInit(init)
			sock.connectionParams = init.Payload
			sock.writer <- &gqlwsmessage.Message{Type: gqlwsmessage.ConnectionAck, Payload: sock.OnConnectionInit(init)}
			sock.inited = true
		}
	}()
}
func (sock *Socket) getConn() *websocket.Conn {
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
func (sock *Socket) handleRequest(msg *gqlwsmessage.Message) {
	var err error
	defer func() {
		defer goutils.RecoverToErr(new(error))
		if err != nil {
			if he, ok := err.(*gqlwserror.HandlableError); ok {
				sock.writer <- he.GetMessage()
			} else {
				sock.breaker <- err
			}
		}
	}()
	defer goutils.RecoverToErr(&err)
	switch msg.Type {
	case gqlwsmessage.ConnectionInit:
		sock.init <- msg
	case gqlwsmessage.Ping:
		var payload gqlwsmessage.Payload
		if sock.OnPing != nil {
			payload = sock.OnPing(msg)
		}
		sock.writer <- &gqlwsmessage.Message{Type: gqlwsmessage.Pong, Payload: payload}
	case gqlwsmessage.Pong:
		if sock.OnPong != nil {
			sock.OnPong(msg)
		}
	case gqlwsmessage.Subscribe:
		if !sock.inited {
			panic(gqlwserror.NewFatalError(4401, `Unauthorized`))
		}
		if msg.ID == nil {
			panic(gqlwserror.NewFatalError(4400, `Subscriber must come with an id`))
		}
		if sock.sm.has(*msg.ID) {
			panic(gqlwserror.NewFatalError(4409, fmt.Sprintf(`Subscriber for %v already exists`, *msg.ID)))
		}
		var query gqlwsmessage.SubscribePayload
		if err := goutils.Try(func() { goutils.UnmarshalJSONFromMap(msg.Payload.(map[string]interface{}), &query) }); err != nil {
			panic(gqlwserror.NewFatalError(4400, `Payload of subscribe request invalid`))
		}
		stopchan := sock.sm.add(*msg.ID)
		defer sock.sm.del(*msg.ID)
		if getOperationTypeOfReq(query.Query) == ast.OperationTypeSubscription {
			params := sock.getGqlParams(&query, stopchan)
			reschan := graphql.Subscribe(*params)
			for {
				res, ok := <-reschan
				if !ok {
					return
				}
				sock.writer <- &gqlwsmessage.Message{Type: gqlwsmessage.Next, Payload: res, ID: msg.ID}
			}
		} else {
			sock.writer <- &gqlwsmessage.Message{Type: gqlwsmessage.Next, ID: msg.ID, Payload: graphql.Do(*sock.getGqlParams(&query, stopchan))}
		}
		sock.writer <- &gqlwsmessage.Message{Type: gqlwsmessage.Complete, ID: msg.ID}
	case gqlwsmessage.Complete:
		if msg.ID == nil {
			panic(gqlwserror.NewFatalError(4400, `complete message must come with an id`))
		}
		sock.sm.del(*msg.ID)
	default:
		panic(gqlwserror.NewFatalError(4400, fmt.Sprintf(`message type %v not supported`, msg.Type)))
	}
}
func (sock *Socket) getGqlParams(q *gqlwsmessage.SubscribePayload, stopchan chan interface{}) *graphql.Params {
	ctx := sock.Context
	if ctx == nil {
		ctx = context.Background()
	}
	return &graphql.Params{
		Schema:         *sock.Schema,
		RequestString:  q.Query,
		VariableValues: q.Variables,
		OperationName:  q.OperationName,
		Context:        context.WithValue(context.WithValue(ctx, connParamsKey, sock.connectionParams), subscriptionStopKey, stopchan),
	}
}
