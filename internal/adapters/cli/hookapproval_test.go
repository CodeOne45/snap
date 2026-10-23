package cli

import (
	"testing"

	"github.com/CodeOne45/snap/internal/adapters/memfs"
)

func TestHookApprovals_ApproveAndCheck(t *testing.T) {
	fsys := memfs.New()
	if err := fsys.MkdirAll("cfg", 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	a := newHookApprovals(fsys, "cfg")

	key := hookApprovalKey("api-starter", "1.0.0")
	if a.IsApproved(key) {
		t.Fatal("IsApproved before Approve: want false, got true")
	}
	if err := a.Approve(key); err != nil {
		t.Fatalf("Approve: %v", err)
	}
	if !a.IsApproved(key) {
		t.Error("IsApproved after Approve: want true, got false")
	}

	// A fresh hookApprovals reading the same path sees the persisted approval.
	a2 := newHookApprovals(fsys, "cfg")
	if !a2.IsApproved(key) {
		t.Error("a fresh hookApprovals over the same path should see the persisted approval")
	}
}

func TestHookApprovals_DifferentVersionNotApproved(t *testing.T) {
	fsys := memfs.New()
	if err := fsys.MkdirAll("cfg", 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	a := newHookApprovals(fsys, "cfg")
	if err := a.Approve(hookApprovalKey("api-starter", "1.0.0")); err != nil {
		t.Fatalf("Approve: %v", err)
	}
	if a.IsApproved(hookApprovalKey("api-starter", "2.0.0")) {
		t.Error("a different version must not be considered approved")
	}
}

func TestHookApprovals_ApproveIsIdempotent(t *testing.T) {
	fsys := memfs.New()
	if err := fsys.MkdirAll("cfg", 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	a := newHookApprovals(fsys, "cfg")
	key := hookApprovalKey("api-starter", "1.0.0")
	if err := a.Approve(key); err != nil {
		t.Fatalf("Approve: %v", err)
	}
	if err := a.Approve(key); err != nil {
		t.Fatalf("Approve (again): %v", err)
	}
	if !a.IsApproved(key) {
		t.Error("IsApproved: want true")
	}
}
