package socketio

import (
	"reflect"
	"strconv"
	"strings"
	"sync"

	"github.com/buger/jsonparser"
	"github.com/mark0725/go-socket.io-client/protocol"
	"github.com/mark0725/go-socket.io-client/utils"
)

const (
	OnMessage       = "message"
	OnConnection    = "connection"
	OnDisconnection = "disconnection"
	OnError         = "error"
)

/*
*
System handler function for internal event processing
*/
type systemHandler func(c *Channel)

/*
*
Contains maps of message processing functions
*/
type methods struct {
	messageHandlers     sync.Map
	messageHandlersLock sync.RWMutex

	onConnection    systemHandler
	onDisconnection systemHandler
}

func (m *methods) On(method string, f interface{}) error {
	c, err := newCaller(f)
	if err != nil {
		return err
	}

	m.messageHandlers.Store(method, c)
	return nil
}

/*
*
Find message processing function associated with given method
*/
func (m *methods) findMethod(method string) (*caller, bool) {
	utils.Debug("[handler]find method", method)
	if f, ok := m.messageHandlers.Load(method); ok {
		utils.Debug("[handler]method", method)
		return f.(*caller), true
	}

	return nil, false
}

func (m *methods) callLoopEvent(c *Channel, event string, args ...interface{}) {
	if m.onConnection != nil && event == OnConnection {
		m.onConnection(c)
	}
	if m.onDisconnection != nil && event == OnDisconnection {
		m.onDisconnection(c)
	}

	f, ok := m.findMethod(event)
	if !ok {
		return
	}

	f.callFunc(c, 0, args...)
}

func (m *methods) getEventArgs(msg string) (string, []interface{}, error) {
	rawArr := make([]interface{}, 0, 1)
	event := ""
	c := 0

	_, err := jsonparser.ArrayEach([]byte(msg), func(value []byte, dataType jsonparser.ValueType, offset int, err error) {
		c++
		if c == 1 {
			event = string(value)
		}

		if dataType == jsonparser.String {
			// string must add "\"\"" ps: "message"
			marshal, err := utils.Json.Marshal("")
			if err != nil {
				return
			}
			v := make([]byte, 0, len(value)+2)
			v = append(v, marshal[0])
			v = append(v, value...)
			v = append(v, marshal[1])

			value = v
		}

		rawArr = append(rawArr, value)
	})

	if err != nil {
		return "", nil, err
	}

	return event, rawArr, nil
}
func (m *methods) processIncomingMessageText(c *Channel, msg string) {
	mType, err := strconv.Atoi(string(msg[0]))
	if err != nil {
		return
	}

	switch mType {
	case protocol.CONNECT:
		sid, err := jsonparser.GetString([]byte(msg[1:]), "sid")
		if err != nil {
			return
		}

		c.header.Sid = sid
		m.callLoopEvent(c, OnConnection)
	case protocol.DISCONNECT:
		closeChannel(c, m)
	case protocol.EVENT:
		utils.Debug("[handler]event msg:", msg)
		i1 := strings.Index(msg, "[")
		i2 := strings.Index(msg, "{")
		di := i1
		if i2 > -1 && (i2 < i1 || i1 == -1) {
			di = i2
		}

		i3 := strings.Index(msg, ",")
		nsp := protocol.DefaultNsp
		acki := 1
		if i3 > -1 && i3 < di {
			nsp = msg[1:i3]
			acki = i3 + 1
		}
		id := ""
		ackId := -1
		if acki < di {
			id = msg[acki:di]
			ackId, _ = strconv.Atoi(id)
			utils.Debug("[handler]event ackid:", ackId)
		}

		utils.Debug("[handler]event nsp:", nsp)
		utils.Debug("[handler]event msg:", msg[di:])
		event, args, err := m.getEventArgs(msg[di:])
		if err != nil {
			return
		}

		f, ok := m.findMethod(event)
		if !ok {
			return
		}

		utils.Debug("[handler]event args: ", args)
		if len(args) == 1 {
			f.callFunc(c, 1)
			return
		}
		ackRes := f.callFunc(c, 1, args[1:]...)
		// ack
		//if string(msg[1]) != "[" {
		if id != "" {
			// ackId, offset, err := parseAckId(msg[1:])
			// if err != nil || ackId < 0 {
			// 	return
			// }

			arr := make([]interface{}, 0, 1)
			for _, v := range ackRes {
				arr = append(arr, v.Interface())
			}

			r := &protocol.Message{
				Type:  protocol.ACK,
				Nsp:   nsp,
				AckId: ackId,
				Args:  arr,
			}

			c.out <- protocol.GetMsgPacket(r)
		}
	case protocol.ACK:
		ackId, offset, err := parseAckId(msg[1:])
		if err != nil || ackId < 0 {
			return
		}

		if waiter, err := c.ack.getWaiter(ackId); err == nil {
			_, args, err := m.getEventArgs(msg[1+offset:])
			if err != nil {
				return
			}
			waiter <- args
		}
	case protocol.CONNECT_ERROR:
		closeChannel(c, m)
	case protocol.BINARY_EVENT:
	case protocol.BINARY_ACK:
	}
}

func (m *methods) processIncomingMessage(c *Channel, msg string) {
	if !c.conn.GetUseBinaryMessage() {
		go m.processIncomingMessageText(c, msg)
		return
	}

	packet := &protocol.MsgPack{}
	err := utils.Json.UnmarshalFromString(msg, &packet)
	if err != nil {
		return
	}

	switch packet.Type {
	case protocol.CONNECT:
		// server protocol 4 & binary msg -> client protocol 3 // 4{"type":0,"data":null,"nsp":"/","id":0}
		if packet.Data == nil {
			return
		}
		// {"type":0,"data":{},"nsp":"/","id":0}
		if reflect.ValueOf(packet.Data).Len() == 0 {
			return
		}
		if packet.Id < 0 {
			return
		}

		c.header.Sid = reflect.ValueOf(packet.Data).MapIndex(reflect.ValueOf("sid")).Interface().(string)
		m.callLoopEvent(c, OnConnection)
	case protocol.DISCONNECT:
		closeChannel(c, m)
	case protocol.EVENT:
		data := packet.Data.([]interface{})
		if len(data) == 0 {
			return
		}
		event := data[0].(string)

		f, ok := m.findMethod(event)
		if !ok {
			return
		}
		if len(data) == 1 {
			f.callFunc(c, 0)
			return
		}

		ackRes := f.callFunc(c, 0, data[1:]...)
		// ack
		if packet.Id >= 0 {

			arr := make([]interface{}, 0, 1)

			for _, v := range ackRes {
				arr = append(arr, v.Interface())
			}

			r := &protocol.Message{
				Type:  protocol.ACK,
				Nsp:   packet.Nsp,
				AckId: packet.Id,
				Args:  arr,
			}

			c.out <- protocol.GetMsgPacket(r)
		}
	case protocol.ACK:
		if waiter, err := c.ack.getWaiter(packet.Id); err == nil {
			waiter <- packet.Data
		}
	case protocol.CONNECT_ERROR:
		closeChannel(c, m)
	case protocol.BINARY_EVENT:
	case protocol.BINARY_ACK:
	}
}

func parseAckId(msg string) (int, int, error) {
	var offset = 0
	var id = ""
	for msg[offset] >= 48 && msg[offset] <= 57 {
		id = id + string(msg[offset])
		offset++
	}
	ret, err := strconv.Atoi(id)
	return ret, offset, err
}
