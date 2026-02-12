// ABOUTME: Bundle CRUD methods for the Zenfra API client.
// ABOUTME: Implements bundle lifecycle including content updates with optimistic locking.

package zenfraclient

import (
	"context"
	"fmt"
	"net/http"
)

// CreateBundle creates a new configuration bundle.
func (c *Client) CreateBundle(ctx context.Context, req CreateBundleRequest) (*Bundle, error) {
	var bundle Bundle
	if err := c.doJSON(ctx, http.MethodPost, "/api/v1/bundles", req, &bundle); err != nil {
		return nil, fmt.Errorf("create bundle: %w", err)
	}
	return &bundle, nil
}

// GetBundle retrieves a bundle by ID.
func (c *Client) GetBundle(ctx context.Context, id string) (*Bundle, error) {
	var bundle Bundle
	if err := c.doJSON(ctx, http.MethodGet, "/api/v1/bundles/"+id, nil, &bundle); err != nil {
		return nil, fmt.Errorf("get bundle: %w", err)
	}
	return &bundle, nil
}

// ListBundles returns all bundles in the organization.
func (c *Client) ListBundles(ctx context.Context) ([]Bundle, error) {
	var resp struct {
		Bundles []Bundle `json:"bundles"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "/api/v1/bundles", nil, &resp); err != nil {
		return nil, fmt.Errorf("list bundles: %w", err)
	}
	return resp.Bundles, nil
}

// UpdateBundle updates bundle metadata.
func (c *Client) UpdateBundle(ctx context.Context, id string, req UpdateBundleRequest) (*Bundle, error) {
	var bundle Bundle
	if err := c.doJSON(ctx, http.MethodPatch, "/api/v1/bundles/"+id, req, &bundle); err != nil {
		return nil, fmt.Errorf("update bundle: %w", err)
	}
	return &bundle, nil
}

// UpdateBundleContent updates the content (env vars, mounted files) of a bundle.
func (c *Client) UpdateBundleContent(ctx context.Context, id string, req UpdateBundleContentRequest) (*UpdateBundleContentResponse, error) {
	var resp UpdateBundleContentResponse
	if err := c.doJSON(ctx, http.MethodPut, "/api/v1/bundles/"+id+"/content", req, &resp); err != nil {
		return nil, fmt.Errorf("update bundle content: %w", err)
	}
	return &resp, nil
}

// DeleteBundle deletes a bundle by ID.
func (c *Client) DeleteBundle(ctx context.Context, id string) error {
	resp, err := c.doRequest(ctx, http.MethodDelete, "/api/v1/bundles/"+id, nil)
	if err != nil {
		return fmt.Errorf("delete bundle: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // best-effort close
	if err := checkResponse(resp); err != nil {
		return fmt.Errorf("delete bundle: %w", err)
	}
	return nil
}
