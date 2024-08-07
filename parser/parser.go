package parser

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

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
	Type        PacketType
	Nsp         string
	Data        interface{}
	Id          *int
	Attachments int
}

func isBinary(obj interface{}) bool {
	switch obj.(type) {
	case []byte, string:
		return false
	case []*bytes.Buffer, []interface {
		Bytes() []byte
		String() string
	}:
		return true
	default:
		return false
	}
}

func hasBinary(obj interface{}) bool {
	switch v := obj.(type) {
	case []interface{}:
		for _, value := range v {
			if hasBinary(value) {
				return true
			}
		}
		return false
	case map[string]interface{}:
		for _, value := range v {
			if hasBinary(value) {
				return true
			}
		}
		return false
	default:
		return isBinary(obj)
	}
}

func deconstructPacket(packet Packet) (Packet, [][]byte) {
	var buffers [][]byte
	packet.Data = _deconstructPacket(packet.Data, &buffers)
	packet.Attachments = len(buffers)
	return packet, buffers
}

func _deconstructPacket(data interface{}, buffers *[][]byte) interface{} {
	switch v := data.(type) {
	case []interface{}:
		newData := make([]interface{}, len(v))
		for i, value := range v {
			newData[i] = _deconstructPacket(value, buffers)
		}
		return newData
	case map[string]interface{}:
		newData := make(map[string]interface{})
		for key, value := range v {
			newData[key] = _deconstructPacket(value, buffers)
		}
		return newData
	case []byte:
		*buffers = append(*buffers, v)
		return map[string]interface{}{"_placeholder": true, "num": len(*buffers) - 1}
	default:
		return data
	}
}

func reconstructPacket(packet Packet, buffers [][]byte) Packet {
	packet.Data = _reconstructPacket(packet.Data, buffers)
	packet.Attachments = 0
	return packet
}

func _reconstructPacket(data interface{}, buffers [][]byte) interface{} {
	switch v := data.(type) {
	case []interface{}:
		for i, value := range v {
			v[i] = _reconstructPacket(value, buffers)
		}
	case map[string]interface{}:
		if placeholder, ok := v["_placeholder"].(bool); ok && placeholder {
			if num, ok := v["num"].(int); ok {
				return buffers[num]
			}
		}
		for key, value := range v {
			v[key] = _reconstructPacket(value, buffers)
		}
	}
	return data
}

func encodeAsString(packet Packet) string {
	var builder strings.Builder

	builder.WriteString(fmt.Sprintf("%d", packet.Type))

	if packet.Type == BINARY_EVENT || packet.Type == BINARY_ACK {
		builder.WriteString(fmt.Sprintf("%d-", packet.Attachments))
	}

	if packet.Nsp != "/" {
		builder.WriteString(fmt.Sprintf("%s,", packet.Nsp))
	}

	if packet.Id != nil {
		builder.WriteString(fmt.Sprintf("%d", *packet.Id))
	}

	if packet.Data != nil {
		dataStr, _ := json.Marshal(packet.Data)
		builder.WriteString(string(dataStr))
	}

	return builder.String()
}

func encodeAsBinary(packet Packet) [][]byte {
	packet, buffers := deconstructPacket(packet)
	encoded := encodeAsString(packet)
	buffers = append([][]byte{[]byte(encoded)}, buffers...)
	return buffers
}

func Encode(obj Packet) [][]byte {

	if obj.Type == EVENT || obj.Type == ACK {
		if hasBinary(obj) {
			obj.Type = map[bool]PacketType{true: BINARY_EVENT, false: BINARY_ACK}[obj.Type == EVENT]
			return encodeAsBinary(obj)
		}
	}
	return [][]byte{[]byte(encodeAsString(obj))}
}

type Decoder struct {
	reconstructor *BinaryReconstructor
	emit          func(packet *Packet)
}

type BinaryReconstructor struct {
	packet  Packet
	buffers [][]byte
}

func NewBinaryReconstructor(packet Packet) *BinaryReconstructor {
	return &BinaryReconstructor{packet: packet}
}

func (d *Decoder) Add(data interface{}) error {
	switch data := data.(type) {
	case string:
		packet, err := DecodeString(data)
		if err != nil {
			return err
		}
		if packet.Type == BINARY_EVENT || packet.Type == BINARY_ACK {
			d.reconstructor = &BinaryReconstructor{packet: packet}
			if packet.Attachments == 0 {
				d.emit(&packet)
			}
		} else {
			d.emit(&packet)
		}
	case []byte:
		if d.reconstructor == nil {
			return errors.New("got binary data when not reconstructing a packet")
		}
		packet := d.reconstructor.takeBinaryData(data)
		if packet != nil {
			d.reconstructor = nil
			d.emit(packet)
		}
	default:
		return errors.New("Unknown type")
	}

	return nil

}

func DecodeString(str string) (Packet, error) {
	packet := Packet{Type: PacketType(str[0] - '0')}
	func() {
		i := 1
		if packet.Type == BINARY_EVENT || packet.Type == BINARY_ACK {
			j := i
			for j < len(str) && str[j] != '-' {
				j++
			}
			packet.Attachments = int(str[i:j][0] - '0')
			i = j + 1
		}

		if i >= len(str) {
			return
		}

		if str[i] == '/' {
			j := i + 1
			for j < len(str) && str[j] != ',' {
				j++
			}
			packet.Nsp = str[i:j]
			i = j + 1
		}

		if i >= len(str) {
			return
		}

		if str[i] >= '0' && str[i] <= '9' {
			j := i
			for j < len(str) && str[j] >= '0' && str[j] <= '9' {
				j++
			}
			num := int(str[i:j][0] - '0')
			packet.Id = &num
			i = j
		}

		if i >= len(str) {
			return
		}

		if i < len(str) {
			data := str[i:]
			packet.Data = tryParse(data)
		}

	}()

	return packet, nil
}

func tryParse(data string) interface{} {
	var result interface{}
	if err := json.Unmarshal([]byte(data), &result); err != nil {
		return nil
	}
	return result
}

func (br *BinaryReconstructor) takeBinaryData(binData []byte) *Packet {
	br.buffers = append(br.buffers, binData)
	if len(br.buffers) == br.packet.Attachments {
		packet := reconstructPacket(br.packet, br.buffers)
		br.finishedReconstruction()
		return &packet
	}
	return nil
}

func (br *BinaryReconstructor) finishedReconstruction() {
	br.buffers = nil
}
