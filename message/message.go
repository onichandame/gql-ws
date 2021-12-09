package message

type Type string

const (
	ConnectionInit Type = `connection_init`
	ConnectionAck  Type = `connection_ack`
	Ping           Type = `ping`
	Pong           Type = `pong`
	Subscribe      Type = `subscribe`
	Next           Type = `next`
	Error          Type = `error`
	Complete       Type = `complete`
)

type Message struct {
	Type    Type    `json:"type"`
	Payload Payload `json:"payload,omitempty"`
	ID      *string `json:"id,omitempty"`
}

type Payload interface{}

type SubscribePayload struct {
	OperationName string                 `json:"operationName,omitempty"`
	Query         string                 `json:"query"`
	Variables     map[string]interface{} `json:"variables,omitempty"`
	Extensions    map[string]interface{} `json:"extensions,omitempty"`
}
