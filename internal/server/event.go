package server

import (
	"context"
	"fmt"
	"log/slog"

	agentv1 "github.com/hnimtadd/hive/gen/agent/v1"
	"github.com/hnimtadd/hive/internal/trace"
	"github.com/hnimtadd/hive/pkg/utils"
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

	for {
		select {
		case event := <-eventCh:
			content := ""
			switch event.typ {
			case EventTypeLLMRequestStart:
				content = fmt.Sprintf("%s executing llm request: %s", event.req.AgentID, event.req.Input)
			case EventTypeLLMRequestFinish:
				content = fmt.Sprintf("%s return llm response: latency: %d, token_used: %d", event.resp.AgentID, event.resp.ExecutionTimeMs, event.resp.TokenUsed.TotalTokens)
			case EventTypeToolCallStart:
				content = fmt.Sprintf("tool call: name: %s, input: %s", event.toolRequest.ToolName, event.toolRequest.Arguments)
			case EventTypeToolCallFinish:
				tr := event.toolResponse
				if tr.Succeed {
					content = fmt.Sprintf("tool call ID: %s, latency: %d, output: %s", tr.CallID, tr.ExecutionTimeMs, tr.Output)
				} else {
					content = fmt.Sprintf("tool call ID: %s, latency: %d, error: %s", tr.CallID, tr.ExecutionTimeMs, tr.Error.Error())
				}
			}
			// Convert to protobuf message
			updateMsg := &agentv1.InProgressUpdate{
				Content: utils.SanitizeUTF8(content),
				Status:  utils.SanitizeUTF8(string(event.typ)),
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
}
