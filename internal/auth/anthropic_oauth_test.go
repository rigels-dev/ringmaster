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
	if !strings.Contains(urlStr, "redirect_uri=http%3A%2F%2Flocalhost%3A53692%2Fcallback") {
		t.Errorf("expected redirect_uri=http://localhost:53692/callback in URL, got: %s", urlStr)
	}
	if !strings.Contains(urlStr, "code_challenge=") {
		t.Errorf("expected code_challenge in URL, got: %s", urlStr)
	}
	if verifier == "" || state == "" {
		t.Errorf("expected non-empty verifier and state")
	}
}

func TestParseAuthCode(t *testing.T) {
	cases := []struct {
		input         string
		expectedCode  string
		expectedState string
	}{
		{"rawcode123", "rawcode123", ""},
		{"code123#state456", "code123", "state456"},
		{"http://localhost:53692/callback?code=abc123xyz&state=my_state", "abc123xyz", "my_state"},
		{"code=query_code&state=query_state", "query_code", "query_state"},
	}

	for _, c := range cases {
		code, state := ParseAuthCode(c.input)
		if code != c.expectedCode {
			t.Errorf("for input %s, expected code %s, got %s", c.input, c.expectedCode, code)
		}
		if state != c.expectedState {
			t.Errorf("for input %s, expected state %s, got %s", c.input, c.expectedState, state)
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
