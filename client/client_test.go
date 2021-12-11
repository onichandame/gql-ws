package gqlwsclient_test

import (
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/graphql-go/graphql"
	"github.com/graphql-go/graphql/gqlerrors"
	gqlwsclient "github.com/onichandame/gql-ws/client"
	gqlwsmessage "github.com/onichandame/gql-ws/message"
	gqlwsserver "github.com/onichandame/gql-ws/server"
	"github.com/stretchr/testify/assert"
)

func TestClient(t *testing.T) {
	ackTimeout := time.Millisecond * 500
	closePeriod := time.Millisecond * 500
	eng := gin.Default()
	eng.GET("", func(c *gin.Context) {
		schema, err := graphql.NewSchema(graphql.SchemaConfig{
			Query: graphql.NewObject(graphql.ObjectConfig{
				Name: `Query`,
				Fields: graphql.Fields{
					"q": &graphql.Field{
						Type: graphql.NewNonNull(graphql.String),
						Resolve: func(p graphql.ResolveParams) (interface{}, error) {
							return `hi`, nil
						},
					},
				},
			}),
			Subscription: graphql.NewObject(graphql.ObjectConfig{
				Name: `Sub`,
				Fields: graphql.Fields{
					"s": &graphql.Field{
						Type: graphql.NewNonNull(graphql.String),
						Resolve: func(p graphql.ResolveParams) (interface{}, error) {
							return p.Source, nil
						},
						Subscribe: func(p graphql.ResolveParams) (interface{}, error) {
							stop := gqlwsserver.GetSubscriptionStopSig(p.Context)
							ticker := time.NewTicker(time.Millisecond)
							res := make(chan interface{})
							go func() {
								for {
									select {
									case <-stop:
										close(res)
									case <-p.Context.Done():
										close(res)
									case <-ticker.C:
										res <- `hi`
									}
								}
							}()
							return res, nil
						},
					},
				},
			}),
		})
		assert.Nil(t, err)
		sock := gqlwsserver.NewSocket(&gqlwsserver.Config{
			Response: c.Writer,
			Request:  c.Request,
			Schema:   &schema,
		})
		sock.Wait()
	})
	srv := httptest.NewServer(eng)
	u, err := url.Parse(srv.URL)
	assert.Nil(t, err)
	u.Scheme = `ws`
	getClient := func() *gqlwsclient.Client {
		return gqlwsclient.NewClient(&gqlwsclient.Config{
			URL:                  u.String(),
			ConnectionAckTimeout: ackTimeout,
			GraceClosePeriod:     closePeriod,
		})
	}
	t.Run("connection init", func(t *testing.T) {
		client := getClient()
		defer client.Close()
		time.Sleep(ackTimeout * 2)
		assert.Nil(t, client.Error())
	})
	t.Run("query", func(t *testing.T) {
		client := getClient()
		defer client.Close()
		res := make(chan string)
		client.Subscribe(gqlwsmessage.SubscribePayload{Query: `query{q}`}, gqlwsclient.Handlers{OnNext: func(r *graphql.Result) { res <- r.Data.(map[string]interface{})[`q`].(string) }, OnError: func(fe gqlerrors.FormattedErrors) { close(res) }})
		v, ok := <-res
		assert.True(t, ok)
		assert.Equal(t, `hi`, v)
	})
	t.Run(`subscription`, func(t *testing.T) {
		client := getClient()
		defer client.Close()
		res := make(chan string)
		client.Subscribe(gqlwsmessage.SubscribePayload{Query: `subscription{s}`}, gqlwsclient.Handlers{OnNext: func(r *graphql.Result) { res <- r.Data.(map[string]interface{})[`s`].(string) }, OnError: func(fe gqlerrors.FormattedErrors) { close(res) }})
		for i := 0; i < 10; i++ {
			v, ok := <-res
			assert.True(t, ok)
			assert.Equal(t, `hi`, v)
		}
	})
}
