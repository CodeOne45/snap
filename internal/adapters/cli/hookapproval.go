package cli

import (
	"encoding/json"
	"fmt"
	"path"
	"sort"

	"github.com/CodeOne45/snap/internal/ports"
)

// hookApprovalsFile is where the CLI remembers that a user already
// confirmed running a specific template version's hooks, so they aren't
// asked again on every subsequent `snap new` of the same template@version.
const hookApprovalsFile = "hook-approvals.json"

type hookApprovals struct {
	fs   ports.FS
	path string
}

func newHookApprovals(fs ports.FS, configDir string) *hookApprovals {
	return &hookApprovals{fs: fs, path: path.Join(configDir, hookApprovalsFile)}
}

func hookApprovalKey(templateName, version string) string {
	return templateName + "@" + version
}

func (a *hookApprovals) IsApproved(key string) bool {
	approved, err := a.load()
	if err != nil {
		return false
	}
	for _, k := range approved {
		if k == key {
			return true
		}
	}
	return false
}

func (a *hookApprovals) Approve(key string) error {
	approved, err := a.load()
	if err != nil {
		approved = nil
	}
	for _, k := range approved {
		if k == key {
			return nil
		}
	}
	approved = append(approved, key)
	sort.Strings(approved)

	data, err := json.MarshalIndent(struct {
		Approved []string `json:"approved"`
	}{approved}, "", "  ")
	if err != nil {
		return fmt.Errorf("hookapprovals: marshal: %w", err)
	}
	if err := a.fs.WriteFile(a.path, data, 0o644); err != nil {
		return fmt.Errorf("hookapprovals: write %s: %w", a.path, err)
	}
	return nil
}

func (a *hookApprovals) load() ([]string, error) {
	data, err := a.fs.ReadFile(a.path)
	if err != nil {
		return nil, err
	}
	var parsed struct {
		Approved []string `json:"approved"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil, fmt.Errorf("hookapprovals: parse %s: %w", a.path, err)
	}
	return parsed.Approved, nil
}
