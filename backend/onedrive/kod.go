package onedrive

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/rclone/rclone/backend/onedrive/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/lib/rest"
)

// KODChanges exposes OneDrive's native Delta cursor without changing the
// existing ChangeNotify contract. The cursor is opaque to KOD; for OneDrive it
// is the complete @odata.deltaLink returned by Microsoft Graph.
func (f *Fs) KODChanges(ctx context.Context, cursor string) (out fs.KODChangeSet, err error) {
	rootID, err := f.dirCache.RootID(ctx, false)
	if err != nil {
		return out, fmt.Errorf("failed to resolve KOD root ID: %w", err)
	}
	out.RootID = rootID
	out.Events = []fs.KODChangeEvent{}

	baseline := cursor == ""
	if baseline {
		cursor = "latest"
	}

	var opts rest.Opts
	if baseline {
		opts = f.buildDriveDeltaOpts(cursor)
	} else {
		// The cursor is an opaque @odata.deltaLink previously returned by Graph.
		opts = rest.Opts{Method: http.MethodGet, RootURL: cursor}
	}
	for {
		var delta api.DeltaResponse
		var resp *http.Response
		err = f.pacer.Call(func() (bool, error) {
			resp, err = f.srv.CallJSON(ctx, &opts, nil, &delta)
			if resp != nil && resp.StatusCode == http.StatusGone {
				return false, fs.ErrKODChangeCursorInvalid
			}
			return shouldRetry(ctx, resp, err)
		})
		if err != nil {
			return out, err
		}

		if !baseline {
			for i := range delta.Value {
				item := &delta.Value[i]
				parentRef := item.GetParentReference()
				remoteID := kodOneDriveID(rootID, parentRef, item.GetID())
				if remoteID == "" || remoteID == rootID {
					continue
				}
				event := fs.KODChangeEvent{
					RemoteID: remoteID,
					Removed:  item.Deleted != nil,
				}
				if item.Deleted == nil && parentRef != nil && parentRef.ID != "" {
					event.ParentIDs = []string{kodOneDriveID(rootID, parentRef, parentRef.ID)}
				}
				out.Events = append(out.Events, event)
			}
		}

		if delta.NextLink != "" {
			// nextLink is produced by Microsoft Graph itself, not supplied by RC.
			opts = rest.Opts{Method: http.MethodGet, RootURL: delta.NextLink}
			continue
		}
		if delta.DeltaLink == "" {
			return out, fmt.Errorf("OneDrive delta response has neither @odata.nextLink nor @odata.deltaLink")
		}
		out.Cursor = delta.DeltaLink
		return out, nil
	}
}

func kodOneDriveID(rootID string, ref *api.ItemReference, itemID string) string {
	if itemID == "" {
		return ""
	}
	if strings.Contains(itemID, "#") {
		return itemID
	}

	driveID := ""
	if ref != nil {
		driveID = ref.DriveID
	}
	if driveID == "" {
		if separator := strings.IndexByte(rootID, '#'); separator >= 0 {
			driveID = rootID[:separator]
		}
	}
	if driveID == "" {
		return itemID
	}
	return driveID + "#" + itemID
}
