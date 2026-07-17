package provider

import (
	"errors"
	"strings"
	"testing"
)

func TestIsUnusedAddress(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "blockfrost 404 not found",
			err:  errors.New(`blockfrost API error 404: {"status_code":404,"error":"Not Found","message":"The requested component has not been found."}`),
			want: true,
		},
		{
			name: "json status only",
			err:  errors.New(`{"status_code":404,"error":"Not Found","message":"The requested component has not been found."}`),
			want: true,
		},
		{
			name: "auth 403",
			err:  errors.New(`blockfrost API error 403: {"status_code":403,"error":"Forbidden"}`),
			want: false,
		},
		{
			name: "rate limit",
			err:  errors.New(`blockfrost API error 429: rate limited`),
			want: false,
		},
		{
			name: "nil",
			err:  nil,
			want: false,
		},
		{
			name: "unrelated",
			err:  errors.New("connection refused"),
			want: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsUnusedAddress(tc.err); got != tc.want {
				t.Fatalf("IsUnusedAddress(%v)=%v want %v", tc.err, got, tc.want)
			}
		})
	}
}

func TestIsAuthFailureAndRateLimited(t *testing.T) {
	auth := errors.New(`blockfrost API error 401: unauthorized`)
	if !IsAuthFailure(auth) {
		t.Fatal("expected auth failure")
	}
	rl := errors.New(`blockfrost API error 429: slow down`)
	if !IsRateLimited(rl) {
		t.Fatal("expected rate limit")
	}
}

func TestExplain(t *testing.T) {
	raw := errors.New(`blockfrost API error 403: {"status_code":403,"error":"Forbidden"}`)
	got := Explain(raw)
	if got == nil {
		t.Fatal("expected explained error")
	}
	if !errors.Is(got, raw) {
		t.Fatalf("expected unwrap to raw: %v", got)
	}
	if !strings.Contains(got.Error(), "authentication failed") {
		t.Fatalf("unexpected message: %v", got)
	}
}

func TestExplainUnusedKeepsUnwrap(t *testing.T) {
	raw := errors.New(`blockfrost API error 404: {"status_code":404,"error":"Not Found","message":"The requested component has not been found."}`)
	got := Explain(raw)
	if !errors.Is(got, raw) {
		t.Fatal("expected unwrap")
	}
	if !IsUnusedAddress(raw) {
		t.Fatal("fixture should classify as unused")
	}
}
