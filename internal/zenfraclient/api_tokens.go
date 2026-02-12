// ABOUTME: API Token CRUD methods for the Zenfra API client.
// ABOUTME: CreateToken returns the write-once token value only available at creation time.

package zenfraclient

import (
	"context"
	"fmt"
	"net/http"
)

// CreateToken creates a new API token.
// The response includes the full token value which is only returned once at creation.
func (c *Client) CreateToken(ctx context.Context, req CreateTokenRequest) (*CreateTokenResponse, error) {
	var resp CreateTokenResponse
	if err := c.doJSON(ctx, http.MethodPost, "/api/v1/tokens", req, &resp); err != nil {
		return nil, fmt.Errorf("create token: %w", err)
	}
	return &resp, nil
}

// GetToken retrieves an API token by ID.
func (c *Client) GetToken(ctx context.Context, id string) (*Token, error) {
	var token Token
	if err := c.doJSON(ctx, http.MethodGet, "/api/v1/tokens/"+id, nil, &token); err != nil {
		return nil, fmt.Errorf("get token: %w", err)
	}
	return &token, nil
}

// ListTokens returns all API tokens in the organization.
func (c *Client) ListTokens(ctx context.Context) ([]Token, error) {
	var tokens []Token
	if err := c.doJSON(ctx, http.MethodGet, "/api/v1/tokens", nil, &tokens); err != nil {
		return nil, fmt.Errorf("list tokens: %w", err)
	}
	return tokens, nil
}

// DeleteToken deletes an API token by ID.
func (c *Client) DeleteToken(ctx context.Context, id string) error {
	resp, err := c.doRequest(ctx, http.MethodDelete, "/api/v1/tokens/"+id, nil)
	if err != nil {
		return fmt.Errorf("delete token: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // best-effort close
	if err := checkResponse(resp); err != nil {
		return fmt.Errorf("delete token: %w", err)
	}
	return nil
}
