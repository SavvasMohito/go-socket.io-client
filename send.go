package socketio

import (
	"errors"
	"log"
	"time"

	"github.com/mark0725/go-socket.io-client/protocol"
)

var (
	ErrorSendTimeout     = errors.New("timeout")
	ErrorSocketOverflood = errors.New("socket overflood")
)

/*
*
Send message packet to socket
*/
func send(c *Channel, msg *protocol.Message) error {
	defer func() {
		if r := recover(); r != nil {
			log.Println("socket.io send panic: ", r)
		}
	}()

	if !c.IsAlive() {
		return nil
	}

	out := protocol.GetMsgPacket(msg)

	if len(c.out) == queueBufferSize {
		return ErrorSocketOverflood
	}

	c.out <- out

	return nil
}

func (c *Channel) Emit(method string, args ...interface{}) error {
	msg := &protocol.Message{
		Type:   protocol.EVENT,
		AckId:  -1,
		Method: method,
		Nsp:    c.namespace,
		Args:   args,
	}

	return send(c, msg)
}

func (c *Channel) Ack(method string, timeout time.Duration, args ...interface{}) (interface{}, error) {
	msg := &protocol.Message{
		Type:   protocol.EVENT,
		AckId:  c.ack.getNextId(),
		Method: method,
		Nsp:    c.namespace,
		Args:   args,
	}

	waiter := make(chan interface{})
	c.ack.addWaiter(msg.AckId, waiter)

	err := send(c, msg)
	if err != nil {
		c.ack.removeWaiter(msg.AckId)
	}

	select {
	case result := <-waiter:
		c.ack.removeWaiter(msg.AckId)
		return result, nil
	case <-time.After(timeout):
		c.ack.removeWaiter(msg.AckId)
		return nil, ErrorSendTimeout
	}
}
