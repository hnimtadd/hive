package hive

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"
)

// ToolClient communicates with a tool process via stdin/stdout.
type ToolClient struct {
	reader   io.ReadCloser
	writer   io.WriteCloser
	debugger io.ReadCloser
	timeout  time.Duration
}

// NewToolClient creates a client for tool communication.
func NewToolClient(reader io.ReadCloser, writer io.WriteCloser, debugger io.ReadCloser) *ToolClient {
	return &ToolClient{
		reader:   reader,
		writer:   writer,
		debugger: debugger,
		timeout:  30 * time.Second,
	}
}

// WithTimeout sets a custom timeout.
func (c *ToolClient) WithTimeout(timeout time.Duration) *ToolClient {
	c.timeout = timeout
	return c
}

// Invoke sends an invoke request with raw JSON payload.
func (c *ToolClient) Invoke(ctx context.Context, payload json.RawMessage) (*Response, error) {
	req := &Request{Action: "invoke", Payload: payload}
	return c.send(ctx, req)
}

// Inspect retrieves tool metadata.
func (c *ToolClient) Inspect(ctx context.Context) (*Response, error) {
	req := &Request{Action: "inspect"}
	return c.send(ctx, req)
}

// DebugLog tries best to read the logs from the debugger which is the executed
// process stderr
func (c *ToolClient) DebugLog() string {
	debugBytes, err := io.ReadAll(c.debugger)
	if err != nil {
		return fmt.Sprintf("failed to read debug logs; %s", err)
	}
	return string(debugBytes)
}

func (c *ToolClient) send(ctx context.Context, req *Request) (*Response, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	// Send request
	if err := json.NewEncoder(c.writer).Encode(req); err != nil {
		return nil, err
	}

	if err := c.writer.Close(); err != nil {
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
