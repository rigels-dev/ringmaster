package auth

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	ClientID     = "9d1c250a-e61b-44d9-88ed-5944d1962f5e"
	AuthorizeURL = "https://claude.com/cai/oauth/authorize"
	TokenURL     = "https://platform.claude.com/v1/oauth/token"
	RedirectURI  = "https://platform.claude.com/oauth/code/callback"
	Scopes       = "user:inference"
)

type AuthStatus struct {
	LoggedIn         bool   `json:"loggedIn"`
	Email            string `json:"email"`
	SubscriptionType string `json:"subscriptionType"`
	AuthMethod       string `json:"authMethod"`
}

func CheckClaudeCLIStatus() AuthStatus {
	cmd := exec.Command("claude", "auth", "status")
	out, err := cmd.Output()
	if err != nil {
		return AuthStatus{LoggedIn: false}
	}

	var status AuthStatus
	if err := json.Unmarshal(out, &status); err != nil {
		return AuthStatus{LoggedIn: false}
	}
	return status
}

func GeneratePKCE() (verifier, challenge string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", err
	}
	verifier = base64.RawURLEncoding.EncodeToString(b)
	hash := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(hash[:])
	return verifier, challenge, nil
}

func GenerateAuthURL() (authURL, verifier, state string, err error) {
	verifier, challenge, err := GeneratePKCE()
	if err != nil {
		return "", "", "", err
	}
	state = verifier

	// Order parameters with redirect_uri and client_id first so they are never truncated by terminal line wraps
	params := fmt.Sprintf(
		"redirect_uri=%s&client_id=%s&response_type=code&code=true&scope=%s&code_challenge=%s&code_challenge_method=S256&state=%s",
		url.QueryEscape(RedirectURI),
		url.QueryEscape(ClientID),
		url.QueryEscape(Scopes),
		url.QueryEscape(challenge),
		url.QueryEscape(state),
	)

	return fmt.Sprintf("%s?%s", AuthorizeURL, params), verifier, state, nil
}

func ParseAuthCode(raw string) (code, state string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", ""
	}

	// Try URL parsing
	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		u, err := url.Parse(raw)
		if err == nil {
			c := u.Query().Get("code")
			s := u.Query().Get("state")
			if c != "" {
				return c, s
			}
		}
	}

	// Handle code#state
	if strings.Contains(raw, "#") {
		parts := strings.SplitN(raw, "#", 2)
		return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
	}

	// Handle query string without scheme (e.g. code=...&state=...)
	if strings.Contains(raw, "code=") {
		parts := strings.Split(raw, "&")
		var extractedCode, extractedState string
		for _, p := range parts {
			kv := strings.SplitN(p, "=", 2)
			if len(kv) == 2 {
				k := strings.TrimSpace(kv[0])
				v := strings.TrimSpace(kv[1])
				if k == "code" {
					extractedCode = v
				} else if k == "state" {
					extractedState = v
				}
			}
		}
		if extractedCode != "" {
			return extractedCode, extractedState
		}
	}

	return raw, ""
}

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

func ExchangeAuthCode(ctx context.Context, inputCode, verifier, defaultState string) (*TokenResponse, error) {
	code, state := ParseAuthCode(inputCode)
	if code == "" {
		return nil, fmt.Errorf("authorization code is empty")
	}
	if state == "" {
		state = defaultState
	}

	payload := map[string]string{
		"grant_type":    "authorization_code",
		"client_id":     ClientID,
		"code":          code,
		"state":         state,
		"redirect_uri":  RedirectURI,
		"code_verifier": verifier,
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, TokenURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token exchange failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("token exchange error (HTTP %d): %s", resp.StatusCode, string(respBody))
	}

	var tokenResp TokenResponse
	if err := json.Unmarshal(respBody, &tokenResp); err != nil {
		return nil, fmt.Errorf("invalid token response: %w", err)
	}

	return &tokenResp, nil
}

func SaveClaudeCredentials(tokens *TokenResponse) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	claudeDir := filepath.Join(home, ".claude")
	_ = os.MkdirAll(claudeDir, 0700)
	credPath := filepath.Join(claudeDir, ".credentials.json")

	data := map[string]any{}
	if existingBytes, err := os.ReadFile(credPath); err == nil {
		_ = json.Unmarshal(existingBytes, &data)
	}

	nowMs := time.Now().UnixMilli()
	expiresAt := nowMs + (tokens.ExpiresIn * 1000)

	oauthData := map[string]any{
		"accessToken":            tokens.AccessToken,
		"refreshToken":           tokens.RefreshToken,
		"expiresAt":              expiresAt,
		"refreshTokenExpiresAt":  nowMs + (30 * 24 * 3600 * 1000),
		"scopes":                 strings.Split(Scopes, " "),
		"subscriptionType":       "pro",
		"subscriptionTypeSource": "claude",
	}

	data["claudeAiOauth"] = oauthData

	outBytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(credPath, outBytes, 0600)
}
