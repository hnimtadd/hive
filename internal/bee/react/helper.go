package react

// // EventStreamingMiddleware creates a middleware that sends tool events to a channel
// func EventStreamingMiddleware(eventCh chan<- *ToolExecutionEvent) ToolExecutionMiddleware {
// 	return func(ctx context.Context, event *ToolExecutionEvent) error {
// 		select {
// 		case eventCh <- event:
// 			return nil
// 		case <-ctx.Done():
// 			return ctx.Err()
// 		default:
// 			// Channel full, log warning but don't block
// 			trace.Logger(ctx).Warn("tool event channel full, dropping event",
// 				slog.String("agent_id", event.AgentID),
// 				slog.String("tool", event.ToolName),
// 				slog.String("event_type", string(event.EventType)),
// 			)
// 			return nil
// 		}
// 	}
// }

// LoggingMiddleware creates a middleware that logs tool events with structured logging.
// func LoggingMiddleware(logger *slog.Logger) ToolExecutionMiddleware {
// 	return func(ctx context.Context, event *ToolExecutionEvent) error {
// 		attrs := []any{
// 			slog.String("agent_id", event.AgentID),
// 			slog.String("tool", event.ToolName),
// 			slog.String("call_id", event.CallID),
// 			slog.String("event_type", string(event.EventType)),
// 		}
//
// 		switch event.EventType {
// 		case ToolEventStarted:
// 			attrs = append(attrs, slog.Int("input_length", len(event.Input)))
// 			logger.InfoContext(ctx, "tool execution started", attrs...)
//
// 		case ToolEventCompleted:
// 			attrs = append(attrs, slog.Int("output_length", len(event.Output)))
// 			logger.InfoContext(ctx, "tool execution completed", attrs...)
//
// 		case ToolEventFailed:
// 			attrs = append(attrs, slog.Any("error", event.Error))
// 			logger.ErrorContext(ctx, "tool execution failed", attrs...)
// 		}
//
// 		return nil
// 	}
// }
