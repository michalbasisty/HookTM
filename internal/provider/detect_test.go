package provider

import (
	"net/http"
	"testing"
)

func TestDetect_Stripe(t *testing.T) {
	h := http.Header{}
	h.Set("Stripe-Signature", "t=1,v1=abc")
	body := []byte(`{"type":"payment_intent.succeeded"}`)
	prov, ev, sig := Detect(h, body)
	if prov != "stripe" {
		t.Fatalf("prov=%q", prov)
	}
	if ev != "payment_intent.succeeded" {
		t.Fatalf("ev=%q", ev)
	}
	if sig == "" {
		t.Fatalf("sig empty")
	}
}

func TestDetect_GitHub(t *testing.T) {
	h := http.Header{}
	h.Set("X-GitHub-Event", "push")
	h.Set("X-Hub-Signature-256", "sha256=deadbeef")
	prov, ev, sig := Detect(h, []byte(`{}`))
	if prov != "github" {
		t.Fatalf("prov=%q", prov)
	}
	if ev != "push" {
		t.Fatalf("ev=%q", ev)
	}
	if sig != "sha256=deadbeef" {
		t.Fatalf("sig=%q", sig)
	}
}

func TestDetect_Unknown(t *testing.T) {
	h := http.Header{}
	h.Set("X-Slack-Signature", "v0=abc123")
	prov, ev, sig := Detect(h, []byte(`{}`))
	if prov != "unknown" {
		t.Fatalf("prov=%q, want unknown", prov)
	}
	if ev != "" {
		t.Fatalf("ev=%q, want empty", ev)
	}
	if sig != "v0=abc123" {
		t.Fatalf("sig=%q, want v0=abc123", sig)
	}
}

func TestDetect_Shopify(t *testing.T) {
	h := http.Header{}
	h.Set("X-Shopify-Hmac-SHA256", "abc123")
	prov, _, sig := Detect(h, []byte(`{}`))
	if prov != "unknown" {
		t.Fatalf("prov=%q, want unknown (shopify not yet implemented)", prov)
	}
	if sig != "abc123" {
		t.Fatalf("sig=%q", sig)
	}
}
