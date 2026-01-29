package provider

import (
	"encoding/json"
	"net/http"
	"strings"
)

// Detect returns (providerName, eventType, signatureHeaderValue).
// This is intentionally heuristic and safe: unknown is the default.
func Detect(h http.Header, body []byte) (string, string, string) {
	// Stripe
	if sig := h.Get("Stripe-Signature"); strings.TrimSpace(sig) != "" {
		return "stripe", jsonEventType(body, "type"), sig
	}
	// GitHub
	if ev := h.Get("X-GitHub-Event"); strings.TrimSpace(ev) != "" {
		return "github", ev, h.Get("X-Hub-Signature-256")
	}

	return "unknown", "", firstNonEmpty(
		h.Get("X-Hub-Signature-256"),
		h.Get("X-Hub-Signature"),
		h.Get("X-Slack-Signature"),
		h.Get("X-Shopify-Hmac-SHA256"),
		h.Get("X-Twilio-Signature"),
	)
}

func jsonEventType(body []byte, key string) string {
	if len(body) == 0 {
		return ""
	}
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		return ""
	}
	v, ok := m[key]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}

func firstNonEmpty(xs ...string) string {
	for _, x := range xs {
		if strings.TrimSpace(x) != "" {
			return x
		}
	}
	return ""
}
