// ABOUTME: Bundle attachment methods for the Zenfra API client.
// ABOUTME: Implements attach, detach, priority update and the explicit + auto-attached listing.

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

// UpdateBundlePriority sets the priority of an explicit attachment. A bundle
// that reaches the stack only by label answers 409 auto_attached (see
// IsAutoAttached).
func (c *Client) UpdateBundlePriority(ctx context.Context, stackID, bundleID string, priority int) error {
	req := UpdateBundlePriorityRequest{Priority: priority}
	if err := c.doJSON(ctx, http.MethodPatch, "/api/v1/stacks/"+stackID+"/bundles/"+bundleID, req, nil); err != nil {
		return fmt.Errorf("update bundle priority: %w", err)
	}
	return nil
}

// DetachBundle detaches a bundle from a stack. A bundle that reaches the
// stack only by label answers 409 auto_attached (see IsAutoAttached).
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

// ListStackBundles returns a stack's explicit attachments and the bundles
// that auto-attach to it by label.
func (c *Client) ListStackBundles(ctx context.Context, stackID string) (*ListAttachmentsResponse, error) {
	var resp ListAttachmentsResponse
	if err := c.doJSON(ctx, http.MethodGet, "/api/v1/stacks/"+stackID+"/bundles", nil, &resp); err != nil {
		return nil, fmt.Errorf("list stack bundles: %w", err)
	}
	return &resp, nil
}

// ExplicitAttachment returns the explicit attachment of bundleID, or nil when
// the bundle has none (it may still appear in AutoAttached).
func (r *ListAttachmentsResponse) ExplicitAttachment(bundleID string) *BundleAttachment {
	for i := range r.Attachments {
		if r.Attachments[i].BundleID == bundleID {
			return &r.Attachments[i]
		}
	}
	return nil
}
