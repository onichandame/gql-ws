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
