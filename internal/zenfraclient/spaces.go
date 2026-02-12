// ABOUTME: Space CRUD methods for the Zenfra API client.
// ABOUTME: Implements CreateSpace, GetSpace, ListSpaces, UpdateSpace, DeleteSpace.

package zenfraclient

import (
	"context"
	"fmt"
	"net/http"
)

// CreateSpace creates a new space.
func (c *Client) CreateSpace(ctx context.Context, req CreateSpaceRequest) (*Space, error) {
	var space Space
	if err := c.doJSON(ctx, http.MethodPost, "/api/v1/spaces", req, &space); err != nil {
		return nil, fmt.Errorf("create space: %w", err)
	}
	return &space, nil
}

// GetSpace retrieves a space by ID.
func (c *Client) GetSpace(ctx context.Context, id string) (*Space, error) {
	var space Space
	if err := c.doJSON(ctx, http.MethodGet, "/api/v1/spaces/"+id, nil, &space); err != nil {
		return nil, fmt.Errorf("get space: %w", err)
	}
	return &space, nil
}

// ListSpaces returns all spaces in the organization.
func (c *Client) ListSpaces(ctx context.Context) ([]Space, error) {
	var resp struct {
		Items []Space `json:"items"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "/api/v1/spaces", nil, &resp); err != nil {
		return nil, fmt.Errorf("list spaces: %w", err)
	}
	return resp.Items, nil
}

// UpdateSpace updates an existing space.
func (c *Client) UpdateSpace(ctx context.Context, id string, req UpdateSpaceRequest) (*Space, error) {
	var space Space
	if err := c.doJSON(ctx, http.MethodPut, "/api/v1/spaces/"+id, req, &space); err != nil {
		return nil, fmt.Errorf("update space: %w", err)
	}
	return &space, nil
}

// DeleteSpace deletes a space by ID.
func (c *Client) DeleteSpace(ctx context.Context, id string) error {
	resp, err := c.doRequest(ctx, http.MethodDelete, "/api/v1/spaces/"+id, nil)
	if err != nil {
		return fmt.Errorf("delete space: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // best-effort close
	if err := checkResponse(resp); err != nil {
		return fmt.Errorf("delete space: %w", err)
	}
	return nil
}
