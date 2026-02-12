// ABOUTME: Unit tests for the zenfra_stack resource model mapping.
// ABOUTME: Verifies correct conversion of nested source, IAC, and trigger types.
package stack

import (
	"context"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/zenfra/terraform-provider-zenfra/internal/zenfraclient"
)

const (
	sourceTypeRawGit = "raw_git"
	sourceTypeVCS    = "vcs"
)

func TestMapStackToState_RawGitSource(t *testing.T) {
	ctx := context.Background()

	// Create a sample API stack with raw_git source
	now := time.Now()
	apiStack := &zenfraclient.Stack{
		ID:              "stack-123",
		OrganizationID:  "org-456",
		SpaceID:         "space-789",
		Name:            "test-stack",
		WorkerPoolID:    strPtr("pool-001"),
		AllowPublicPool: false,
		IAC: zenfraclient.IACConfig{
			Engine:  "terraform",
			Version: "1.6.0",
		},
		Source: zenfraclient.StackSource{
			Type: sourceTypeRawGit,
			RawGit: &zenfraclient.StackSourceRawGit{
				URL: "https://github.com/example/repo.git",
				Ref: zenfraclient.StackSourceRef{
					Type: "branch",
					Name: "main",
				},
				Path: "infra",
			},
		},
		Triggers: zenfraclient.StackTriggers{
			OnPush: zenfraclient.StackTriggerOnPush{
				Enabled: true,
				Paths:   []string{"infra/**", "modules/**"},
			},
		},
		CreatedBy: "user-111",
		UpdatedBy: "user-222",
		CreatedAt: now,
		UpdatedAt: now,
	}

	// Convert to Terraform state model
	model, diags := mapStackToState(ctx, apiStack)
	if diags.HasError() {
		t.Fatalf("mapStackToState returned errors: %v", diags.Errors())
	}

	// Verify basic fields
	if model.ID.ValueString() != "stack-123" {
		t.Errorf("expected ID 'stack-123', got %s", model.ID.ValueString())
	}
	if model.OrganizationID.ValueString() != "org-456" {
		t.Errorf("expected OrganizationID 'org-456', got %s", model.OrganizationID.ValueString())
	}
	if model.SpaceID.ValueString() != "space-789" {
		t.Errorf("expected SpaceID 'space-789', got %s", model.SpaceID.ValueString())
	}
	if model.Name.ValueString() != "test-stack" {
		t.Errorf("expected Name 'test-stack', got %s", model.Name.ValueString())
	}
	if model.WorkerPoolID.ValueString() != "pool-001" {
		t.Errorf("expected WorkerPoolID 'pool-001', got %s", model.WorkerPoolID.ValueString())
	}
	if model.AllowPublicPool.ValueBool() != false {
		t.Errorf("expected AllowPublicPool false, got %v", model.AllowPublicPool.ValueBool())
	}

	// Verify IAC config
	var iacModel IACModel
	diags = model.IAC.As(ctx, &iacModel, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		t.Fatalf("failed to extract IAC model: %v", diags.Errors())
	}
	if iacModel.Engine.ValueString() != "terraform" {
		t.Errorf("expected IAC engine 'terraform', got %s", iacModel.Engine.ValueString())
	}
	if iacModel.Version.ValueString() != "1.6.0" {
		t.Errorf("expected IAC version '1.6.0', got %s", iacModel.Version.ValueString())
	}

	// Verify source
	var sourceModel SourceModel
	diags = model.Source.As(ctx, &sourceModel, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		t.Fatalf("failed to extract source model: %v", diags.Errors())
	}
	if sourceModel.Type.ValueString() != sourceTypeRawGit {
		t.Errorf("expected source type '%s', got %s", sourceTypeRawGit, sourceModel.Type.ValueString())
	}

	// Verify raw_git details
	var rawGitModel RawGitModel
	diags = sourceModel.RawGit.As(ctx, &rawGitModel, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		t.Fatalf("failed to extract raw_git model: %v", diags.Errors())
	}
	if rawGitModel.URL.ValueString() != "https://github.com/example/repo.git" {
		t.Errorf("expected URL 'https://github.com/example/repo.git', got %s", rawGitModel.URL.ValueString())
	}
	if rawGitModel.Path.ValueString() != "infra" {
		t.Errorf("expected path 'infra', got %s", rawGitModel.Path.ValueString())
	}

	// Verify ref
	var refModel RefModel
	diags = rawGitModel.Ref.As(ctx, &refModel, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		t.Fatalf("failed to extract ref model: %v", diags.Errors())
	}
	if refModel.Type.ValueString() != "branch" {
		t.Errorf("expected ref type 'branch', got %s", refModel.Type.ValueString())
	}
	if refModel.Name.ValueString() != "main" {
		t.Errorf("expected ref name 'main', got %s", refModel.Name.ValueString())
	}

	// Verify triggers
	var triggersModel TriggersModel
	diags = model.Triggers.As(ctx, &triggersModel, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		t.Fatalf("failed to extract triggers model: %v", diags.Errors())
	}

	var onPushModel OnPushModel
	diags = triggersModel.OnPush.As(ctx, &onPushModel, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		t.Fatalf("failed to extract on_push model: %v", diags.Errors())
	}
	if onPushModel.Enabled.ValueBool() != true {
		t.Errorf("expected on_push enabled true, got %v", onPushModel.Enabled.ValueBool())
	}

	var paths []string
	diags = onPushModel.Paths.ElementsAs(ctx, &paths, false)
	if diags.HasError() {
		t.Fatalf("failed to extract paths: %v", diags.Errors())
	}
	if len(paths) != 2 {
		t.Fatalf("expected 2 paths, got %d", len(paths))
	}
	if paths[0] != "infra/**" {
		t.Errorf("expected first path 'infra/**', got %s", paths[0])
	}
	if paths[1] != "modules/**" {
		t.Errorf("expected second path 'modules/**', got %s", paths[1])
	}
}

func TestMapStackToState_VCSSource(t *testing.T) {
	ctx := context.Background()

	// Create a sample API stack with vcs source
	now := time.Now()
	apiStack := &zenfraclient.Stack{
		ID:              "stack-456",
		OrganizationID:  "org-789",
		SpaceID:         "space-012",
		Name:            "vcs-stack",
		AllowPublicPool: true,
		IAC: zenfraclient.IACConfig{
			Engine:  "opentofu",
			Version: "1.6.0",
		},
		Source: zenfraclient.StackSource{
			Type: sourceTypeVCS,
			VCS: &zenfraclient.StackSourceVCS{
				Provider:      "github",
				IntegrationID: "int-123",
				RepositoryID:  "repo-456",
				Ref: zenfraclient.StackSourceRef{
					Type: "tag",
					Name: "v1.0.0",
				},
				Path: "terraform",
			},
		},
		Triggers: zenfraclient.StackTriggers{
			OnPush: zenfraclient.StackTriggerOnPush{
				Enabled: false,
				Paths:   []string{},
			},
		},
		CreatedBy: "user-333",
		UpdatedBy: "user-444",
		CreatedAt: now,
		UpdatedAt: now,
	}

	// Convert to Terraform state model
	model, diags := mapStackToState(ctx, apiStack)
	if diags.HasError() {
		t.Fatalf("mapStackToState returned errors: %v", diags.Errors())
	}

	// Verify source
	var sourceModel SourceModel
	diags = model.Source.As(ctx, &sourceModel, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		t.Fatalf("failed to extract source model: %v", diags.Errors())
	}
	if sourceModel.Type.ValueString() != sourceTypeVCS {
		t.Errorf("expected source type '%s', got %s", sourceTypeVCS, sourceModel.Type.ValueString())
	}

	// Verify VCS details
	var vcsModel VCSModel
	diags = sourceModel.VCS.As(ctx, &vcsModel, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		t.Fatalf("failed to extract vcs model: %v", diags.Errors())
	}
	if vcsModel.Provider.ValueString() != "github" {
		t.Errorf("expected provider 'github', got %s", vcsModel.Provider.ValueString())
	}
	if vcsModel.IntegrationID.ValueString() != "int-123" {
		t.Errorf("expected integration_id 'int-123', got %s", vcsModel.IntegrationID.ValueString())
	}
	if vcsModel.RepositoryID.ValueString() != "repo-456" {
		t.Errorf("expected repository_id 'repo-456', got %s", vcsModel.RepositoryID.ValueString())
	}
	if vcsModel.Path.ValueString() != "terraform" {
		t.Errorf("expected path 'terraform', got %s", vcsModel.Path.ValueString())
	}

	// Verify ref
	var refModel RefModel
	diags = vcsModel.Ref.As(ctx, &refModel, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		t.Fatalf("failed to extract ref model: %v", diags.Errors())
	}
	if refModel.Type.ValueString() != "tag" {
		t.Errorf("expected ref type 'tag', got %s", refModel.Type.ValueString())
	}
	if refModel.Name.ValueString() != "v1.0.0" {
		t.Errorf("expected ref name 'v1.0.0', got %s", refModel.Name.ValueString())
	}
}

func TestBuildSourceFromModel_RawGit(t *testing.T) {
	ctx := context.Background()

	// Create a Terraform source model for raw_git
	refObj, _ := types.ObjectValueFrom(ctx, RefModelAttrTypes, &RefModel{
		Type: types.StringValue("branch"),
		Name: types.StringValue("develop"),
	})

	rawGitObj, _ := types.ObjectValueFrom(ctx, RawGitModelAttrTypes, &RawGitModel{
		URL:  types.StringValue("https://github.com/test/repo.git"),
		Ref:  refObj,
		Path: types.StringValue("stacks/prod"),
	})

	sourceObj, _ := types.ObjectValueFrom(ctx, SourceModelAttrTypes, &SourceModel{
		Type:   types.StringValue(sourceTypeRawGit),
		RawGit: rawGitObj,
		VCS:    types.ObjectNull(VCSModelAttrTypes),
	})

	var sourceModel SourceModel
	_ = sourceObj.As(ctx, &sourceModel, basetypes.ObjectAsOptions{})

	// Build API source from model
	source, diags := buildSourceFromModel(ctx, &sourceModel)
	if diags.HasError() {
		t.Fatalf("buildSourceFromModel returned errors: %v", diags.Errors())
	}

	if source.Type != sourceTypeRawGit {
		t.Errorf("expected type '%s', got %s", sourceTypeRawGit, source.Type)
	}
	if source.RawGit == nil {
		t.Fatal("expected RawGit to be non-nil")
	}
	if source.RawGit.URL != "https://github.com/test/repo.git" {
		t.Errorf("expected URL 'https://github.com/test/repo.git', got %s", source.RawGit.URL)
	}
	if source.RawGit.Ref.Type != "branch" {
		t.Errorf("expected ref type 'branch', got %s", source.RawGit.Ref.Type)
	}
	if source.RawGit.Ref.Name != "develop" {
		t.Errorf("expected ref name 'develop', got %s", source.RawGit.Ref.Name)
	}
	if source.RawGit.Path != "stacks/prod" {
		t.Errorf("expected path 'stacks/prod', got %s", source.RawGit.Path)
	}
}

func TestBuildTriggersFromModel(t *testing.T) {
	ctx := context.Background()

	// Create a Terraform triggers model
	paths, _ := types.ListValueFrom(ctx, types.StringType, []string{"src/**", "config/**"})

	onPushObj, _ := types.ObjectValueFrom(ctx, OnPushModelAttrTypes, &OnPushModel{
		Enabled: types.BoolValue(true),
		Paths:   paths,
	})

	triggersObj, _ := types.ObjectValueFrom(ctx, TriggersModelAttrTypes, &TriggersModel{
		OnPush: onPushObj,
	})

	var triggersModel TriggersModel
	_ = triggersObj.As(ctx, &triggersModel, basetypes.ObjectAsOptions{})

	// Build API triggers from model
	triggers, diags := buildTriggersFromModel(ctx, &triggersModel)
	if diags.HasError() {
		t.Fatalf("buildTriggersFromModel returned errors: %v", diags.Errors())
	}

	if triggers.OnPush.Enabled != true {
		t.Errorf("expected on_push enabled true, got %v", triggers.OnPush.Enabled)
	}
	if len(triggers.OnPush.Paths) != 2 {
		t.Fatalf("expected 2 paths, got %d", len(triggers.OnPush.Paths))
	}
	if triggers.OnPush.Paths[0] != "src/**" {
		t.Errorf("expected first path 'src/**', got %s", triggers.OnPush.Paths[0])
	}
	if triggers.OnPush.Paths[1] != "config/**" {
		t.Errorf("expected second path 'config/**', got %s", triggers.OnPush.Paths[1])
	}
}

// Helper function to create string pointers
func strPtr(s string) *string {
	return &s
}
