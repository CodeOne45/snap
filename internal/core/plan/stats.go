package plan

import (
	"github.com/CodeOne45/snap/internal/core/render"
	"github.com/CodeOne45/snap/internal/core/tokens"
)

// Stats summarizes what a Plan writes: concrete numbers for "Snap wrote
// this instead of the agent" receipts. Computed on demand from an
// already-built Plan — never stored on Plan itself, so building a Plan
// never needs a Counter.
type Stats struct {
	Files        int   `json:"files"`
	TextFiles    int   `json:"text_files"`
	Bytes        int64 `json:"bytes"`
	ApproxTokens int   `json:"approx_tokens"`
}

// ComputeStats summarizes every WriteFileOp and ModifyFileOp in p — both
// write real content to a file, so both count the same way. A binary
// file (per render.IsBinary) counts toward Files and Bytes but never
// TextFiles or ApproxTokens — counting tokens in binary content would be
// meaningless. CreateDirOp and RunHookOp never contribute.
func ComputeStats(p Plan, counter tokens.Counter) Stats {
	var st Stats
	for _, op := range p.Ops {
		var content []byte
		switch o := op.(type) {
		case WriteFileOp:
			content = o.Content
		case ModifyFileOp:
			content = o.Content
		default:
			continue
		}
		st.Files++
		st.Bytes += int64(len(content))
		if !render.IsBinary(content) {
			st.TextFiles++
			st.ApproxTokens += counter.Count(string(content))
		}
	}
	return st
}
