// ABOUTME: VCS Integration CRUD methods for the Zenfra API client.
// ABOUTME: Implements lifecycle for GitHub App and GitLab PAT integrations.

package zenfraclient

import (
	"context"
	"fmt"
	"net/http"
)

// CreateVCSIntegration creates a new VCS integration.
func (c *Client) CreateVCSIntegration(ctx context.Context, req CreateVCSIntegrationRequest) (*VCSIntegration, error) {
	var integration VCSIntegration
	if err := c.doJSON(ctx, http.MethodPost, "/api/v1/vcs/integrations", req, &integration); err != nil {
		return nil, fmt.Errorf("create vcs integration: %w", err)
	}
	return &integration, nil
}

// GetVCSIntegration retrieves a VCS integration by ID.
func (c *Client) GetVCSIntegration(ctx context.Context, id string) (*VCSIntegration, error) {
	var integration VCSIntegration
	if err := c.doJSON(ctx, http.MethodGet, "/api/v1/vcs/integrations/"+id, nil, &integration); err != nil {
		return nil, fmt.Errorf("get vcs integration: %w", err)
	}
	return &integration, nil
}

// ListVCSIntegrations returns all VCS integrations in the organization.
func (c *Client) ListVCSIntegrations(ctx context.Context) ([]VCSIntegration, error) {
	var integrations []VCSIntegration
	if err := c.doJSON(ctx, http.MethodGet, "/api/v1/vcs/integrations", nil, &integrations); err != nil {
		return nil, fmt.Errorf("list vcs integrations: %w", err)
	}
	return integrations, nil
}

// UpdateVCSIntegration updates an existing VCS integration.
func (c *Client) UpdateVCSIntegration(ctx context.Context, id string, req UpdateVCSIntegrationRequest) (*VCSIntegration, error) {
	var integration VCSIntegration
	if err := c.doJSON(ctx, http.MethodPatch, "/api/v1/vcs/integrations/"+id, req, &integration); err != nil {
		return nil, fmt.Errorf("update vcs integration: %w", err)
	}
	return &integration, nil
}

// DeleteVCSIntegration deletes a VCS integration by ID.
func (c *Client) DeleteVCSIntegration(ctx context.Context, id string) error {
	resp, err := c.doRequest(ctx, http.MethodDelete, "/api/v1/vcs/integrations/"+id, nil)
	if err != nil {
		return fmt.Errorf("delete vcs integration: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // best-effort close
	if err := checkResponse(resp); err != nil {
		return fmt.Errorf("delete vcs integration: %w", err)
	}
	return nil
}
