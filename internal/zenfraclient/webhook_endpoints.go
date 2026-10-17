// ABOUTME: Webhook endpoint CRUD methods for the Zenfra API client (/api/v1/webhook-endpoints, Admin role).
// ABOUTME: CreateWebhookEndpoint returns the signing secret, which no other call returns.

package zenfraclient

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

const webhookEndpointsPath = "/api/v1/webhook-endpoints"

// CreateWebhookEndpoint creates a webhook endpoint. The response carries the
// signing secret, shown only here. It sends at most one POST: the create has
// no idempotency key, so replaying one the API committed but answered with
// an error (or whose response was lost) would make a second endpoint.
func (c *Client) CreateWebhookEndpoint(ctx context.Context, req CreateWebhookEndpointRequest) (*CreateWebhookEndpointResponse, error) {
	once := *c
	once.retry.maxRetries = 0
	var resp CreateWebhookEndpointResponse
	if err := once.doJSON(ctx, http.MethodPost, webhookEndpointsPath, req, &resp); err != nil {
		return nil, fmt.Errorf("create webhook endpoint: %w", err)
	}
	return &resp, nil
}

// GetWebhookEndpoint returns a webhook endpoint by ID, without its secret.
func (c *Client) GetWebhookEndpoint(ctx context.Context, id string) (*WebhookEndpoint, error) {
	var ep WebhookEndpoint
	if err := c.doJSON(ctx, http.MethodGet, webhookEndpointsPath+"/"+url.PathEscape(id), nil, &ep); err != nil {
		return nil, fmt.Errorf("get webhook endpoint: %w", err)
	}
	return &ep, nil
}

// PatchWebhookEndpoint applies a leaf patch and returns the endpoint after it.
func (c *Client) PatchWebhookEndpoint(ctx context.Context, id string, req PatchWebhookEndpointRequest) (*WebhookEndpoint, error) {
	var ep WebhookEndpoint
	if err := c.doJSON(ctx, http.MethodPatch, webhookEndpointsPath+"/"+url.PathEscape(id), req, &ep); err != nil {
		return nil, fmt.Errorf("update webhook endpoint: %w", err)
	}
	return &ep, nil
}

// DeleteWebhookEndpoint deletes a webhook endpoint by ID.
func (c *Client) DeleteWebhookEndpoint(ctx context.Context, id string) error {
	if err := c.doJSON(ctx, http.MethodDelete, webhookEndpointsPath+"/"+url.PathEscape(id), nil, nil); err != nil {
		return fmt.Errorf("delete webhook endpoint: %w", err)
	}
	return nil
}
