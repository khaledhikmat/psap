package data

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/khaledhikmat/psap/model"
)

func TestInitiateLaunchReplacesActiveLaunchAtomically(t *testing.T) {
	ctx := context.Background()
	svc := NewSQLLite(ctx, fileConfig{filepath.Join(t.TempDir(), "assessment.db")})
	if svc == nil {
		t.Fatal("unable to open database")
	}
	defer svc.Finalize()

	owner, manager := "owner-1", "manager-1"
	asset := model.Asset{
		ID: "asset-1", Name: "Asset", Campus: "campus", Building: "building",
		BusinessOwnerID: &owner, BusinessManagerID: &manager, CreatedBy: "test",
	}
	if err := svc.NewAsset(ctx, asset); err != nil {
		t.Fatal(err)
	}
	if err := svc.InitiateLaunch(ctx, model.Launch{ID: "launch-1", AssetID: asset.ID}); err != nil {
		t.Fatal(err)
	}
	if err := svc.InitiateLaunch(ctx, model.Launch{ID: "launch-2", AssetID: asset.ID}); err != nil {
		t.Fatal(err)
	}

	launches, err := svc.RetrieveLaunchesByAssetID(ctx, asset.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(launches) != 2 || launches[0].State != model.CancelledLaunchState || launches[1].State != model.WaitingForAnswersLaunchState {
		t.Fatalf("launches after replacement = %+v", launches)
	}
	updated, err := svc.RetrieveAssetByID(ctx, asset.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.LastLaunchID == nil || *updated.LastLaunchID != "launch-2" {
		t.Fatalf("LastLaunchID = %v, want launch-2", updated.LastLaunchID)
	}
}

func TestInitiateLaunchRollsBackWithoutParticipants(t *testing.T) {
	ctx := context.Background()
	svc := NewSQLLite(ctx, fileConfig{filepath.Join(t.TempDir(), "assessment.db")})
	if svc == nil {
		t.Fatal("unable to open database")
	}
	defer svc.Finalize()
	if err := svc.NewAsset(ctx, model.Asset{
		ID: "asset-1", Name: "Asset", Campus: "campus", Building: "building", CreatedBy: "test",
	}); err != nil {
		t.Fatal(err)
	}
	err := svc.InitiateLaunch(ctx, model.Launch{ID: "launch-1", AssetID: "asset-1"})
	if !errors.Is(err, ErrInvalidState) {
		t.Fatalf("InitiateLaunch error = %v, want ErrInvalidState", err)
	}
	if _, err := svc.RetrieveLaunchByID(ctx, "launch-1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("launch after failed initiation error = %v, want ErrNotFound", err)
	}
}

func TestValidateLaunchAndCompleteShieldUpdateAssetAtomically(t *testing.T) {
	ctx := context.Background()
	svc := NewSQLLite(ctx, fileConfig{filepath.Join(t.TempDir(), "assessment.db")})
	if svc == nil {
		t.Fatal("unable to open database")
	}
	defer svc.Finalize()

	owner, manager := "owner-1", "manager-1"
	asset := model.Asset{
		ID: "asset-1", Name: "Asset", Campus: "campus", Building: "building",
		BusinessOwnerID: &owner, BusinessManagerID: &manager, CreatedBy: "test",
	}
	if err := svc.NewAsset(ctx, asset); err != nil {
		t.Fatal(err)
	}
	if err := svc.NewLaunch(ctx, model.Launch{
		ID: "launch-1", AssetID: asset.ID, State: model.WaitingForTierApprovalLaunchState,
		TierID: ptr("tier-1"), CreatedBy: "test",
	}); err != nil {
		t.Fatal(err)
	}
	if err := svc.ValidateLaunchTier(ctx, "launch-1", "", "test"); err != nil {
		t.Fatal(err)
	}
	validated, err := svc.RetrieveLaunchByID(ctx, "launch-1")
	if err != nil {
		t.Fatal(err)
	}
	if validated.State != model.TierValidatedLaunchState {
		t.Fatalf("launch state = %q", validated.State)
	}
	updatedAsset, err := svc.RetrieveAssetByID(ctx, asset.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updatedAsset.State != model.PendingShieldAssetState || updatedAsset.TierID == nil || *updatedAsset.TierID != "tier-1" {
		t.Fatalf("asset after launch validation = %+v", updatedAsset)
	}

	if err := svc.NewShield(ctx, model.Shield{ID: "shield-old", AssetID: asset.ID, State: model.WaitingForEvidencesShieldState, CreatedBy: "test"}); err != nil {
		t.Fatal(err)
	}
	if err := svc.InitiateShield(ctx, model.Shield{ID: "shield-1", AssetID: asset.ID}); err != nil {
		t.Fatal(err)
	}
	shields, err := svc.RetrieveShieldsByAssetID(ctx, asset.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(shields) != 2 || shields[0].State != model.CancelledShieldState || shields[1].State != model.WaitingForAnswersShieldState {
		t.Fatalf("shields after replacement = %+v", shields)
	}
	updatedAsset, err = svc.RetrieveAssetByID(ctx, asset.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updatedAsset.LastShieldID == nil || *updatedAsset.LastShieldID != "shield-1" {
		t.Fatalf("LastShieldID = %v, want shield-1", updatedAsset.LastShieldID)
	}
	shield := model.Shield{ID: "shield-1", AssetID: asset.ID, State: model.WaitingForEvidencesShieldState, CreatedBy: "test"}
	if err := svc.UpdateShield(ctx, shield); err != nil {
		t.Fatal(err)
	}
	if err := svc.CompleteShield(ctx, "shield-1", 42, "test"); err != nil {
		t.Fatal(err)
	}
	completed, err := svc.RetrieveShieldByID(ctx, "shield-1")
	if err != nil {
		t.Fatal(err)
	}
	if completed.State != model.CompletedShieldState || completed.Score != 42 {
		t.Fatalf("completed shield = %+v", completed)
	}
	updatedAsset, err = svc.RetrieveAssetByID(ctx, asset.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updatedAsset.State != model.ShieldedAssetState {
		t.Fatalf("asset state after Shield completion = %q", updatedAsset.State)
	}
}

func ptr[T any](value T) *T {
	return &value
}
