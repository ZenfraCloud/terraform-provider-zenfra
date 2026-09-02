// ABOUTME: VCS Repository read methods for the Zenfra API client.
// ABOUTME: Implements ListVCSRepositories (by integration) and GetVCSRepository (by ID).

package zenfraclient

import (
	"context"
	"fmt"
	"net/http"
)

// ListVCSRepositories returns all repositories for a given VCS integration.
func (c *Client) ListVCSRepositories(ctx context.Context, integrationID string) ([]VCSRepository, error) {
	var resp struct {
		Repositories []VCSRepository `json:"repositories"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "/api/v1/vcs/integrations/"+integrationID+"/repos", nil, &resp); err != nil {
		return nil, fmt.Errorf("list vcs repositories: %w", err)
	}
	return resp.Repositories, nil
}

// GetVCSRepository retrieves a single VCS repository by ID.
func (c *Client) GetVCSRepository(ctx context.Context, id string) (*VCSRepository, error) {
	var repo VCSRepository
	if err := c.doJSON(ctx, http.MethodGet, "/api/v1/vcs/repos/"+id, nil, &repo); err != nil {
		return nil, fmt.Errorf("get vcs repository: %w", err)
	}
	return &repo, nil
}
