// ABOUTME: Organization methods for the Zenfra API client.
// ABOUTME: Implements GetCurrentOrganization for reading the authenticated user's organization.

package zenfraclient

import (
	"context"
	"fmt"
	"net/http"
)

// GetCurrentOrganization retrieves the organization for the authenticated user.
func (c *Client) GetCurrentOrganization(ctx context.Context) (*Organization, error) {
	var org Organization
	if err := c.doJSON(ctx, http.MethodGet, "/api/v1/organizations/current", nil, &org); err != nil {
		return nil, fmt.Errorf("get current organization: %w", err)
	}
	return &org, nil
}
