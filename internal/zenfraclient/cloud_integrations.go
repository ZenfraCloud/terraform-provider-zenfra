// ABOUTME: Cloud Integration read methods for the Zenfra API client.
// ABOUTME: Implements GetCloudIntegration, ListCloudIntegrations, and cloud attachment operations.

package zenfraclient

import (
	"context"
	"fmt"
	"net/http"
)

// GetCloudIntegration retrieves a cloud integration by ID.
func (c *Client) GetCloudIntegration(ctx context.Context, id string) (*CloudIntegration, error) {
	var integration CloudIntegration
	if err := c.doJSON(ctx, http.MethodGet, "/api/v1/cloud/integrations/"+id, nil, &integration); err != nil {
		return nil, fmt.Errorf("get cloud integration: %w", err)
	}
	return &integration, nil
}

// ListCloudIntegrationsOptions are optional query parameters for listing cloud integrations.
type ListCloudIntegrationsOptions struct {
	SpaceID *string
}

// ListCloudIntegrations returns all cloud integrations in the organization, optionally filtered.
func (c *Client) ListCloudIntegrations(ctx context.Context, opts *ListCloudIntegrationsOptions) ([]CloudIntegration, error) {
	path := "/api/v1/cloud/integrations"
	if opts != nil {
		if opts.SpaceID != nil {
			path += "?space_id=" + *opts.SpaceID
		}
	}

	var resp struct {
		Integrations []CloudIntegration `json:"integrations"`
	}
	if err := c.doJSON(ctx, http.MethodGet, path, nil, &resp); err != nil {
		return nil, fmt.Errorf("list cloud integrations: %w", err)
	}
	return resp.Integrations, nil
}

// AttachCloudIntegration attaches a cloud integration to a stack.
func (c *Client) AttachCloudIntegration(ctx context.Context, integrationID string, req AttachCloudIntegrationRequest) (*CloudAttachment, error) {
	var attachment CloudAttachment
	if err := c.doJSON(ctx, http.MethodPost, "/api/v1/cloud/integrations/"+integrationID+"/attachments", req, &attachment); err != nil {
		return nil, fmt.Errorf("attach cloud integration: %w", err)
	}
	return &attachment, nil
}

// ListCloudAttachments returns all attachments for a cloud integration.
func (c *Client) ListCloudAttachments(ctx context.Context, integrationID string) ([]CloudAttachment, error) {
	var resp struct {
		Attachments []CloudAttachment `json:"attachments"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "/api/v1/cloud/integrations/"+integrationID+"/attachments", nil, &resp); err != nil {
		return nil, fmt.Errorf("list cloud attachments: %w", err)
	}
	return resp.Attachments, nil
}

// DetachCloudIntegration removes a cloud integration attachment.
func (c *Client) DetachCloudIntegration(ctx context.Context, integrationID, attachmentID string) error {
	resp, err := c.doRequest(ctx, http.MethodDelete, "/api/v1/cloud/integrations/"+integrationID+"/attachments/"+attachmentID, nil)
	if err != nil {
		return fmt.Errorf("detach cloud integration: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // best-effort close
	if err := checkResponse(resp); err != nil {
		return fmt.Errorf("detach cloud integration: %w", err)
	}
	return nil
}
