package redactor

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestRedactHeadersCaseInsensitive(t *testing.T) {
	r := New([]string{"Authorization", "Cookie"}, nil, nil, "")
	hdr := http.Header{}
	hdr.Set("Authorization", "Bearer secret")
	hdr.Set("COOKIE", "session=abc")
	hdr.Set("X-Other", "keep")

	out := r.RedactHeaders(hdr)
	if out.Get("Authorization") != "[REDACTED]" {
		t.Fatalf("Authorization = %q, want [REDACTED]", out.Get("Authorization"))
	}
	if out.Get("COOKIE") != "[REDACTED]" {
		t.Fatalf("COOKIE = %q, want [REDACTED]", out.Get("COOKIE"))
	}
	if out.Get("X-Other") != "keep" {
		t.Fatalf("X-Other = %q, want keep", out.Get("X-Other"))
	}
	// Original must not be mutated.
	if hdr.Get("Authorization") != "Bearer secret" {
		t.Fatal("original header was mutated")
	}
}

func TestRedactQuery(t *testing.T) {
	r := New(nil, []string{"token", "password"}, nil, "[REDACTED]")
	v := url.Values{}
	v.Set("user", "aditya")
	v.Set("token", "secret123")
	encoded := r.RedactQuery(v)
	if !strings.Contains(encoded, "user=aditya") {
		t.Fatalf("missing user in %q", encoded)
	}
	if !strings.Contains(encoded, "token=%5BREDACTED%5D") {
		t.Fatalf("token not redacted in %q", encoded)
	}
	if strings.Contains(encoded, "secret123") {
		t.Fatalf("secret leaked in %q", encoded)
	}
}

func TestRedactJSONNested(t *testing.T) {
	r := New(nil, nil, []string{"password", "credit_card.number"}, "[REDACTED]")
	input := []byte(`{"user":"aditya","password":"hunter2","credit_card":{"number":"4111","cvv":"123"},"tags":["a","b"]}`)
	out, err := r.RedactJSON(input)
	if err != nil {
		t.Fatalf("RedactJSON error: %v", err)
	}
	s := string(out)
	if strings.Contains(s, "hunter2") || strings.Contains(s, "4111") {
		t.Fatalf("secret leaked in %s", s)
	}
	if !strings.Contains(s, `"password":"[REDACTED]"`) || !strings.Contains(s, `"number":"[REDACTED]"`) {
		t.Fatalf("redaction markers missing in %s", s)
	}
	if !strings.Contains(s, `"cvv":"123"`) {
		t.Fatalf("non-redacted field corrupted in %s", s)
	}
}

func TestRedactJSONInvalidReturnsError(t *testing.T) {
	r := New(nil, nil, []string{"password"}, "")
	if _, err := r.RedactJSON([]byte("{not json")); err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestRedactJSONEmpty(t *testing.T) {
	r := New(nil, nil, []string{"password"}, "")
	out, err := r.RedactJSON([]byte{})
	if err != nil {
		t.Fatalf("empty input error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("empty input output = %q, want empty", out)
	}
}

func TestRedactAttributes(t *testing.T) {
	r := New(nil, nil, []string{"password"}, "[REDACTED]")
	attrs := map[string]any{"password": "hunter2", "user": "aditya"}
	out := r.RedactAttributes(attrs)
	if out["password"] != "[REDACTED]" {
		t.Fatalf("password = %v, want [REDACTED]", out["password"])
	}
	if out["user"] != "aditya" {
		t.Fatalf("user = %v, want aditya", out["user"])
	}
}
