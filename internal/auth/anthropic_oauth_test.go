package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGenerateAuthURL(t *testing.T) {
	urlStr, verifier, state, err := GenerateAuthURL()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.HasPrefix(urlStr, AuthorizeURL) {
		t.Errorf("expected URL to start with %s, got: %s", AuthorizeURL, urlStr)
	}
	if !strings.Contains(urlStr, "code_challenge=") {
		t.Errorf("expected code_challenge in URL, got: %s", urlStr)
	}
	if verifier == "" || state == "" {
		t.Errorf("expected non-empty verifier and state")
	}
}

func TestParseAuthCodeInput(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"rawcode123", "rawcode123"},
		{"code#state", "code"},
		{"https://console.anthropic.com/oauth/code/callback?code=abc123xyz&state=test", "abc123xyz"},
	}

	for _, c := range cases {
		res := ParseAuthCodeInput(c.input)
		if res != c.expected {
			t.Errorf("for input %s, expected %s, got %s", c.input, c.expected, res)
		}
	}
}

func TestTokenResponseUnmarshal(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := TokenResponse{
			AccessToken:  "mock-access",
			RefreshToken: "mock-refresh",
			ExpiresIn:    3600,
			TokenType:    "Bearer",
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, ts.URL, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var tokenResp TokenResponse
	_ = json.NewDecoder(resp.Body).Decode(&tokenResp)

	if tokenResp.AccessToken != "mock-access" {
		t.Errorf("expected mock-access, got %s", tokenResp.AccessToken)
	}
}
