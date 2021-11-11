package clog

import "context"

// Client defines the external client that can be used in other libraries
type Client struct {
	loggerActions
}

func LogClient() *Client {
	return &Client{loggerActions: logger}
}

func (c *Client) WithField(ctx context.Context, key string, value interface{}) context.Context {
	return WithField(ctx, key, value)
}
