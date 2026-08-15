package fs

import (
	"context"
	"errors"
)

// ErrKODChangeCursorInvalid means the provider can no longer resume from the
// supplied durable cursor and the caller must establish a new baseline.
var ErrKODChangeCursorInvalid = errors.New("KOD change cursor is no longer valid")

// KODChangeEvent is the provider-neutral identity information KOD needs from a
// native incremental change feed. Path/name/metadata are deliberately omitted:
// KOD obtains those through rclone's canonical operations/list implementation.
type KODChangeEvent struct {
	RemoteID  string   `json:"remoteId"`
	Removed   bool     `json:"removed"`
	ParentIDs []string `json:"parentIds,omitempty"`
}

// KODChangeSet is one complete provider change-feed page sequence, ending at a
// durable cursor which can be persisted by KOD and supplied on the next call.
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
