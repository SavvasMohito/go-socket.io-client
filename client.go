package socketio

import (
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/mark0725/go-socket.io-client/protocol"
	"github.com/mark0725/go-socket.io-client/utils"
	"github.com/mark0725/go-socket.io-client/websocket"
)

// const (
// 	webSocketProtocol       = "ws://"
// 	webSocketSecureProtocol = "wss://"
// 	socketIOUrl             = "/socket.io/?transport=websocket"
// )

// namespace
const (
	aliasRootNamespace = "/"
	rootNamespace      = ""
)

type ClientOptions struct {
	Namespace string
	Path      string
	Auth      map[string]string
	//IOOpts    *engineio.Options
}

type Client struct {
	namespace string
	url       string
	path      string
	auth      map[string]string

	//tr websocket.Transport
	//handlers *namespaceHandlers
	handlers methods
	channel  Channel
}

func NewClient(addr string, opts *ClientOptions) (*Client, error) {
	c := &Client{}

	var err error
	if addr == "" {
		return nil, errors.New("EmptyAddrErr")
	}

	u, err := url.Parse(addr)
	if err != nil {
		return nil, err
	}

	switch u.Scheme {
	case "http":
		u.Scheme = "ws"
	case "https":
		u.Scheme = "wss"
	}

	namespace := fmtNS(u.Path)
	if opts.Namespace != "" {
		namespace = fmtNS(opts.Namespace)
	}
	c.namespace = namespace
	c.channel.namespace = namespace

	u.Path = "/socket.io"
	if opts.Path != "" {
		u.Path = opts.Path
	}

	u.Path = u.EscapedPath()
	if !strings.HasSuffix(u.Path, "/") {
		u.Path += "/"
	}
	c.path = u.Path

	c.url = u.String()
	utils.Debug("url:", c.url)

	if opts.Auth != nil {
		c.auth = opts.Auth
	}

	return c, nil
}

func (c *Client) Connect() error {
	var err error
	tr := websocket.GetDefaultWebsocketTransport()

	u, err := url.Parse(c.url)
	if err != nil {
		return err
	}

	queryParams := u.Query()
	queryParams.Set("transport", "websocket")

	if tr.Protocol == protocol.Protocol3 {
		queryParams.Set("EIO", "3")
	} else if tr.Protocol == protocol.Protocol4 {
		queryParams.Set("EIO", "4")
	} else {
		queryParams.Set("EIO", "4")
	}

	queryParams.Set("t", time.Now().Format("02150405"))

	u.RawQuery = queryParams.Encode()

	eioAddr := u.String()
	utils.Debug("[sockio-client] full addr: ", eioAddr)

	c.channel.initChannel()
	c.channel.conn, err = tr.Connect(eioAddr)
	if err != nil {
		return err
	}

	go c.clientRead()
	go c.clientWrite()

	return nil
}

func (c *Client) Close() {
	closeChannel(&c.channel, &c.handlers)
}

func (c *Client) On(method string, f interface{}) error {
	return c.handlers.On(method, f)
}

func (c *Client) Emit(method string, args ...interface{}) error {
	return c.channel.Emit(method, args...)
}

func (c *Client) clientRead() error {
	for {
		msg, err := c.channel.conn.GetMessage()
		if err != nil {
			return closeChannel(&c.channel, &c.handlers, err)
		}

		prefix := string(msg[0])
		protocolV := c.channel.conn.GetProtocol()

		switch prefix {
		case protocol.OpenMsg:
			if err := utils.Json.UnmarshalFromString(msg[1:], &c.channel.header); err != nil {
				closeErr := &websocket.CloseError{}
				closeErr.Code = websocket.ParseOpenMsgCode
				closeErr.Text = err.Error()

				return closeChannel(&c.channel, &c.handlers, closeErr)
			}

			if protocolV == protocol.Protocol3 {
				c.handlers.callLoopEvent(&c.channel, OnConnection)
				// in protocol v3, the client sends a ping, and the server answers with a pong
				go SchedulePing(&c.channel)
			}

			if c.channel.conn.GetProtocol() == protocol.Protocol4 {

				// in protocol v4 & binary msg Connection to a namespace
				if c.channel.conn.GetUseBinaryMessage() {
					c.channel.out <- &protocol.MsgPack{
						Type: protocol.CONNECT,
						//Nsp:  protocol.DefaultNsp,
						Nsp:  c.namespace,
						Data: c.auth,
					}

					// in protocol v4 & text msg Connection to a namespace
				} else {
					replyMsg := protocol.CommonMsg + protocol.OpenMsg + c.namespace
					if c.auth != nil {
						jsonData, _ := json.Marshal(c.auth)
						dataText := string(jsonData)
						replyMsg = replyMsg + dataText
					}

					c.channel.out <- replyMsg
				}
			}
		case protocol.CloseMsg:
			return closeChannel(&c.channel, &c.handlers)
		case protocol.PingMsg:
			// in protocol v4, the server sends a ping, and the client answers with a pong
			c.channel.out <- protocol.PongMsg
		case protocol.PongMsg:
		case protocol.UpgradeMsg:
		case protocol.CommonMsg:
			// in protocol v3 & binary msg  ps: 4{"type":0,"data":null,"nsp":"/","id":0}
			// in protocol v3 & text msg  ps: 40 or 41 or 42["message", ...]
			// in protocol v4 & text msg  ps: 40 or 41 or 42["message", ...]
			go c.handlers.processIncomingMessage(&c.channel, msg[1:])
		default:
			// in protocol v4 & binary msg ps: {"type":0,"data":{"sid":"HWEr440000:1:R1CHyink:shadiao:101"},"nsp":"/","id":0}
			go c.handlers.processIncomingMessage(&c.channel, msg)
		}
	}
}

func (c *Client) clientWrite() error {
	for {
		outBufferLen := len(c.channel.out)
		if outBufferLen >= queueBufferSize-1 {
			closeErr := &websocket.CloseError{}
			closeErr.Code = websocket.QueueBufferSizeCode
			closeErr.Text = ErrorSocketOverflood.Error()

			closeChannel(&c.channel, &c.handlers, closeErr)
		}

		msg := <-c.channel.out
		if msg == protocol.CloseMsg {
			return nil
		}

		err := c.channel.conn.WriteMessage(msg)
		if err != nil {
			closeErr := &websocket.CloseError{}
			closeErr.Code = websocket.WriteBufferErrCode
			closeErr.Text = err.Error()

			closeChannel(&c.channel, &c.handlers, closeErr)
		}
	}
}

func fmtNS(ns string) string {
	if ns == aliasRootNamespace {
		return rootNamespace
	}

	return ns
}
