// ABOUTME: Stack CRUD methods for the Zenfra API client.
// ABOUTME: Implements stack lifecycle plus variables, source, and trigger management endpoints.

package zenfraclient

import (
	"context"
	"fmt"
	"net/http"
)

// CreateStack creates a new stack.
func (c *Client) CreateStack(ctx context.Context, req CreateStackRequest) (*Stack, error) {
	var stack Stack
	if err := c.doJSON(ctx, http.MethodPost, "/api/v1/stacks", req, &stack); err != nil {
		return nil, fmt.Errorf("create stack: %w", err)
	}
	return &stack, nil
}

// GetStack retrieves a stack by ID.
func (c *Client) GetStack(ctx context.Context, id string) (*Stack, error) {
	var stack Stack
	if err := c.doJSON(ctx, http.MethodGet, "/api/v1/stacks/"+id, nil, &stack); err != nil {
		return nil, fmt.Errorf("get stack: %w", err)
	}
	return &stack, nil
}

// ListStacksOptions are optional query parameters for listing stacks.
type ListStacksOptions struct {
	SpaceID *string
	Limit   *int
	Offset  *int
}

// ListStacks returns stacks in the organization, optionally filtered.
func (c *Client) ListStacks(ctx context.Context, opts *ListStacksOptions) ([]Stack, error) {
	path := "/api/v1/stacks"
	sep := "?"
	if opts != nil {
		if opts.SpaceID != nil {
			path += sep + "space_id=" + *opts.SpaceID
			sep = "&"
		}
		if opts.Limit != nil {
			path += sep + fmt.Sprintf("limit=%d", *opts.Limit)
			sep = "&"
		}
		if opts.Offset != nil {
			path += sep + fmt.Sprintf("offset=%d", *opts.Offset)
		}
	}

	var resp struct {
		Items []Stack `json:"items"`
	}
	if err := c.doJSON(ctx, http.MethodGet, path, nil, &resp); err != nil {
		return nil, fmt.Errorf("list stacks: %w", err)
	}
	return resp.Items, nil
}

// UpdateStack updates an existing stack.
func (c *Client) UpdateStack(ctx context.Context, id string, req UpdateStackRequest) (*Stack, error) {
	var stack Stack
	if err := c.doJSON(ctx, http.MethodPut, "/api/v1/stacks/"+id, req, &stack); err != nil {
		return nil, fmt.Errorf("update stack: %w", err)
	}
	return &stack, nil
}

// DeleteStack deletes a stack by ID.
func (c *Client) DeleteStack(ctx context.Context, id string) error {
	resp, err := c.doRequest(ctx, http.MethodDelete, "/api/v1/stacks/"+id, nil)
	if err != nil {
		return fmt.Errorf("delete stack: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // best-effort close
	if err := checkResponse(resp); err != nil {
		return fmt.Errorf("delete stack: %w", err)
	}
	return nil
}

// GetStackVariables retrieves the environment variables for a stack.
// Secret values are returned masked as "****".
func (c *Client) GetStackVariables(ctx context.Context, stackID string) ([]StackVariable, error) {
	var resp GetStackVariablesResponse
	if err := c.doJSON(ctx, http.MethodGet, "/api/v1/stacks/"+stackID+"/variables", nil, &resp); err != nil {
		return nil, fmt.Errorf("get stack variables: %w", err)
	}
	return resp.Variables, nil
}

// SetStackVariables replaces all environment variables on a stack.
// This is a replace-all operation; missing keys are deleted.
func (c *Client) SetStackVariables(ctx context.Context, stackID string, vars []StackVariable) ([]StackVariable, error) {
	req := SetStackVariablesRequest{Variables: vars}
	var resp GetStackVariablesResponse
	if err := c.doJSON(ctx, http.MethodPut, "/api/v1/stacks/"+stackID+"/variables", req, &resp); err != nil {
		return nil, fmt.Errorf("set stack variables: %w", err)
	}
	return resp.Variables, nil
}

// SetStackSource updates the source configuration for a stack.
func (c *Client) SetStackSource(ctx context.Context, stackID string, source StackSource) error {
	if err := c.doJSON(ctx, http.MethodPut, "/api/v1/stacks/"+stackID+"/source", source, nil); err != nil {
		return fmt.Errorf("set stack source: %w", err)
	}
	return nil
}

// SetStackTriggers updates the trigger configuration for a stack.
func (c *Client) SetStackTriggers(ctx context.Context, stackID string, triggers StackTriggers) error {
	if err := c.doJSON(ctx, http.MethodPut, "/api/v1/stacks/"+stackID+"/triggers", triggers, nil); err != nil {
		return fmt.Errorf("set stack triggers: %w", err)
	}
	return nil
}
