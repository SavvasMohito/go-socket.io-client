package engineio

import "sync"

type client struct {
	// conn      transport.Conn
	// params    transport.ConnParameters
	transport string
	context   interface{}
	close     chan struct{}
	closeOnce sync.Once
}

func (c *client) serve() {

}
