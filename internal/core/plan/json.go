package plan

import (
	"encoding/json"
	"fmt"
	"io/fs"
)

// opWire is the tagged-union JSON shape for an Op: a "kind" discriminator
// plus whichever fields that kind uses. Adding a new Op kind means adding a
// new field here (and a new case in toWire/fromWire) — existing kinds' JSON
// is unaffected because unused fields are omitted.
type opWire struct {
	Kind           OpKind      `json:"kind"`
	Path           string      `json:"path,omitempty"`
	Content        []byte      `json:"content,omitempty"`
	Mode           fs.FileMode `json:"mode,omitempty"`
	Run            string      `json:"run,omitempty"`
	ExpectedSHA256 string      `json:"expected_sha256,omitempty"`
}

func toWire(op Op) (opWire, error) {
	switch o := op.(type) {
	case CreateDirOp:
		return opWire{Kind: OpCreateDir, Path: o.Path}, nil
	case WriteFileOp:
		return opWire{Kind: OpWriteFile, Path: o.Path, Content: o.Content, Mode: o.Mode}, nil
	case RunHookOp:
		return opWire{Kind: OpRunHook, Run: o.Run}, nil
	case ModifyFileOp:
		return opWire{Kind: OpModifyFile, Path: o.Path, Content: o.Content, Mode: o.Mode, ExpectedSHA256: o.ExpectedSHA256}, nil
	default:
		return opWire{}, fmt.Errorf("plan: marshal: unsupported op type %T", op)
	}
}

func fromWire(w opWire) (Op, error) {
	switch w.Kind {
	case OpCreateDir:
		return CreateDirOp{Path: w.Path}, nil
	case OpWriteFile:
		return WriteFileOp{Path: w.Path, Content: w.Content, Mode: w.Mode}, nil
	case OpRunHook:
		return RunHookOp{Run: w.Run}, nil
	case OpModifyFile:
		return ModifyFileOp{Path: w.Path, Content: w.Content, Mode: w.Mode, ExpectedSHA256: w.ExpectedSHA256}, nil
	default:
		return nil, fmt.Errorf("plan: unmarshal: unknown op kind %q", w.Kind)
	}
}

type planWire struct {
	TargetDir string   `json:"target_dir"`
	Ops       []opWire `json:"ops"`
}

// MarshalJSON implements json.Marshaler, producing the tagged-union wire
// format for Ops.
func (p Plan) MarshalJSON() ([]byte, error) {
	w := planWire{TargetDir: p.TargetDir, Ops: make([]opWire, len(p.Ops))}
	for i, op := range p.Ops {
		ow, err := toWire(op)
		if err != nil {
			return nil, err
		}
		w.Ops[i] = ow
	}
	return json.Marshal(w)
}

// UnmarshalJSON implements json.Unmarshaler, reconstructing concrete Op
// types from the "kind" discriminator.
func (p *Plan) UnmarshalJSON(data []byte) error {
	var w planWire
	if err := json.Unmarshal(data, &w); err != nil {
		return fmt.Errorf("plan: unmarshal: %w", err)
	}
	ops := make([]Op, len(w.Ops))
	for i, ow := range w.Ops {
		op, err := fromWire(ow)
		if err != nil {
			return err
		}
		ops[i] = op
	}
	p.TargetDir = w.TargetDir
	p.Ops = ops
	return nil
}

// ToJSON serializes p as indented JSON, suitable for --json CLI output.
func ToJSON(p Plan) ([]byte, error) {
	return json.MarshalIndent(p, "", "  ")
}

// FromJSON parses indented or compact JSON produced by ToJSON.
func FromJSON(data []byte) (Plan, error) {
	var p Plan
	err := json.Unmarshal(data, &p)
	return p, err
}
