package parser

const Protocol = 5

type PacketType int

const (
	CONNECT PacketType = iota
	DISCONNECT
	EVENT
	ACK
	CONNECT_ERROR
	BINARY_EVENT
	BINARY_ACK
)

type Packet struct {
	Type        PacketType  `json:"type"`
	Nsp         string      `json:"nsp,omitempty"`
	Data        interface{} `json:"data,omitempty"`
	NeedAck     bool
	Id          int `json:"id,omitempty"`
	Attachments int
}

// Header of packet.
type Header struct {
	Type      PacketType
	ID        uint64
	NeedAck   bool
	Namespace string
	Query     string
}

// Payload of packet.
type Payload struct {
	Header Header

	Data []interface{}
}
