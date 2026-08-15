package operations

import (
	"context"
	"errors"
	"fmt"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/rc"
)

func init() {
	rc.Add(rc.Call{
		Path:  "kod/changes",
		Fn:    rcKODChanges,
		Title: "Return durable incremental changes for KOD",
		Help: `This private KOD extension takes:

- fs - an rclone remote, optionally rooted at a subdirectory
- cursor - an opaque cursor returned by a previous call (optional)

With no cursor, the backend establishes a current baseline and returns no events.
With a cursor, it returns changes since that baseline and a new durable cursor.
If the provider invalidated the cursor, resetRequired is true and the returned
cursor is a new baseline; callers must rebuild their authoritative snapshot once.
`,
	})
}

func rcKODChanges(ctx context.Context, in rc.Params) (rc.Params, error) {
	f, err := rc.GetFs(ctx, in)
	if err != nil {
		return nil, err
	}

	cursor, err := in.GetString("cursor")
	if rc.IsErrParamNotFound(err) {
		cursor = ""
	} else if err != nil {
		return nil, err
	}

	lister, ok := f.(fs.KODChangeLister)
	if !ok {
		return nil, fmt.Errorf("%v does not support kod/changes", f)
	}

	changes, err := lister.KODChanges(ctx, cursor)
	resetRequired := false
	if errors.Is(err, fs.ErrKODChangeCursorInvalid) {
		changes, err = lister.KODChanges(ctx, "")
		resetRequired = true
	}
	if err != nil {
		return nil, err
	}
	if changes.Events == nil {
		changes.Events = []fs.KODChangeEvent{}
	}

	return rc.Params{
		"rootId":        changes.RootID,
		"cursor":        changes.Cursor,
		"events":        changes.Events,
		"resetRequired": resetRequired,
	}, nil
}
