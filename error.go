package gqlws

import (
	"github.com/gorilla/websocket"
	"github.com/graphql-go/graphql/gqlerrors"
	"github.com/onichandame/gql-ws/message"
)

type HandleError struct {
	ID      string
	message string
}

func NewHandleError(id string, message string) *HandleError {
	var err HandleError
	err.ID = id
	err.message = message
	return &err
}

func (e *HandleError) Error() string {
	return e.message
}

func (e *HandleError) getMessage() *message.Message {
	return (&message.Message{Type: message.Error, Payload: gqlerrors.FormatErrors(e), ID: &e.ID})
}

type FatalError struct {
	code    int
	message string
}

func NewFatalError(code int, msg string) *FatalError {
	var err FatalError
	err.code = code
	err.message = msg
	return &err
}

func (e *FatalError) Error() string {
	return string(websocket.FormatCloseMessage(e.code, e.message))
}
