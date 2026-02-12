// ABOUTME: Worker Pool CRUD methods for the Zenfra API client.
// ABOUTME: CreateWorkerPool returns the write-once api_key only available at creation time.

package zenfraclient

import (
	"context"
	"fmt"
	"net/http"
)

// CreateWorkerPool creates a new worker pool.
// The response includes the api_key which is only returned once at creation.
func (c *Client) CreateWorkerPool(ctx context.Context, req CreateWorkerPoolRequest) (*CreateWorkerPoolResponse, error) {
	var resp CreateWorkerPoolResponse
	if err := c.doJSON(ctx, http.MethodPost, "/api/v1/worker-pools", req, &resp); err != nil {
		return nil, fmt.Errorf("create worker pool: %w", err)
	}
	return &resp, nil
}

// GetWorkerPool retrieves a worker pool by ID.
func (c *Client) GetWorkerPool(ctx context.Context, id string) (*WorkerPool, error) {
	var pool WorkerPool
	if err := c.doJSON(ctx, http.MethodGet, "/api/v1/worker-pools/"+id, nil, &pool); err != nil {
		return nil, fmt.Errorf("get worker pool: %w", err)
	}
	return &pool, nil
}

// ListWorkerPools returns all worker pools in the organization.
func (c *Client) ListWorkerPools(ctx context.Context) ([]WorkerPool, error) {
	var resp struct {
		Pools []WorkerPool `json:"pools"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "/api/v1/worker-pools", nil, &resp); err != nil {
		return nil, fmt.Errorf("list worker pools: %w", err)
	}
	return resp.Pools, nil
}

// UpdateWorkerPool updates an existing worker pool.
func (c *Client) UpdateWorkerPool(ctx context.Context, id string, req UpdateWorkerPoolRequest) (*WorkerPool, error) {
	var pool WorkerPool
	if err := c.doJSON(ctx, http.MethodPatch, "/api/v1/worker-pools/"+id, req, &pool); err != nil {
		return nil, fmt.Errorf("update worker pool: %w", err)
	}
	return &pool, nil
}

// DeleteWorkerPool deletes a worker pool by ID.
func (c *Client) DeleteWorkerPool(ctx context.Context, id string) error {
	resp, err := c.doRequest(ctx, http.MethodDelete, "/api/v1/worker-pools/"+id, nil)
	if err != nil {
		return fmt.Errorf("delete worker pool: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // best-effort close
	if err := checkResponse(resp); err != nil {
		return fmt.Errorf("delete worker pool: %w", err)
	}
	return nil
}
