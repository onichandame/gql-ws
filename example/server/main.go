package main

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/graphql-go/graphql"
	goutils "github.com/onichandame/go-utils"
	gqlwsserver "github.com/onichandame/gql-ws/server"
)

func main() {
	server := gin.Default()
	server.GET("/graphql", func(c *gin.Context) {
		schema, err := graphql.NewSchema(graphql.SchemaConfig{
			Query: graphql.NewObject(graphql.ObjectConfig{
				Name: `Query`,
				Fields: graphql.Fields{
					"echo": &graphql.Field{
						Type: graphql.NewNonNull(graphql.String),
						Args: graphql.FieldConfigArgument{
							"input": &graphql.ArgumentConfig{
								Type: graphql.NewNonNull(graphql.String),
							},
						},
						Resolve: func(p graphql.ResolveParams) (interface{}, error) {
							return p.Args["input"], nil
						},
					},
				},
			}),
			Subscription: graphql.NewObject(graphql.ObjectConfig{
				Name: `Subscription`,
				Fields: graphql.Fields{
					"timestamp": &graphql.Field{
						Type: graphql.NewNonNull(graphql.DateTime),
						Resolve: func(p graphql.ResolveParams) (interface{}, error) {
							return p.Source, nil
						},
						Subscribe: func(p graphql.ResolveParams) (interface{}, error) {
							reschan := make(chan interface{})
							stopchan := gqlwsserver.GetSubscriptionStopSig(p.Context)
							go func() {
								ticker := time.NewTicker(time.Second)
								for {
									select {
									case <-ticker.C:
										reschan <- time.Now()
									case <-p.Context.Done():
										defer close(reschan)
										return
									case <-stopchan:
										defer close(reschan)
										return
									}
								}
							}()
							return reschan, nil
						},
					},
				},
			}),
		})
		goutils.Assert(err)
		sock := gqlwsserver.NewSocket(&gqlwsserver.Config{
			Response: c.Writer, Request: c.Request, Schema: &schema,
		})
		sock.Wait()
		fmt.Println(sock.Error())
	})
	server.StaticFS("home", getFS())
	server.Run(`0.0.0.0:80`)
}
