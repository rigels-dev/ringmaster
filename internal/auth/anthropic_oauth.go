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
	AuthorizeURL = "https://claude.ai/oauth/authorize"
	TokenURL     = "https://platform.claude.com/v1/oauth/token"
	RedirectURI  = "https://console.anthropic.com/oauth/code/callback"
	Scopes       = "org:create_api_key user:profile user:inference user:sessions:claude_code user:mcp_servers user:file_upload"
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

	q := url.Values{}
	q.Set("code", "true")
	q.Set("client_id", ClientID)
	q.Set("response_type", "code")
	q.Set("redirect_uri", RedirectURI)
	q.Set("scope", Scopes)
	q.Set("code_challenge", challenge)
	q.Set("code_challenge_method", "S256")
	q.Set("state", state)

	return fmt.Sprintf("%s?%s", AuthorizeURL, q.Encode()), verifier, state, nil
}

func ParseAuthCodeInput(raw string) string {
	raw = strings.TrimSpace(raw)
	if strings.Contains(raw, "code=") {
		u, err := url.Parse(raw)
		if err == nil {
			if c := u.Query().Get("code"); c != "" {
				return c
			}
		}
	}
	if strings.Contains(raw, "#") {
		parts := strings.Split(raw, "#")
		return strings.TrimSpace(parts[0])
	}
	return raw
}

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

func ExchangeAuthCode(ctx context.Context, code, verifier, state string) (*TokenResponse, error) {
	cleanCode := ParseAuthCodeInput(code)
	if cleanCode == "" {
		return nil, fmt.Errorf("authorization code is empty")
	}

	payload := map[string]string{
		"grant_type":    "authorization_code",
		"client_id":     ClientID,
		"code":          cleanCode,
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
