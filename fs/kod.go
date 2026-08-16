package fs

import (
	"context"
	"errors"
)

// ErrKODChangeCursorInvalid means the provider can no longer resume from the
// supplied durable cursor and the caller must establish a new baseline.
var ErrKODChangeCursorInvalid = errors.New("KOD change cursor is no longer valid")

// KODChangeEvent is the provider-neutral change hint KOD needs from a native
// incremental feed. Stable provider IDs are optional: when unavailable, an
// adapter may supply Path/ParentPath and KOD falls back to path identity.
// Canonical metadata still comes from stock operations/list or operations/stat.
type KODChangeEvent struct {
	RemoteID   string   `json:"remoteId,omitempty"`
	Path       string   `json:"path,omitempty"`
	Removed    bool     `json:"removed"`
	ParentIDs  []string `json:"parentIds,omitempty"`
	ParentPath string   `json:"parentPath,omitempty"`
}

// KODChangeSet is one complete provider change-feed page sequence, ending at a
// durable cursor which can be persisted by KOD and supplied on the next call.
// RootID may be empty for a backend which does not expose a stable scoped-root ID.
type KODChangeSet struct {
	RootID string           `json:"rootId"`
	Cursor string           `json:"cursor"`
	Events []KODChangeEvent `json:"events"`
}

// KODChangeLister is intentionally separate from Features. Only the private KOD
// RC endpoint consumes it, so unsupported backends require no changes at all.
type KODChangeLister interface {
	KODChanges(ctx context.Context, cursor string) (KODChangeSet, error)
}
