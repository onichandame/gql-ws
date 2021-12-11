package gqlwserror

import (
	"github.com/gorilla/websocket"
	"github.com/graphql-go/graphql/gqlerrors"
	gqlwsmessage "github.com/onichandame/gql-ws/message"
)

type HandlableError struct {
	ID           string
	gqlwsmessage string
}

func NewHandlableError(id string, gqlwsmessage string) *HandlableError {
	var err HandlableError
	err.ID = id
	err.gqlwsmessage = gqlwsmessage
	return &err
}

func (e *HandlableError) Error() string {
	return e.gqlwsmessage
}

func (e *HandlableError) GetMessage() *gqlwsmessage.Message {
	return (&gqlwsmessage.Message{Type: gqlwsmessage.Error, Payload: gqlerrors.FormatErrors(e), ID: &e.ID})
}

type FatalError struct {
	code         int
	gqlwsmessage string
}

func NewFatalError(code int, msg string) *FatalError {
	var err FatalError
	err.code = code
	err.gqlwsmessage = msg
	return &err
}

func (e *FatalError) Error() string {
	return string(websocket.FormatCloseMessage(e.code, e.gqlwsmessage))
}
