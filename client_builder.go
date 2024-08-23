package socketio

type ClientBuilder struct{}

type ClientOption func(*ClientOptions)

func (c *ClientBuilder) WithNamespace(v string) ClientOption {
	return func(c *ClientOptions) {
		c.Namespace = v
	}
}

func (c *ClientBuilder) WithPath(v string) ClientOption {
	return func(c *ClientOptions) {
		c.Path = v
	}
}

func (c *ClientBuilder) WithAuth(v map[string]string) ClientOption {
	return func(c *ClientOptions) {
		c.Auth = v
	}
}

func (c *ClientBuilder) WithIOOpts(v map[string]string) ClientOption {
	return func(c *ClientOptions) {
		c.Auth = v
	}
}

func (c *ClientBuilder) Build(addr string, opts ...ClientOption) (*Client, error) {
	// 设置默认值
	clientOptions := &ClientOptions{
		Namespace: "",
		Auth:      nil,
		//IOOpts:    nil,
	}

	// 应用选项函数进行定制化设置
	for _, opt := range opts {
		opt(clientOptions)
	}

	client, err := NewClient(addr, clientOptions)

	return client, err
}
