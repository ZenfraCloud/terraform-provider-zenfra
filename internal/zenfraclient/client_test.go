// ABOUTME: Unit tests for the Zenfra API client using httptest.
// ABOUTME: Tests auth headers, error parsing, retry logic, and CRUD operations per resource type.

package zenfraclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// newTestClient creates a Client pointing at the given httptest.Server.
func newTestClient(t *testing.T, server *httptest.Server) *Client {
	t.Helper()
	c, err := NewClient(ClientConfig{
		Endpoint:   server.URL,
		APIToken:   "test-token-abc123",
		MaxRetries: 1,
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return c
}

func TestNewClient_Validation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		cfg     ClientConfig
		wantErr bool
	}{
		{
			name:    "missing endpoint",
			cfg:     ClientConfig{APIToken: "tok"},
			wantErr: true,
		},
		{
			name:    "missing token",
			cfg:     ClientConfig{Endpoint: "https://api.example.com"},
			wantErr: true,
		},
		{
			name: "valid config",
			cfg: ClientConfig{
				Endpoint: "https://api.example.com",
				APIToken: "tok",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := NewClient(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewClient() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestAuthHeaderSent(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-token-abc123" {
			t.Errorf("expected Bearer test-token-abc123, got %q", auth)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		ua := r.Header.Get("User-Agent")
		if ua == "" {
			t.Error("expected User-Agent header, got empty")
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"id": "org1", "name": "Test Org", "slug": "test"})
	}))
	defer server.Close()

	client := newTestClient(t, server)
	org, err := client.GetCurrentOrganization(context.Background())
	if err != nil {
		t.Fatalf("GetCurrentOrganization: %v", err)
	}
	if org.Name != "Test Org" {
		t.Errorf("expected name 'Test Org', got %q", org.Name)
	}
}

func TestErrorParsing_404(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "not_found", "message": "space not found"})
	}))
	defer server.Close()

	client := newTestClient(t, server)
	_, err := client.GetSpace(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !IsNotFound(err) {
		t.Errorf("expected NotFoundError, got %T: %v", err, err)
	}
}

func TestErrorParsing_409(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "duplicate_slug", "message": "already exists"})
	}))
	defer server.Close()

	client := newTestClient(t, server)
	_, err := client.CreateSpace(context.Background(), CreateSpaceRequest{Name: "test", Slug: "test"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !IsConflict(err) {
		t.Errorf("expected ConflictError, got %T: %v", err, err)
	}
}

func TestErrorParsing_401(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "unauthenticated", "message": "invalid token"})
	}))
	defer server.Close()

	client := newTestClient(t, server)
	_, err := client.GetCurrentOrganization(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !IsUnauthorized(err) {
		t.Errorf("expected UnauthorizedError, got %T: %v", err, err)
	}
}

func TestErrorParsing_422(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error":   "validation_error",
			"message": "name is required",
			"fields":  map[string]string{"name": "required"},
		})
	}))
	defer server.Close()

	client := newTestClient(t, server)
	_, err := client.CreateStack(context.Background(), CreateStackRequest{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var ve *ValidationError
	if !isValidationErr(err, &ve) {
		t.Errorf("expected ValidationError, got %T: %v", err, err)
	}
	if ve != nil && ve.Message != "name is required" {
		t.Errorf("expected message 'name is required', got %q", ve.Message)
	}
}

func isValidationErr(err error, target **ValidationError) bool {
	if err == nil {
		return false
	}
	var ve *ValidationError
	if errors.As(err, &ve) {
		*target = ve
		return true
	}
	return false
}

func TestRetryOn429(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := attempts.Add(1)
		if n <= 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"id": "org1", "name": "Test", "slug": "test"})
	}))
	defer server.Close()

	client := newTestClient(t, server)
	org, err := client.GetCurrentOrganization(context.Background())
	if err != nil {
		t.Fatalf("expected success after retry, got: %v", err)
	}
	if org.Name != "Test" {
		t.Errorf("expected name 'Test', got %q", org.Name)
	}
	if got := attempts.Load(); got != 2 {
		t.Errorf("expected 2 attempts, got %d", got)
	}
}

func TestRetryOn503(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := attempts.Add(1)
		if n <= 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"id": "space1", "name": "Test Space"})
	}))
	defer server.Close()

	client := newTestClient(t, server)
	space, err := client.GetSpace(context.Background(), "space1")
	if err != nil {
		t.Fatalf("expected success after retry, got: %v", err)
	}
	if space.Name != "Test Space" {
		t.Errorf("expected name 'Test Space', got %q", space.Name)
	}
}

func TestCRUD_Space(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/spaces", func(w http.ResponseWriter, r *http.Request) {
		var req CreateSpaceRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(Space{
			ID:   "space-1",
			Name: req.Name,
			Slug: req.Slug,
		})
	})
	mux.HandleFunc("GET /api/v1/spaces/space-1", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(Space{ID: "space-1", Name: "test"})
	})
	mux.HandleFunc("PUT /api/v1/spaces/space-1", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(Space{ID: "space-1", Name: "updated"})
	})
	mux.HandleFunc("DELETE /api/v1/spaces/space-1", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /api/v1/spaces", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"items": []Space{{ID: "space-1", Name: "test"}}})
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	client := newTestClient(t, server)
	ctx := context.Background()

	// Create
	space, err := client.CreateSpace(ctx, CreateSpaceRequest{Name: "test", Slug: "test"})
	if err != nil {
		t.Fatalf("CreateSpace: %v", err)
	}
	if space.ID != "space-1" {
		t.Errorf("expected id space-1, got %s", space.ID)
	}

	// Get
	got, err := client.GetSpace(ctx, "space-1")
	if err != nil {
		t.Fatalf("GetSpace: %v", err)
	}
	if got.ID != "space-1" {
		t.Errorf("expected id space-1, got %s", got.ID)
	}

	// List
	spaces, err := client.ListSpaces(ctx)
	if err != nil {
		t.Fatalf("ListSpaces: %v", err)
	}
	if len(spaces) != 1 {
		t.Errorf("expected 1 space, got %d", len(spaces))
	}

	// Update
	name := "updated"
	updated, err := client.UpdateSpace(ctx, "space-1", UpdateSpaceRequest{Name: &name})
	if err != nil {
		t.Fatalf("UpdateSpace: %v", err)
	}
	if updated.Name != "updated" {
		t.Errorf("expected name 'updated', got %q", updated.Name)
	}

	// Delete
	if err := client.DeleteSpace(ctx, "space-1"); err != nil {
		t.Fatalf("DeleteSpace: %v", err)
	}
}

func TestCRUD_Stack(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/stacks", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(Stack{ID: "stack-1", Name: "my-stack"})
	})
	mux.HandleFunc("GET /api/v1/stacks/stack-1", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(Stack{ID: "stack-1", Name: "my-stack"})
	})
	mux.HandleFunc("DELETE /api/v1/stacks/stack-1", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /api/v1/stacks/stack-1/variables", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(GetStackVariablesResponse{
			Variables: []StackVariable{
				{Key: "DB_HOST", Value: "localhost", Secret: false},
				{Key: "DB_PASS", Value: "****", Secret: true},
			},
		})
	})
	mux.HandleFunc("PUT /api/v1/stacks/stack-1/variables", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(GetStackVariablesResponse{
			Variables: []StackVariable{
				{Key: "DB_HOST", Value: "localhost", Secret: false},
			},
		})
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	client := newTestClient(t, server)
	ctx := context.Background()

	// Create
	stack, err := client.CreateStack(ctx, CreateStackRequest{
		SpaceID: "space-1",
		Name:    "my-stack",
		IAC:     IACConfig{Engine: "terraform", Version: "1.5.0"},
		Source: StackSource{
			Type: "raw_git",
			RawGit: &StackSourceRawGit{
				URL: "https://github.com/example/repo.git",
				Ref: StackSourceRef{Type: "branch", Name: "main"},
			},
		},
	})
	if err != nil {
		t.Fatalf("CreateStack: %v", err)
	}
	if stack.ID != "stack-1" {
		t.Errorf("expected stack id stack-1, got %s", stack.ID)
	}

	// Get
	got, err := client.GetStack(ctx, "stack-1")
	if err != nil {
		t.Fatalf("GetStack: %v", err)
	}
	if got.Name != "my-stack" {
		t.Errorf("expected name my-stack, got %s", got.Name)
	}

	// Variables
	vars, err := client.GetStackVariables(ctx, "stack-1")
	if err != nil {
		t.Fatalf("GetStackVariables: %v", err)
	}
	if len(vars) != 2 {
		t.Errorf("expected 2 variables, got %d", len(vars))
	}
	if vars[1].Value != "****" {
		t.Errorf("expected masked secret, got %q", vars[1].Value)
	}

	// Set Variables
	newVars, err := client.SetStackVariables(ctx, "stack-1", []StackVariable{
		{Key: "DB_HOST", Value: "localhost"},
	})
	if err != nil {
		t.Fatalf("SetStackVariables: %v", err)
	}
	if len(newVars) != 1 {
		t.Errorf("expected 1 variable, got %d", len(newVars))
	}

	// Delete
	if err := client.DeleteStack(ctx, "stack-1"); err != nil {
		t.Fatalf("DeleteStack: %v", err)
	}
}

func TestCRUD_WorkerPool(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/worker-pools", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(CreateWorkerPoolResponse{
			Pool:   WorkerPool{ID: "pool-1", Name: "my-pool"},
			APIKey: "secret-api-key-only-shown-once",
		})
	})
	mux.HandleFunc("GET /api/v1/worker-pools/pool-1", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(WorkerPool{ID: "pool-1", Name: "my-pool"})
	})
	mux.HandleFunc("DELETE /api/v1/worker-pools/pool-1", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	client := newTestClient(t, server)
	ctx := context.Background()

	// Create (returns write-once API key)
	resp, err := client.CreateWorkerPool(ctx, CreateWorkerPoolRequest{Name: "my-pool"})
	if err != nil {
		t.Fatalf("CreateWorkerPool: %v", err)
	}
	if resp.APIKey != "secret-api-key-only-shown-once" {
		t.Errorf("expected api key, got %q", resp.APIKey)
	}
	if resp.Pool.ID != "pool-1" {
		t.Errorf("expected pool id pool-1, got %s", resp.Pool.ID)
	}

	// Get
	pool, err := client.GetWorkerPool(ctx, "pool-1")
	if err != nil {
		t.Fatalf("GetWorkerPool: %v", err)
	}
	if pool.Name != "my-pool" {
		t.Errorf("expected name my-pool, got %s", pool.Name)
	}

	// Delete
	if err := client.DeleteWorkerPool(ctx, "pool-1"); err != nil {
		t.Fatalf("DeleteWorkerPool: %v", err)
	}
}

func TestCRUD_Bundle(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/bundles", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(Bundle{ID: "bundle-1", Name: "env-config"})
	})
	mux.HandleFunc("GET /api/v1/bundles/bundle-1", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(Bundle{
			ID:   "bundle-1",
			Name: "env-config",
			EnvironmentVariables: []EnvVariable{
				{Key: "API_KEY", Value: "", Secret: true},
			},
		})
	})
	mux.HandleFunc("DELETE /api/v1/bundles/bundle-1", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	client := newTestClient(t, server)
	ctx := context.Background()

	// Create
	bundle, err := client.CreateBundle(ctx, CreateBundleRequest{Name: "env-config", Slug: "env-config"})
	if err != nil {
		t.Fatalf("CreateBundle: %v", err)
	}
	if bundle.ID != "bundle-1" {
		t.Errorf("expected bundle id bundle-1, got %s", bundle.ID)
	}

	// Get (secret values should be empty)
	got, err := client.GetBundle(ctx, "bundle-1")
	if err != nil {
		t.Fatalf("GetBundle: %v", err)
	}
	if len(got.EnvironmentVariables) != 1 {
		t.Fatalf("expected 1 env var, got %d", len(got.EnvironmentVariables))
	}
	if got.EnvironmentVariables[0].Value != "" {
		t.Errorf("expected empty value for secret, got %q", got.EnvironmentVariables[0].Value)
	}

	// Delete
	if err := client.DeleteBundle(ctx, "bundle-1"); err != nil {
		t.Fatalf("DeleteBundle: %v", err)
	}
}

func TestCRUD_Token(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/tokens", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(CreateTokenResponse{
			Token:    "zat_secret_token_shown_once",
			TokenObj: Token{ID: "tok-1", Name: "ci-token"},
		})
	})
	mux.HandleFunc("GET /api/v1/tokens/tok-1", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(Token{ID: "tok-1", Name: "ci-token", TokenPrefix: "zat_secr"})
	})
	mux.HandleFunc("DELETE /api/v1/tokens/tok-1", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	client := newTestClient(t, server)
	ctx := context.Background()

	// Create (returns write-once token)
	resp, err := client.CreateToken(ctx, CreateTokenRequest{Name: "ci-token", Role: "admin"})
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}
	if resp.Token != "zat_secret_token_shown_once" {
		t.Errorf("expected token value, got %q", resp.Token)
	}

	// Get
	tok, err := client.GetToken(ctx, "tok-1")
	if err != nil {
		t.Fatalf("GetToken: %v", err)
	}
	if tok.TokenPrefix != "zat_secr" {
		t.Errorf("expected prefix zat_secr, got %s", tok.TokenPrefix)
	}

	// Delete
	if err := client.DeleteToken(ctx, "tok-1"); err != nil {
		t.Fatalf("DeleteToken: %v", err)
	}
}

func TestCRUD_VCSIntegration(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/vcs/integrations", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(VCSIntegration{
			ID:       "vcs-1",
			Provider: "github",
			Status:   "active",
		})
	})
	mux.HandleFunc("GET /api/v1/vcs/integrations/vcs-1", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(VCSIntegration{
			ID:       "vcs-1",
			Provider: "github",
			Status:   "active",
			GitHub:   &VCSGitHubConfig{InstallationID: 12345},
		})
	})
	mux.HandleFunc("DELETE /api/v1/vcs/integrations/vcs-1", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /api/v1/vcs/integrations", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		all := []VCSIntegration{
			{ID: "vcs-1", Provider: "github", Status: "active"},
			{ID: "vcs-2", Provider: "gitlab", Status: "inactive"},
		}

		provider := r.URL.Query().Get("provider")
		status := r.URL.Query().Get("status")

		var filtered []VCSIntegration
		for _, v := range all {
			if provider != "" && v.Provider != provider {
				continue
			}
			if status != "" && v.Status != status {
				continue
			}
			filtered = append(filtered, v)
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"integrations": filtered,
			"total":        len(filtered),
			"limit":        100,
			"offset":       0,
		})
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	client := newTestClient(t, server)
	ctx := context.Background()

	// Create
	vcs, err := client.CreateVCSIntegration(ctx, CreateVCSIntegrationRequest{
		Provider: "github",
		GitHub:   &CreateVCSGitHubRequest{InstallationID: 12345},
	})
	if err != nil {
		t.Fatalf("CreateVCSIntegration: %v", err)
	}
	if vcs.ID != "vcs-1" {
		t.Errorf("expected id vcs-1, got %s", vcs.ID)
	}

	// Get
	got, err := client.GetVCSIntegration(ctx, "vcs-1")
	if err != nil {
		t.Fatalf("GetVCSIntegration: %v", err)
	}
	if got.GitHub == nil || got.GitHub.InstallationID != 12345 {
		t.Errorf("expected github installation_id 12345, got %+v", got.GitHub)
	}

	// List (no filter)
	all, err := client.ListVCSIntegrations(ctx, nil)
	if err != nil {
		t.Fatalf("ListVCSIntegrations: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("expected 2 integrations, got %d", len(all))
	}

	// List (filter by provider)
	ghProvider := "github"
	ghOnly, err := client.ListVCSIntegrations(ctx, &ListVCSIntegrationsOptions{Provider: &ghProvider})
	if err != nil {
		t.Fatalf("ListVCSIntegrations(provider=github): %v", err)
	}
	if len(ghOnly) != 1 {
		t.Errorf("expected 1 github integration, got %d", len(ghOnly))
	}
	if len(ghOnly) > 0 && ghOnly[0].Provider != "github" {
		t.Errorf("expected provider github, got %s", ghOnly[0].Provider)
	}

	// List (filter by status)
	activeStatus := "active"
	activeOnly, err := client.ListVCSIntegrations(ctx, &ListVCSIntegrationsOptions{Status: &activeStatus})
	if err != nil {
		t.Fatalf("ListVCSIntegrations(status=active): %v", err)
	}
	if len(activeOnly) != 1 {
		t.Errorf("expected 1 active integration, got %d", len(activeOnly))
	}

	// Delete
	if err := client.DeleteVCSIntegration(ctx, "vcs-1"); err != nil {
		t.Fatalf("DeleteVCSIntegration: %v", err)
	}
}

func TestCRUD_BundleAttachments(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/stacks/stack-1/bundles", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})
	mux.HandleFunc("DELETE /api/v1/stacks/stack-1/bundles/bundle-1", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /api/v1/stacks/stack-1/bundles", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"attachments": [{"id":"att-1","organization_id":"org-1","stack_id":"stack-1","bundle_id":"bundle-1",
				"source":"explicit","priority":0,"attached_at":"2026-10-01T10:00:00Z","attached_by":"user-1"}],
			"total": 1,
			"auto_attached": [{"organization_id":"org-1","stack_id":"stack-1","bundle_id":"bundle-2",
				"source":"auto","bundle_slug":"shared-hooks"}],
			"auto_match_count": 1
		}`))
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	client := newTestClient(t, server)
	ctx := context.Background()

	// Attach
	if err := client.AttachBundle(ctx, "stack-1", "bundle-1"); err != nil {
		t.Fatalf("AttachBundle: %v", err)
	}

	// List
	list, err := client.ListStackBundles(ctx, "stack-1")
	if err != nil {
		t.Fatalf("ListStackBundles: %v", err)
	}
	if len(list.Attachments) != 1 || list.Total != 1 {
		t.Fatalf("expected 1 explicit attachment, got %d (total %d)", len(list.Attachments), list.Total)
	}
	explicit := list.Attachments[0]
	if explicit.Source != AttachmentSourceExplicit || explicit.Priority == nil || *explicit.Priority != 0 || explicit.AttachedAt == nil {
		t.Errorf("explicit row = %+v, want source explicit, priority 0 and attached_at", explicit)
	}
	if len(list.AutoAttached) != 1 || list.AutoMatchCount != 1 {
		t.Fatalf("expected 1 auto-attached row, got %d (count %d)", len(list.AutoAttached), list.AutoMatchCount)
	}
	auto := list.AutoAttached[0]
	if auto.Source != AttachmentSourceAuto || auto.BundleSlug != "shared-hooks" || auto.Priority != nil || auto.ID != "" {
		t.Errorf("auto row = %+v, want source auto, slug, no priority and no id", auto)
	}
	if got := list.ExplicitAttachment("bundle-1"); got == nil || got.ID != "att-1" {
		t.Errorf("ExplicitAttachment(bundle-1) = %+v, want att-1", got)
	}
	if got := list.ExplicitAttachment("bundle-2"); got != nil {
		t.Errorf("ExplicitAttachment(bundle-2) = %+v, want nil: an auto-attached bundle has no explicit attachment", got)
	}

	// Detach
	if err := client.DetachBundle(ctx, "stack-1", "bundle-1"); err != nil {
		t.Fatalf("DetachBundle: %v", err)
	}
}

func TestContextCancellation(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(5 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := newTestClient(t, server)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := client.GetSpace(ctx, "space-1")
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
}

// Ensure unused imports don't cause issues.
var _ = fmt.Sprintf

func TestGetStack_DecodesBothTriggerSwitches(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"s1","triggers":{"on_push":{"enabled":false,"paths":["modules/**"]},"on_pull_request":{"enabled":true}}}`))
	}))
	defer server.Close()

	stack, err := newTestClient(t, server).GetStack(context.Background(), "s1")
	if err != nil {
		t.Fatalf("GetStack: %v", err)
	}
	if stack.Triggers.OnPush.Enabled || len(stack.Triggers.OnPush.Paths) != 1 {
		t.Errorf("on_push = %+v, want disabled with one path", stack.Triggers.OnPush)
	}
	if !stack.Triggers.OnPullRequest.Enabled {
		t.Errorf("on_pull_request.enabled = false, want true")
	}
}

func TestGetStack_DecodesThePRCommentSetting(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"s1","pr_comment":{"resource_addresses":"private_or_internal_repos"}}`))
	}))
	defer server.Close()

	stack, err := newTestClient(t, server).GetStack(context.Background(), "s1")
	if err != nil {
		t.Fatalf("GetStack: %v", err)
	}
	if stack.PRComment == nil || stack.PRComment.ResourceAddresses != "private_or_internal_repos" {
		t.Errorf("pr_comment = %+v, want private_or_internal_repos", stack.PRComment)
	}
}

func TestUpdateStackRequest_OmitsAnUnsetPRComment(t *testing.T) {
	t.Parallel()

	name := "renamed"
	raw, err := json.Marshal(UpdateStackRequest{Name: &name})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "pr_comment") {
		t.Errorf("an unset setting must be absent (absent leaves it unchanged): %s", raw)
	}
	off := "off"
	raw, err = json.Marshal(UpdateStackRequest{PRComment: &StackPRCommentRequest{ResourceAddresses: &off}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"pr_comment":{"resource_addresses":"off"}`) {
		t.Errorf("got %s", raw)
	}
}

func TestUpdateBundlePriority_PatchesThePriority(t *testing.T) {
	t.Parallel()

	var gotMethod, gotPath, gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"message":"priority updated successfully"}`))
	}))
	defer server.Close()

	if err := newTestClient(t, server).UpdateBundlePriority(context.Background(), "stack-1", "bundle-1", 4); err != nil {
		t.Fatalf("UpdateBundlePriority: %v", err)
	}
	if gotMethod != http.MethodPatch || gotPath != "/api/v1/stacks/stack-1/bundles/bundle-1" {
		t.Errorf("request = %s %s, want PATCH /api/v1/stacks/stack-1/bundles/bundle-1", gotMethod, gotPath)
	}
	if gotBody != `{"priority":4}` {
		t.Errorf("body = %s, want {\"priority\":4}", gotBody)
	}
}

func TestDetachBundle_AutoAttachedConflictIsRecognised(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		body             string
		wantAutoAttached bool
	}{
		{
			name:             "auto_attached",
			body:             `{"code":"auto_attached","message":"bundle is auto-attached by label"}`,
			wantAutoAttached: true,
		},
		{
			name:             "another conflict",
			body:             `{"code":"version_conflict","message":"bundle version conflict"}`,
			wantAutoAttached: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusConflict)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()

			err := newTestClient(t, server).DetachBundle(context.Background(), "stack-1", "bundle-1")
			if !IsConflict(err) {
				t.Fatalf("err = %v, want a ConflictError", err)
			}
			if got := IsAutoAttached(err); got != tt.wantAutoAttached {
				t.Errorf("IsAutoAttached = %v, want %v (err %v)", got, tt.wantAutoAttached, err)
			}
		})
	}
}

func TestIsAutoAttached_OnlyA409(t *testing.T) {
	t.Parallel()

	notConflict := &ValidationError{APIError: APIError{StatusCode: http.StatusBadRequest, Code: CodeAutoAttached}}
	if IsAutoAttached(notConflict) {
		t.Error("a 400 carrying the code must not count as auto_attached")
	}
	if IsAutoAttached(nil) {
		t.Error("nil must not count as auto_attached")
	}
}

func TestUpdateBundle_AcceptsTheStatusMessageResponse(t *testing.T) {
	t.Parallel()

	var gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"message":"bundle updated successfully"}`))
	}))
	defer server.Close()

	none := []string{}
	if err := newTestClient(t, server).UpdateBundle(context.Background(), "bundle-1", UpdateBundleRequest{AutoAttachLabels: &none}); err != nil {
		t.Fatalf("UpdateBundle: %v", err)
	}
	if gotBody != `{"auto_attach_labels":[]}` {
		t.Errorf("body = %s, want only auto_attach_labels as [] (a clear)", gotBody)
	}
}

func TestLabelRequests_AbsentPreservesAndEmptyClears(t *testing.T) {
	t.Parallel()

	raw, err := json.Marshal(UpdateStackRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "labels") {
		t.Errorf("unset labels must be absent (absent preserves): %s", raw)
	}
	none := []string{}
	raw, err = json.Marshal(UpdateStackRequest{Labels: &none})
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != `{"labels":[]}` {
		t.Errorf("got %s, want {\"labels\":[]}", raw)
	}
	raw, err = json.Marshal(UpdateBundleRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "auto_attach_labels") {
		t.Errorf("unset auto_attach_labels must be absent (absent preserves): %s", raw)
	}
}

func TestBundleContent_HooksNilOmittedEmptyClears(t *testing.T) {
	t.Parallel()

	raw, err := json.Marshal(BundleContent{})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "hooks") {
		t.Errorf("nil hooks must be absent (absent keeps the stored hooks): %s", raw)
	}
	raw, err = json.Marshal(BundleContent{Hooks: &Hooks{}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"hooks":{}`) {
		t.Errorf("empty hooks must marshal as {} (a clear): %s", raw)
	}
	raw, err = json.Marshal(BundleContent{Hooks: &Hooks{BeforePlan: []string{"make lint"}}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"hooks":{"before_plan":["make lint"]}`) {
		t.Errorf("got %s", raw)
	}
}

func TestGetBundle_DecodesHooksAndAutoAttachLabels(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"b1","auto_attach_labels":["prod","eu-west"],
			"hooks":{"before_init":["echo init"],"after_apply":["./notify.sh","echo done"]}}`))
	}))
	defer server.Close()

	bundle, err := newTestClient(t, server).GetBundle(context.Background(), "b1")
	if err != nil {
		t.Fatalf("GetBundle: %v", err)
	}
	if len(bundle.AutoAttachLabels) != 2 || bundle.AutoAttachLabels[1] != "eu-west" {
		t.Errorf("auto_attach_labels = %v", bundle.AutoAttachLabels)
	}
	if bundle.Hooks == nil || len(bundle.Hooks.BeforeInit) != 1 || len(bundle.Hooks.AfterApply) != 2 || bundle.Hooks.BeforePlan != nil {
		t.Errorf("hooks = %+v", bundle.Hooks)
	}
}

func TestGetStack_DecodesLabels(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"s1","labels":["prod","team.payments"]}`))
	}))
	defer server.Close()

	stack, err := newTestClient(t, server).GetStack(context.Background(), "s1")
	if err != nil {
		t.Fatalf("GetStack: %v", err)
	}
	if len(stack.Labels) != 2 || stack.Labels[0] != "prod" || stack.Labels[1] != "team.payments" {
		t.Errorf("labels = %v", stack.Labels)
	}
}
