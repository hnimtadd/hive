package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/hnimtadd/hive/pkg/types"
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
		Index        int    `json:"index"`
		Message      any    `json:"message"`
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

var mockDirectiveRegex = regexp.MustCompile(`(?:#mock\s*)([^"\\]|\\(?:["\\\/bfnrt]|u[0-9a-fA-F]{4}))*`)

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
		_, _ = fmt.Fprintf(w, `{"object":"list","data":[{"id":%q,"object":"model"}]}`+"\n", *defaultModel)
	})

	mux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var req chatCompletionsRequest
		dec := json.NewDecoder(r.Body)
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

		// Keep request parsing permissive so the mock can accept
		// OpenAI-compatible fields (e.g. tools) sent by real clients.
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
					"status":  d.Status,
					"content": d.Content,
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
			Index        int    `json:"index"`
			Message      any    `json:"message"`
			FinishReason string `json:"finish_reason"`
		}, 1)
		resp.Choices[0].Index = 0
		resp.Choices[0].FinishReason = "stop"
		resp.Choices[0].Message = map[string]any{
			"role":    "assistant",
			"content": maybeUnescapeJSONLikeString(assistantText),
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
		if strings.EqualFold(msgs[i].Role, string(types.RoleUser)) {
			raw := contentToString(msgs[i].Content)

			// Unquote the raw string first
			if quoted, err := strconv.Unquote(raw); err == nil {
				raw = quoted
			}

			if raw != "" {
				return raw
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
		if !strings.EqualFold(msgs[i].Role, string(types.RoleUser)) {
			continue
		}
		raw := contentToString(msgs[i].Content)

		// Unquote the raw string first
		if quoted, err := strconv.Unquote(raw); err == nil {
			raw = quoted
		}

		jsonStr, ok := findMockDirective(raw)
		if !ok {
			return mockDirective{}, false
		}

		jsonStr = strings.TrimSpace(strings.TrimPrefix(jsonStr, "#mock"))
		// Unescape the extracted JSON string (handle escaped quotes like \" )
		if unescaped, err := strconv.Unquote("\"" + jsonStr + "\""); err == nil {
			jsonStr = unescaped
		}

		var d mockDirective
		dec := json.NewDecoder(strings.NewReader(jsonStr))
		dec.UseNumber()
		if err := dec.Decode(&d); err != nil {
			return mockDirective{}, false
		}
		fmt.Println(d)
		return d, true
	}
	return mockDirective{}, false
}

// findMockDirective extracts a "#mock { ...json... }" directive using regex.
func findMockDirective(s string) (string, bool) {
	matches := mockDirectiveRegex.FindStringSubmatch(s)
	if len(matches) < 2 {
		return "", false
	}

	return matches[0], true
}

func contentToString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	default:
		bs, _ := json.Marshal(v)
		return string(bs)
	}
}

func maybeUnescapeJSONLikeString(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}
	if strings.HasPrefix(s, "{") || strings.HasPrefix(s, "[") {
		return s
	}
	// Quoted JSON string: "{...}".
	if (strings.HasPrefix(s, "\"") && strings.HasSuffix(s, "\"")) || (strings.HasPrefix(s, "'") && strings.HasSuffix(s, "'")) {
		if unq, err := strconv.Unquote(s); err == nil {
			unq = strings.TrimSpace(unq)
			if strings.HasPrefix(unq, "{") || strings.HasPrefix(unq, "[") {
				return unq
			}
		}
	}
	// Escaped JSON blob: {\"k\":\"v\"}.
	if strings.Contains(s, `\"`) || strings.Contains(s, `\\`) {
		if unq, err := strconv.Unquote("\"" + s + "\""); err == nil {
			unq = strings.TrimSpace(unq)
			if strings.HasPrefix(unq, "{") || strings.HasPrefix(unq, "[") {
				return unq
			}
		}
	}
	return s
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
