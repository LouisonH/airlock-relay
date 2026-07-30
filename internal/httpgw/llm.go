package httpgw

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/LouisonH/airlock-relay/internal/routes"
)

const maxLLMUsageCaptureBytes = 8 << 20

type llmUsageCapture struct {
	payload   []byte
	truncated bool
}

func newLLMUsageCapture() *llmUsageCapture {
	return &llmUsageCapture{payload: make([]byte, 0, 32<<10)}
}

func (c *llmUsageCapture) Write(payload []byte) (int, error) {
	if c == nil || c.truncated {
		return len(payload), nil
	}
	if len(c.payload)+len(payload) > maxLLMUsageCaptureBytes {
		c.truncated = true
		clear(c.payload)
		c.payload = c.payload[:0]
		return len(payload), nil
	}
	c.payload = append(c.payload, payload...)
	return len(payload), nil
}

func (c *llmUsageCapture) TokenUsage(streaming bool) (uint64, uint64) {
	if c == nil || c.truncated || len(c.payload) == 0 {
		return 0, 0
	}
	if !streaming {
		return extractLLMTokenUsage(c.payload)
	}
	var inputTokens, outputTokens uint64
	for _, line := range bytes.Split(c.payload, []byte{'\n'}) {
		line = bytes.TrimSpace(line)
		if !bytes.HasPrefix(line, []byte("data:")) {
			continue
		}
		data := bytes.TrimSpace(bytes.TrimPrefix(line, []byte("data:")))
		if len(data) == 0 || bytes.Equal(data, []byte("[DONE]")) {
			continue
		}
		input, output := extractLLMTokenUsage(data)
		inputTokens = max(inputTokens, input)
		outputTokens = max(outputTokens, output)
	}
	return inputTokens, outputTokens
}

func (c *llmUsageCapture) Clear() {
	if c == nil {
		return
	}
	clear(c.payload)
	c.payload = nil
	c.truncated = false
}

func extractLLMTokenUsage(payload []byte) (uint64, uint64) {
	var object map[string]json.RawMessage
	if json.Unmarshal(payload, &object) != nil || object == nil {
		return 0, 0
	}
	var inputTokens, outputTokens uint64
	if usage, ok := object["usage"]; ok {
		inputTokens, outputTokens = parseLLMUsageObject(usage)
	}
	for _, key := range []string{"response", "message"} {
		if nested, ok := object[key]; ok {
			input, output := extractLLMTokenUsage(nested)
			inputTokens = max(inputTokens, input)
			outputTokens = max(outputTokens, output)
		}
	}
	return inputTokens, outputTokens
}

func parseLLMUsageObject(payload json.RawMessage) (uint64, uint64) {
	var usage map[string]json.RawMessage
	if json.Unmarshal(payload, &usage) != nil || usage == nil {
		return 0, 0
	}
	inputTokens := firstTokenCount(usage, "input_tokens", "prompt_tokens")
	if _, anthropic := usage["input_tokens"]; anthropic {
		inputTokens += firstTokenCount(usage, "cache_creation_input_tokens")
		inputTokens += firstTokenCount(usage, "cache_read_input_tokens")
	}
	return inputTokens, firstTokenCount(usage, "output_tokens", "completion_tokens")
}

func firstTokenCount(usage map[string]json.RawMessage, keys ...string) uint64 {
	for _, key := range keys {
		var count uint64
		if raw, ok := usage[key]; ok && json.Unmarshal(raw, &count) == nil {
			return count
		}
	}
	return 0
}

type llmRequestError struct {
	Status  int
	Code    string
	Message string
}

func prepareLLMRequest(request *http.Request, route routes.HTTPRoute, endpointPath string) ([]byte, *llmRequestError) {
	limit := route.Policy.MaxRequestBytes
	if request == nil || request.Body == nil {
		return nil, invalidLLMRequest("request body is required")
	}
	if request.ContentLength > limit {
		return nil, &llmRequestError{Status: http.StatusRequestEntityTooLarge, Code: "request_too_large", Message: "request body is too large"}
	}
	payload, err := io.ReadAll(io.LimitReader(request.Body, limit+1))
	if err != nil {
		return nil, invalidLLMRequest("request body could not be read")
	}
	if int64(len(payload)) > limit {
		clear(payload)
		return nil, &llmRequestError{Status: http.StatusRequestEntityTooLarge, Code: "request_too_large", Message: "request body is too large"}
	}
	defer clear(payload)

	var object map[string]json.RawMessage
	if err := json.Unmarshal(payload, &object); err != nil || object == nil {
		return nil, invalidLLMRequest("request body must be a JSON object")
	}
	var model string
	if raw, ok := object["model"]; !ok || json.Unmarshal(raw, &model) != nil || !route.Policy.AllowsModel(model) {
		return nil, &llmRequestError{Status: http.StatusForbidden, Code: "model_not_allowed", Message: "model is not allowed by this route"}
	}
	if requestError := enforceOutputLimit(object, route.Provider, endpointPath, route.Policy.MaxOutputTokens); requestError != nil {
		return nil, requestError
	}
	validated, err := json.Marshal(object)
	if err != nil {
		return nil, invalidLLMRequest("request body could not be validated")
	}
	return validated, nil
}

func enforceOutputLimit(object map[string]json.RawMessage, provider, endpointPath string, limit int) *llmRequestError {
	field := "max_tokens"
	if provider == routes.ProviderOpenAI {
		switch endpointPath {
		case "/v1/responses":
			field = "max_output_tokens"
		case "/v1/chat/completions":
			_, modern := object["max_completion_tokens"]
			_, legacy := object["max_tokens"]
			if modern && legacy {
				return invalidLLMRequest("use only one output token field")
			}
			if legacy {
				field = "max_tokens"
			} else {
				field = "max_completion_tokens"
			}
		default:
			return invalidLLMRequest("endpoint is not supported")
		}
	}
	if raw, ok := object[field]; ok {
		var requested int
		if json.Unmarshal(raw, &requested) != nil || requested < 1 {
			return invalidLLMRequest(fmt.Sprintf("%s must be a positive integer", field))
		}
		if requested > limit {
			return &llmRequestError{Status: http.StatusBadRequest, Code: "output_limit_exceeded", Message: "requested output tokens exceed this route's limit"}
		}
		return nil
	}
	encoded, _ := json.Marshal(limit)
	object[field] = encoded
	return nil
}

func invalidLLMRequest(message string) *llmRequestError {
	return &llmRequestError{Status: http.StatusBadRequest, Code: "invalid_request", Message: message}
}

func writeLLMError(w http.ResponseWriter, provider string, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	var payload any
	if provider == routes.ProviderAnthropic {
		payload = map[string]any{
			"type":  "error",
			"error": map[string]string{"type": code, "message": "airlock: " + message},
		}
	} else {
		payload = map[string]any{
			"error": map[string]string{"message": "airlock: " + message, "type": "airlock_policy_error", "code": code},
		}
	}
	_ = json.NewEncoder(w).Encode(payload)
}
