package codegen

import (
	"strings"
	"testing"

	"hooktm/internal/store"
)

func TestRenderFromWebhook_UnsupportedLang(t *testing.T) {
	wh := store.Webhook{ID: "test", Provider: "stripe"}
	_, err := RenderFromWebhook(wh, "rust")
	if err == nil {
		t.Fatal("expected error for unsupported language")
	}
	if !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRenderFromWebhook_Stripe(t *testing.T) {
	wh := store.Webhook{
		ID:        "test123",
		Provider:  "stripe",
		EventType: "payment_intent.succeeded",
		Signature: "t=123,v1=abc",
	}

	langs := []string{"go", "ts", "typescript", "python", "py", "php", "ruby"}
	for _, lang := range langs {
		t.Run(lang, func(t *testing.T) {
			code, err := RenderFromWebhook(wh, lang)
			if err != nil {
				t.Fatalf("RenderFromWebhook error: %v", err)
			}
			if code == "" {
				t.Fatal("expected non-empty code")
			}
			if !strings.Contains(code, "stripe") {
				t.Fatal("expected code to contain 'stripe'")
			}
		})
	}
}

func TestRenderFromWebhook_GitHub(t *testing.T) {
	wh := store.Webhook{
		ID:        "test456",
		Provider:  "github",
		EventType: "push",
		Signature: "sha256=abc",
	}

	code, err := RenderFromWebhook(wh, "go")
	if err != nil {
		t.Fatalf("RenderFromWebhook error: %v", err)
	}
	// Check for GitHub in the generated code (function name is ValidateGitHubWebhook)
	// hmac is lowercase as it's the package name
	if !strings.Contains(code, "GitHub") || !strings.Contains(code, "hmac") {
		t.Fatalf("expected GitHub HMAC validation code, got:\n%s", code)
	}
}

func TestRenderFromWebhook_UnknownProvider(t *testing.T) {
	wh := store.Webhook{
		ID:        "test789",
		Provider:  "",
		Signature: "sha256=xyz",
	}

	code, err := RenderFromWebhook(wh, "go")
	if err != nil {
		t.Fatalf("RenderFromWebhook error: %v", err)
	}
	if !strings.Contains(code, "Generic") && !strings.Contains(code, "HMAC") {
		t.Fatal("expected generic HMAC template for unknown provider")
	}
}
