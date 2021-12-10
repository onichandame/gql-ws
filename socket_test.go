package gqlws_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/graphql-go/graphql"
	goutils "github.com/onichandame/go-utils"
	gqlws "github.com/onichandame/gql-ws"
	"github.com/onichandame/gql-ws/message"
	"github.com/stretchr/testify/assert"
)

func TestSocket(t *testing.T) {
	ConnectionInitTimeout := time.Millisecond * 500
	eng := gin.Default()
	eng.GET("", func(c *gin.Context) {
		schema, err := graphql.NewSchema(graphql.SchemaConfig{
			Query: graphql.NewObject(graphql.ObjectConfig{
				Name: `Query`,
				Fields: graphql.Fields{
					"q": &graphql.Field{
						Type: graphql.NewNonNull(graphql.String),
						Resolve: func(p graphql.ResolveParams) (interface{}, error) {
							return "hi", nil
						},
					},
				},
			}),
			Subscription: graphql.NewObject(graphql.ObjectConfig{
				Name: `Subscription`,
				Fields: graphql.Fields{
					"s": &graphql.Field{
						Type: graphql.NewNonNull(graphql.String),
						Resolve: func(p graphql.ResolveParams) (interface{}, error) {
							return p.Source, nil
						},
						Subscribe: func(p graphql.ResolveParams) (interface{}, error) {
							c := make(chan interface{})
							stop := gqlws.GetSubscriptionStopSig(p.Context)
							go func() {
								ticker := time.NewTicker(time.Millisecond)
								for {
									select {
									case <-p.Context.Done():
										close(c)
										return
									case <-stop:
										close(c)
										return
									case <-ticker.C:
										c <- `hi`
									}
								}
							}()
							return c, nil
						},
					},
				},
			}),
		})
		assert.Nil(t, err)
		sock := gqlws.Socket{Response: c.Writer, Request: c.Request, Schema: &schema, ConnectionInitTimeout: ConnectionInitTimeout}
		sock.Listen()
	})
	server := httptest.NewServer(eng)
	defer server.Close()
	uri, err := url.Parse(server.URL)
	assert.Nil(t, err)
	uri.Scheme = `ws`
	getClient := func() *websocket.Conn {
		conn, _, err := websocket.DefaultDialer.Dial(uri.String(), http.Header{"Sec-WebSocket-Protocol": []string{`graphql-transport-ws`}})
		assert.Nil(t, err)
		return conn
	}
	closeClient := func(conn *websocket.Conn) {
		conn.WriteControl(websocket.CloseMessage, []byte(``), time.Now().Add(time.Second))
		conn.Close()
	}
	getMessage := func(conn *websocket.Conn) *message.Message {
		var msg message.Message
		assert.Nil(t, conn.ReadJSON(&msg))
		return &msg
	}
	initClient := func(conn *websocket.Conn) {
		assert.Nil(t, conn.WriteJSON(&message.Message{Type: message.ConnectionInit}))
		msg := getMessage(conn)
		assert.Equal(t, message.ConnectionAck, msg.Type)
	}
	getResult := func(msg *message.Message) *graphql.Result {
		assert.Equal(t, message.Next, msg.Type)
		payload, ok := msg.Payload.(map[string]interface{})
		assert.True(t, ok)
		var p graphql.Result
		goutils.UnmarshalJSONFromMap(payload, &p)
		return &p
	}
	t.Run("ConnectionInit", func(t *testing.T) {
		t.Run("can init", func(t *testing.T) {
			client := getClient()
			defer client.Close()
			initClient(client)
		})
		t.Run("closes after timeout", func(t *testing.T) {
			client := getClient()
			defer closeClient(client)
			time.Sleep(ConnectionInitTimeout * 2)
			_, _, err := client.ReadMessage()
			assert.NotNil(t, err)
			assert.IsType(t, new(websocket.CloseError), err)
			e := err.(*websocket.CloseError)
			assert.Equal(t, 4408, e.Code)
		})
	})
	t.Run("can query", func(t *testing.T) {
		client := getClient()
		defer closeClient(client)
		initClient(client)
		id := uuid.NewString()
		assert.Nil(t, client.WriteJSON(&message.Message{Type: message.Subscribe, ID: &id, Payload: &message.SubscribePayload{Query: `query{q}`}}))
		msg := getMessage(client)
		assert.Equal(t, id, *msg.ID)
		result := getResult(msg)
		assert.NotNil(t, result)
		assert.Equal(t, `hi`, result.Data.(map[string]interface{})["q"])
		msg = getMessage(client)
		assert.Equal(t, id, *msg.ID)
		assert.Equal(t, message.Complete, msg.Type)
	})
	t.Run("can subscription", func(t *testing.T) {
		client := getClient()
		defer closeClient(client)
		initClient(client)
		id := uuid.NewString()
		assert.Nil(t, client.WriteJSON(&message.Message{Type: message.Subscribe, ID: &id, Payload: &message.SubscribePayload{Query: `subscription{s}`}}))
		attempts := 10
		for i := 0; i < attempts; i++ {
			msg := getMessage(client)
			assert.Equal(t, id, *msg.ID)
			result := getResult(msg)
			assert.NotNil(t, result)
			assert.Equal(t, `hi`, result.Data.(map[string]interface{})["s"])
		}
	})
}
