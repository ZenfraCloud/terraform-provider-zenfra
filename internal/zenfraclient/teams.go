// ABOUTME: Team, team member and space grant methods for the Zenfra API client (zenfra-cloud#953).
// ABOUTME: Writes need an Admin token and the access-boundary activation, else 409 access_boundaries_inactive.

package zenfraclient

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

const teamsPath = "/api/v1/teams"

func teamPath(id string) string { return teamsPath + "/" + url.PathEscape(id) }

func spaceGrantsPath(spaceID string) string {
	return "/api/v1/spaces/" + url.PathEscape(spaceID) + "/grants"
}

// noRetry returns a copy of c that sends a request once. Creates use it: a
// replay of one the API committed would answer a duplicate 409.
func (c *Client) noRetry() *Client {
	once := *c
	once.retry.maxRetries = 0
	return &once
}

// CreateTeam creates a team.
func (c *Client) CreateTeam(ctx context.Context, req CreateTeamRequest) (*Team, error) {
	var t Team
	if err := c.noRetry().doJSON(ctx, http.MethodPost, teamsPath, req, &t); err != nil {
		return nil, fmt.Errorf("create team: %w", err)
	}
	return &t, nil
}

// GetTeam returns a team by ID.
func (c *Client) GetTeam(ctx context.Context, id string) (*Team, error) {
	var t Team
	if err := c.doJSON(ctx, http.MethodGet, teamPath(id), nil, &t); err != nil {
		return nil, fmt.Errorf("get team: %w", err)
	}
	return &t, nil
}

// UpdateTeam renames a team or changes its description.
func (c *Client) UpdateTeam(ctx context.Context, id string, req UpdateTeamRequest) (*Team, error) {
	var t Team
	if err := c.doJSON(ctx, http.MethodPatch, teamPath(id), req, &t); err != nil {
		return nil, fmt.Errorf("update team: %w", err)
	}
	return &t, nil
}

// DeleteTeam deletes a team with its members and space grants.
func (c *Client) DeleteTeam(ctx context.Context, id string) error {
	if err := c.doJSON(ctx, http.MethodDelete, teamPath(id), nil, nil); err != nil {
		return fmt.Errorf("delete team: %w", err)
	}
	return nil
}

// AddTeamMember adds a user of the organization to a team.
func (c *Client) AddTeamMember(ctx context.Context, teamID, userID string) (*TeamMember, error) {
	var m TeamMember
	if err := c.noRetry().doJSON(ctx, http.MethodPost, teamPath(teamID)+"/members", AddTeamMemberRequest{UserID: userID}, &m); err != nil {
		return nil, fmt.Errorf("add team member: %w", err)
	}
	return &m, nil
}

// ListTeamMembers returns a team's members.
func (c *Client) ListTeamMembers(ctx context.Context, teamID string) ([]TeamMember, error) {
	var list TeamMemberList
	if err := c.doJSON(ctx, http.MethodGet, teamPath(teamID)+"/members", nil, &list); err != nil {
		return nil, fmt.Errorf("list team members: %w", err)
	}
	return list.Members, nil
}

// RemoveTeamMember removes a user from a team.
func (c *Client) RemoveTeamMember(ctx context.Context, teamID, userID string) error {
	if err := c.doJSON(ctx, http.MethodDelete, teamPath(teamID)+"/members/"+url.PathEscape(userID), nil, nil); err != nil {
		return fmt.Errorf("remove team member: %w", err)
	}
	return nil
}

// CreateSpaceGrant gives a team a role on a space.
func (c *Client) CreateSpaceGrant(ctx context.Context, spaceID string, req CreateSpaceGrantRequest) (*SpaceGrant, error) {
	var g SpaceGrant
	if err := c.noRetry().doJSON(ctx, http.MethodPost, spaceGrantsPath(spaceID), req, &g); err != nil {
		return nil, fmt.Errorf("create space grant: %w", err)
	}
	return &g, nil
}

// GetSpaceGrant returns a team's grant on a space.
func (c *Client) GetSpaceGrant(ctx context.Context, spaceID, teamID string) (*SpaceGrant, error) {
	var g SpaceGrant
	if err := c.doJSON(ctx, http.MethodGet, spaceGrantsPath(spaceID)+"/"+url.PathEscape(teamID), nil, &g); err != nil {
		return nil, fmt.Errorf("get space grant: %w", err)
	}
	return &g, nil
}

// UpdateSpaceGrant changes a grant's role.
func (c *Client) UpdateSpaceGrant(ctx context.Context, spaceID, teamID, role string) (*SpaceGrant, error) {
	var g SpaceGrant
	if err := c.doJSON(ctx, http.MethodPatch, spaceGrantsPath(spaceID)+"/"+url.PathEscape(teamID),
		UpdateSpaceGrantRequest{Role: role}, &g); err != nil {
		return nil, fmt.Errorf("update space grant: %w", err)
	}
	return &g, nil
}

// DeleteSpaceGrant removes a team's grant on a space.
func (c *Client) DeleteSpaceGrant(ctx context.Context, spaceID, teamID string) error {
	if err := c.doJSON(ctx, http.MethodDelete, spaceGrantsPath(spaceID)+"/"+url.PathEscape(teamID), nil, nil); err != nil {
		return fmt.Errorf("delete space grant: %w", err)
	}
	return nil
}
