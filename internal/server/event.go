package server

import (
	"context"
	"fmt"
	"log/slog"

	agentv1 "github.com/hnimtadd/hive/gen/agent/v1"
	"github.com/hnimtadd/hive/internal/trace"
	"google.golang.org/grpc"
)

// forwardToolEvents forwards tool execution events from the channel to the gRPC stream.
func (s *HiveServer) forwardToolEvents(
	ctx context.Context,
	srv grpc.BidiStreamingServer[agentv1.ExecuteTaskRequest, agentv1.ExecuteTaskResponse],
	eventCh <-chan ExecutionEvent,
	doneCh <-chan struct{},
) {
	logger := trace.Logger(ctx)

	select {
	case event := <-eventCh:
		content := ""
		switch event.typ {
		case EventTypeLLMRequest:
			content = fmt.Sprintf("%s executing llm request: %s", event.req.AgentID, event.req.Input)
		case EventTypeLLMResponse:
			content = fmt.Sprintf("%s return llm response: latency: %d, token_used: %d", event.resp.AgentID, event.resp.ExecutionTimeMs, event.resp.TokenUsed.TotalTokens)
		case EventTypeToolCall:
			if event.tool.Succeed {
				content = fmt.Sprintf("tool call: name: %s, latency: %d, output: %s", event.tool.ToolName, event.tool.ExecutionTimeMs, event.tool.Output)
			} else {
				content = fmt.Sprintf("tool call: name: %s, latency: %d, error: %s", event.tool.ToolName, event.tool.ExecutionTimeMs, event.tool.Error.Error())
			}
		}
		// Convert to protobuf message
		updateMsg := &agentv1.InProgressUpdate{
			Content: content,
			Status:  string(event.typ),
		}

		// Send to client via gRPC stream
		if err := srv.Send(&agentv1.ExecuteTaskResponse{
			Payload: &agentv1.ExecuteTaskResponse_Update{
				Update: updateMsg,
			},
		}); err != nil {
			logger.ErrorContext(ctx, "failed to send execution event",
				slog.Any("error", err),
			)
			return
		}

		logger.DebugContext(ctx, "execution event sent", slog.String("typ", string(event.typ)))
	case <-doneCh:
		return
	}
}
