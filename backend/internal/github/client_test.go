package github

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"Frank2006x/Pipe/internal/config"
)

// mockTransport implements http.RoundTripper to intercept HTTP requests in-memory.
type mockTransport struct {
	fn func(req *http.Request) (*http.Response, error)
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.fn(req)
}

// newTestClient creates a Client configured with a custom mock RoundTripper.
func newTestClient(fn func(req *http.Request) (*http.Response, error)) *Client {
	cfg := config.Config{
		GITHUB_CLIENT_ID:     "test_client_id",
		GITHUB_CLIENT_SECRET: "test_client_secret",
		GITHUB_CALLBACK_URL:  "http://localhost:8080/callback",
	}

	client := NewClient(cfg)
	client.httpClient = &http.Client{
		Transport: &mockTransport{fn: fn},
	}
	return client
}

// ============================================================================
// UNIT TESTS
// ============================================================================

func TestGetAuthURL(t *testing.T) {
	cfg := config.Config{
		GITHUB_CLIENT_ID:    "my_client_id",
		GITHUB_CALLBACK_URL: "http://localhost:3000/api/auth/callback",
	}
	client := NewClient(cfg)

	authURL, err := client.GetAuthURL("random_state_123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedPrefix := "https://github.com/login/oauth/authorize?"
	if !strings.HasPrefix(authURL, expectedPrefix) {
		t.Fatalf("expected url to start with %s, got: %s", expectedPrefix, authURL)
	}
}

func TestExchangeCode_Success(t *testing.T) {
	client := newTestClient(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost {
			t.Errorf("expected POST method, got %s", req.Method)
		}
		if req.URL.String() != tokenURL {
			t.Errorf("expected URL %s, got %s", tokenURL, req.URL.String())
		}

		jsonResp := `{"access_token": "gho_mock_token_123", "token_type": "bearer", "scope": "repo"}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(jsonResp)),
			Header:     make(http.Header),
		}, nil
	})

	resp, err := client.ExchangeCode(context.Background(), "valid_code")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.AccessToken != "gho_mock_token_123" {
		t.Errorf("expected token 'gho_mock_token_123', got '%s'", resp.AccessToken)
	}
}

func TestGetUser_Success(t *testing.T) {
	client := newTestClient(func(req *http.Request) (*http.Response, error) {
		if req.Header.Get("Authorization") != "Bearer mock_token" {
			t.Errorf("missing or invalid authorization header")
		}
		if req.URL.String() != userEndpoint {
			t.Errorf("expected URL %s, got %s", userEndpoint, req.URL.String())
		}

		jsonResp := `{"id": 101, "login": "octocat", "name": "Monomer Cat", "avatar_url": "https://github.com/avatar.png"}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(jsonResp)),
			Header:     make(http.Header),
		}, nil
	})

	user, err := client.GetUser(context.Background(), "mock_token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if user.Login != "octocat" || user.ID != 101 {
		t.Errorf("unexpected user mapping: %+v", user)
	}
}

func TestListRepositories_Success(t *testing.T) {
	client := newTestClient(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() != listAllRepositoriesEndpoint {
			t.Errorf("expected URL %s, got %s", listAllRepositoriesEndpoint, req.URL.String())
		}

		jsonResp := `[{"id": 1, "name": "Pipe", "full_name": "Frank2006x/Pipe", "private": false}]`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(jsonResp)),
			Header:     make(http.Header),
		}, nil
	})

	repos, err := client.ListRepositories(context.Background(), "mock_token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(repos) != 1 || repos[0].Name != "Pipe" {
		t.Errorf("unexpected repositories list: %+v", repos)
	}
}

func TestCreateWebhook_Success(t *testing.T) {
	client := newTestClient(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost {
			t.Errorf("expected POST method, got %s", req.Method)
		}

		jsonResp := `{"id": 9988}`
		return &http.Response{
			StatusCode: http.StatusCreated,
			Body:       io.NopCloser(strings.NewReader(jsonResp)),
			Header:     make(http.Header),
		}, nil
	})

	whConfig := WebhookConfig{
		URL:         "https://pipe.example.com/webhook",
		ContentType: "json",
		Secret:      "supersecret",
	}

	webhook, err := client.CreateWebhook(context.Background(), "token", "owner", "repo", whConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if webhook.ID != 9988 {
		t.Errorf("expected webhook ID 9988, got %d", webhook.ID)
	}
}

func TestDo_ErrorHandling(t *testing.T) {
	client := newTestClient(func(req *http.Request) (*http.Response, error) {
		jsonResp := `{"message": "Bad credentials"}`
		return &http.Response{
			StatusCode: http.StatusUnauthorized,
			Body:       io.NopCloser(strings.NewReader(jsonResp)),
			Header:     make(http.Header),
		}, nil
	})

	req, err := client.newRequest(context.Background(), http.MethodGet, userEndpoint, http.NoBody, "bad_token")
	if err != nil {
		t.Fatalf("failed creating request: %v", err)
	}

	err = client.do(req, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if err.Error() != "github: Bad credentials" {
		t.Errorf("expected 'github: Bad credentials', got '%s'", err.Error())
	}
}

// ============================================================================
// BENCHMARKS
// ============================================================================

func BenchmarkClient_NewRequestCreation(b *testing.B) {
	cfg := config.Config{}
	client := NewClient(cfg)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = client.newRequest(ctx, http.MethodGet, userEndpoint, http.NoBody, "test_token")
	}
}

func BenchmarkClient_DoResponseParsing(b *testing.B) {
	jsonResp := []byte(`[{"id": 1, "name": "Pipe", "full_name": "Frank2006x/Pipe", "private": false, "created_at": "2026-01-01T00:00:00Z"}]`)

	client := newTestClient(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader(jsonResp)),
			Header:     make(http.Header),
		}, nil
	})

	ctx := context.Background()
	req, _ := client.newRequest(ctx, http.MethodGet, listAllRepositoriesEndpoint, http.NoBody, "token")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var repos []Repository
		_ = client.do(req, &repos)
	}
}
