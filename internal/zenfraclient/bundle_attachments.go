// ABOUTME: Bundle attachment methods for the Zenfra API client.
// ABOUTME: Implements AttachBundle, DetachBundle, and ListStackBundles for stack-bundle linking.

package zenfraclient

import (
	"context"
	"fmt"
	"net/http"
)

// AttachBundle attaches a bundle to a stack.
func (c *Client) AttachBundle(ctx context.Context, stackID, bundleID string) error {
	req := AttachBundleRequest{BundleID: bundleID}
	if err := c.doJSON(ctx, http.MethodPost, "/api/v1/stacks/"+stackID+"/bundles", req, nil); err != nil {
		return fmt.Errorf("attach bundle: %w", err)
	}
	return nil
}

// DetachBundle detaches a bundle from a stack.
func (c *Client) DetachBundle(ctx context.Context, stackID, bundleID string) error {
	resp, err := c.doRequest(ctx, http.MethodDelete, "/api/v1/stacks/"+stackID+"/bundles/"+bundleID, nil)
	if err != nil {
		return fmt.Errorf("detach bundle: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // best-effort close
	if err := checkResponse(resp); err != nil {
		return fmt.Errorf("detach bundle: %w", err)
	}
	return nil
}

// ListStackBundles returns all bundle attachments for a stack.
func (c *Client) ListStackBundles(ctx context.Context, stackID string) ([]BundleAttachment, error) {
	var resp ListAttachmentsResponse
	if err := c.doJSON(ctx, http.MethodGet, "/api/v1/stacks/"+stackID+"/bundles", nil, &resp); err != nil {
		return nil, fmt.Errorf("list stack bundles: %w", err)
	}
	return resp.Attachments, nil
}
