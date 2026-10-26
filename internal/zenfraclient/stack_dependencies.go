// ABOUTME: Stack dependency methods for the Zenfra API client (/api/v1/stacks/:id/dependencies).
// ABOUTME: Every write is addressed through the edge's child stack; there is no single-edge GET.

package zenfraclient

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

func stackDependenciesPath(stackID string) string {
	return "/api/v1/stacks/" + url.PathEscape(stackID) + "/dependencies"
}

// CreateStackDependency creates the enabled edge parent → child. It sends at
// most one POST: a replayed create the API had committed would be refused
// with dependency_exists while nothing reached state.
func (c *Client) CreateStackDependency(ctx context.Context, childID, parentID string) (*StackDependency, error) {
	once := *c
	once.retry.maxRetries = 0
	var edge StackDependency
	req := CreateStackDependencyRequest{ParentStackID: parentID}
	if err := once.doJSON(ctx, http.MethodPost, stackDependenciesPath(childID), req, &edge); err != nil {
		return nil, fmt.Errorf("create stack dependency: %w", err)
	}
	return &edge, nil
}

// ListStackDependencies returns the stack's upstream and downstream edges.
func (c *Client) ListStackDependencies(ctx context.Context, stackID string) (*StackDependencyList, error) {
	var list StackDependencyList
	if err := c.doJSON(ctx, http.MethodGet, stackDependenciesPath(stackID), nil, &list); err != nil {
		return nil, fmt.Errorf("list stack dependencies: %w", err)
	}
	return &list, nil
}

// SetStackDependencyEnabled enables or disables an edge and returns it. A
// same-state PUT changes nothing, so it is safe to retry.
func (c *Client) SetStackDependencyEnabled(ctx context.Context, childID, edgeID string, enabled bool) (*StackDependency, error) {
	var edge StackDependency
	path := stackDependenciesPath(childID) + "/" + url.PathEscape(edgeID) + "/enabled"
	if err := c.doJSON(ctx, http.MethodPut, path, SetStackDependencyEnabledRequest{Enabled: enabled}, &edge); err != nil {
		return nil, fmt.Errorf("set stack dependency enabled: %w", err)
	}
	return &edge, nil
}

// DeleteStackDependency deletes an edge.
func (c *Client) DeleteStackDependency(ctx context.Context, childID, edgeID string) error {
	path := stackDependenciesPath(childID) + "/" + url.PathEscape(edgeID)
	if err := c.doJSON(ctx, http.MethodDelete, path, nil, nil); err != nil {
		return fmt.Errorf("delete stack dependency: %w", err)
	}
	return nil
}
