package gqlws

import (
	"context"

	"github.com/graphql-go/graphql/language/ast"
	"github.com/graphql-go/graphql/language/parser"
	"github.com/graphql-go/graphql/language/source"
)

var connParamsKey = &struct{}{}

func GetConnectionParams(ctx context.Context) ConnectionParams {
	return ctx.Value(connParamsKey).(ConnectionParams)
}

var subscriptionStopKey = &struct{}{}

func GetSubscriptionStopSig(ctx context.Context) chan interface{} {
	return ctx.Value(subscriptionStopKey).(chan interface{})
}

func getOperationTypeOfReq(reqStr string) string {
	source := source.NewSource(&source.Source{
		Body: []byte(reqStr),
		Name: "GraphQL request",
	})

	AST, err := parser.Parse(parser.ParseParams{Source: source})
	if err != nil {
		return ""
	}

	for _, node := range AST.Definitions {
		if operationDef, ok := node.(*ast.OperationDefinition); ok {
			return operationDef.Operation
		}
	}
	return ""
}
