package main

import (
	"github.com/gin-gonic/gin"
	"github.com/graphql-go/graphql"
	goutils "github.com/onichandame/go-utils"
	gqlws "github.com/onichandame/gql-ws"
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
						Resolve: func(p graphql.ResolveParams) (interface{}, error) {
							return `echo`, nil
						},
					},
				},
			}),
		})
		goutils.Assert(err)
		sock := gqlws.Socket{Response: c.Writer, Request: c.Request, Schema: &schema}
		sock.Listen()
	})
	server.StaticFS("home", getFS())
	server.Run(`0.0.0.0:80`)
}
