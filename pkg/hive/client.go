package hive

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"
)

// ToolClient communicates with a tool process via stdin/stdout.
type ToolClient struct {
	reader  *bufio.Reader
	writer  io.Writer
	timeout time.Duration
}

// NewToolClient creates a client for tool communication.
func NewToolClient(reader io.Reader, writer io.Writer) *ToolClient {
	var r *bufio.Reader
	if br, ok := reader.(*bufio.Reader); ok {
		r = br
	} else {
		r = bufio.NewReader(reader)
	}

	return &ToolClient{
		reader:  r,
		writer:  writer,
		timeout: 30 * time.Second,
	}
}

// WithTimeout sets a custom timeout.
func (c *ToolClient) WithTimeout(timeout time.Duration) *ToolClient {
	c.timeout = timeout
	return c
}

// Invoke sends an invoke request to the tool.
func (c *ToolClient) Invoke(ctx context.Context, input interface{}) (*Response, error) {
	payload, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("marshal error: %w", err)
	}

	req := &Request{Action: "invoke", Payload: payload}
	return c.send(ctx, req)
}

// InvokeRaw sends an invoke request with raw JSON payload.
func (c *ToolClient) InvokeRaw(ctx context.Context, payload json.RawMessage) (*Response, error) {
	req := &Request{Action: "invoke", Payload: payload}
	return c.send(ctx, req)
}

// Inspect retrieves tool metadata.
func (c *ToolClient) Inspect(ctx context.Context) (*Response, error) {
	req := &Request{Action: "inspect"}
	return c.send(ctx, req)
}

func (c *ToolClient) send(ctx context.Context, req *Request) (*Response, error) {
	if deadline, ok := ctx.Deadline(); ok {
		c.timeout = time.Until(deadline)
	}
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	// Send request
	data, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	if _, err := fmt.Fprintf(c.writer, "%s\n", data); err != nil {
		return nil, err
	}

	// Read response
	type result struct {
		resp *Response
		err  error
	}
	ch := make(chan result, 1)

	go func() {
		var resp Response
		if err := json.NewDecoder(c.reader).Decode(&resp); err != nil {
			ch <- result{err: err}
			return
		}
		ch <- result{resp: &resp}
	}()

	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("timeout: %w", ctx.Err())
	case r := <-ch:
		return r.resp, r.err
	}
}

