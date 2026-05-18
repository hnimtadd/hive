package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

// Minimal OpenAI-compatible mock server.
// Implements:
// - POST /v1/chat/completions
// - GET  /v1/models
//
// Designed for local integration testing. It does not validate auth.

type chatCompletionsRequest struct {
	Model    string `json:"model"`
	Messages []struct {
		Role    string `json:"role"`
		Content any    `json:"content"`
	} `json:"messages"`
	Stream bool `json:"stream"`
}

type chatCompletionsResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index        int `json:"index"`
		Message      any `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage map[string]int `json:"usage,omitempty"`
}

type mockDirective struct {
	// If set, respond with this HTTP status code.
	HTTPStatus int `json:"http_status"`
	// If set, sleep before responding.
	DelayMs int `json:"delay_ms"`

	// If set, use this exact assistant content string.
	Content string `json:"content"`
	// If set, JSON-encode this object and use it as assistant content.
	JSON any `json:"json"`

	// Convenience: build a Hive queen-style JSON response.
	Status     string `json:"status"`
	NextAction string `json:"next_action"`

	// Convenience: if true, echo the last user message (excluding directive).
	Echo bool `json:"echo"`
}

var mockDirectiveRe = regexp.MustCompile(`(?s)#mock\s*(\{.*\})`)

func main() {
	var (
		addr         = flag.String("addr", "127.0.0.1:18080", "listen address")
		defaultModel = flag.String("model", "mock", "model name returned in responses")
		mode         = flag.String("mode", "fixed", "response mode: fixed|echo")
		fixedText    = flag.String("fixed", os.Getenv("MOCK_OPENAI_FIXED_RESPONSE"), "fixed assistant text (or env MOCK_OPENAI_FIXED_RESPONSE)")
	)
	flag.Parse()

	if *fixedText == "" {
		*fixedText = "{\"status\":\"completed\",\"content\":\"mock response\"}"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	mux.HandleFunc("/v1/models", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(fmt.Sprintf(`{"object":"list","data":[{"id":%q,"object":"model"}]}`+"\n", *defaultModel)))
	})

	mux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var req chatCompletionsRequest
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		if err := dec.Decode(&req); err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid_request_error", err.Error())
			return
		}
		if req.Stream {
			// Keep it explicit: Hive's current OpenAI client usage is non-stream.
			writeJSONError(w, http.StatusBadRequest, "invalid_request_error", "streaming not supported by mock server")
			return
		}

		modelName := req.Model
		if modelName == "" {
			modelName = *defaultModel
		}

		assistantText := *fixedText
		if strings.EqualFold(*mode, "echo") {
			assistantText = echoFromMessages(req.Messages)
		}
		if d, ok := parseDirectiveFromMessages(req.Messages); ok {
			if d.DelayMs > 0 {
				time.Sleep(time.Duration(d.DelayMs) * time.Millisecond)
			}
			if d.HTTPStatus != 0 && d.HTTPStatus != http.StatusOK {
				writeJSONError(w, d.HTTPStatus, "mock_error", "simulated error")
				return
			}

			switch {
			case d.Echo:
				assistantText = echoFromMessages(req.Messages)
			case d.Content != "":
				assistantText = d.Content
			case d.JSON != nil:
				bs, err := json.Marshal(d.JSON)
				if err != nil {
					writeJSONError(w, http.StatusBadRequest, "invalid_request_error", "mock.json must be JSON-serializable")
					return
				}
				assistantText = string(bs)
			case d.Status != "":
				// Queen output schema: {status, content, next_action?}
				payload := map[string]any{
					"status": d.Status,
					"content": assistantText,
				}
				if d.NextAction != "" {
					payload["next_action"] = d.NextAction
				}
				bs, _ := json.Marshal(payload)
				assistantText = string(bs)
			}
		}

		resp := chatCompletionsResponse{
			ID:      "chatcmpl-mock",
			Object:  "chat.completion",
			Created: time.Now().Unix(),
			Model:   modelName,
			Usage: map[string]int{
				"prompt_tokens":     0,
				"completion_tokens": 0,
				"total_tokens":      0,
			},
		}
		resp.Choices = make([]struct {
			Index        int `json:"index"`
			Message      any `json:"message"`
			FinishReason string `json:"finish_reason"`
		}, 1)
		resp.Choices[0].Index = 0
		resp.Choices[0].FinishReason = "stop"
		resp.Choices[0].Message = map[string]any{
			"role":    "assistant",
			"content": assistantText,
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			log.Printf("failed to encode response: %v", err)
		}
	})

	log.Printf("mock-openai listening on http://%s", *addr)
	if err := http.ListenAndServe(*addr, mux); err != nil {
		log.Fatal(err)
	}
}

func echoFromMessages(msgs []struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}) string {
	// Prefer the last user message as the echo base.
	for i := len(msgs) - 1; i >= 0; i-- {
		if strings.EqualFold(msgs[i].Role, "user") {
			s := stripDirective(contentToString(msgs[i].Content))
			if s != "" {
				return s
			}
		}
	}
	return ""
}

func parseDirectiveFromMessages(msgs []struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}) (mockDirective, bool) {
	for i := len(msgs) - 1; i >= 0; i-- {
		if !strings.EqualFold(msgs[i].Role, "user") {
			continue
		}
		raw := contentToString(msgs[i].Content)
		m := mockDirectiveRe.FindStringSubmatch(raw)
		if len(m) != 2 {
			return mockDirective{}, false
		}
		var d mockDirective
		dec := json.NewDecoder(strings.NewReader(m[1]))
		dec.UseNumber()
		if err := dec.Decode(&d); err != nil {
			return mockDirective{}, false
		}
		return d, true
	}
	return mockDirective{}, false
}

func stripDirective(s string) string {
	loc := mockDirectiveRe.FindStringIndex(s)
	if loc == nil {
		return strings.TrimSpace(s)
	}
	// Remove directive portion to get a clean echo.
	return strings.TrimSpace(s[:loc[0]])
}

func contentToString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case []any:
		// OpenAI can send multi-part content; join any text parts.
		var b strings.Builder
		for _, part := range t {
			m, ok := part.(map[string]any)
			if !ok {
				continue
			}
			if m["type"] == "text" {
				if txt, ok := m["text"].(string); ok {
					b.WriteString(txt)
				}
			}
		}
		return b.String()
	default:
		// Best-effort: serialize other shapes.
		bs, _ := json.Marshal(v)
		return string(bs)
	}
}

func writeJSONError(w http.ResponseWriter, status int, errType, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]any{
			"type":    errType,
			"message": msg,
		},
	})
}
