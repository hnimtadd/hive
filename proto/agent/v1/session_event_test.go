package agentv1

import "testing"

func TestSessionEventToHiveSessionResponse(t *testing.T) {
	t.Parallel()

	event := NewSessionEventTurnResponse("req-1", &TurnResponse{
		RequestId: "req-1",
		Payload: &TurnResponse_Update{
			Update: &TurnUpdate{
				Content: "hello",
			},
		},
	})

	resp, err := event.ToHiveSessionResponse()
	if err != nil {
		t.Fatalf("ToHiveSessionResponse() error = %v", err)
	}
	if resp.GetInReplyTo() != "req-1" {
		t.Fatalf("expected in_reply_to req-1, got %s", resp.GetInReplyTo())
	}
	if resp.GetTurnResponse() == nil {
		t.Fatal("expected turn_response payload")
	}
	if got := resp.GetTurnResponse().GetUpdate().GetContent(); got != "hello" {
		t.Fatalf("expected update content hello, got %s", got)
	}
}
