package websocket

import (
	"encoding/json"
	"hse-dss-efimov/network"
)

type MessageKind int

const (
	MK_Hello     MessageKind = 1
	MK_Request   MessageKind = 2
	MK_Response  MessageKind = 3
	MK_Broadcast MessageKind = 4
)

type Message struct {
	Kind      MessageKind `json:"kind"`
	MsgNumber string      `json:"msgNumber,omitempty"`
	Data      string `json:"data,omitempty"`
	Request   string      `json:"request,omitempty"`
}

type wsMessage struct {
	Src string `json:"src"`
	Dst string `json:"dst"`
	MsgNumber string `json:"msgNumber"`
	Payload string `json:"payload"`
}

type Chans_ports struct {
	MsgsDb MsgDb
	MsgChan chan network.Message
}

type CallCtx interface {
	Data() json.RawMessage
	Reply(response interface{})
}

type CallHandler func(ctx CallCtx)
