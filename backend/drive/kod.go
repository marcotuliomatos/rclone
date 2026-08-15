package drive

import (
	"context"
	"errors"
	"fmt"

	"github.com/rclone/rclone/fs"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/googleapi"
)

// KODChanges exposes Google Drive's durable Changes page token while leaving
// ChangeNotify and all normal backend behavior untouched.
func (f *Fs) KODChanges(ctx context.Context, cursor string) (out fs.KODChangeSet, err error) {
	rootID, err := f.dirCache.RootID(ctx, false)
	if err != nil {
		return out, fmt.Errorf("failed to resolve KOD root ID: %w", err)
	}
	out.RootID = rootID
	out.Events = []fs.KODChangeEvent{}

	// rclone normally dereferences Google Drive shortcuts and exposes composite
	// IDs which cannot be reconstructed reliably from the Changes feed alone,
	// especially for directory shortcuts. Refuse an ambiguous projection instead
	// of silently corrupting KOD identity.
	if !f.opt.SkipShortcuts {
		return out, fmt.Errorf("kod/changes for Google Drive requires drive skip_shortcuts=true")
	}

	if cursor == "" {
		out.Cursor, err = f.changeNotifyStartPageToken(ctx)
		return out, err
	}

	pageToken := cursor
	for {
		var changeList *drive.ChangeList
		err = f.pacer.Call(func() (bool, error) {
			changesCall := f.svc.Changes.List(pageToken).
				Fields("nextPageToken,newStartPageToken,changes(fileId,removed,file(parents))")
			if f.opt.ListChunk > 0 {
				changesCall.PageSize(f.opt.ListChunk)
			}
			changesCall.SupportsAllDrives(true)
			changesCall.IncludeItemsFromAllDrives(true)
			if f.isTeamDrive {
				changesCall.DriveId(f.opt.TeamDriveID)
			}
			if f.rootFolderID == "appDataFolder" {
				changesCall.Spaces("appDataFolder")
			}
			changesCall.RestrictToMyDrive(!f.opt.SharedWithMe)
			changeList, err = changesCall.Context(ctx).Do()
			var apiErr *googleapi.Error
			if errors.As(err, &apiErr) && apiErr.Code == 410 {
				return false, fs.ErrKODChangeCursorInvalid
			}
			return f.shouldRetry(ctx, err)
		})
		if err != nil {
			return out, err
		}

		for _, change := range changeList.Changes {
			if change.FileId == "" || change.FileId == rootID {
				continue
			}
			event := fs.KODChangeEvent{
				RemoteID: change.FileId,
				Removed:  change.Removed,
			}
			if !change.Removed && change.File != nil {
				event.ParentIDs = append(event.ParentIDs, change.File.Parents...)
			}
			out.Events = append(out.Events, event)
		}

		switch {
		case changeList.NewStartPageToken != "":
			out.Cursor = changeList.NewStartPageToken
			return out, nil
		case changeList.NextPageToken != "":
			pageToken = changeList.NextPageToken
		default:
			return out, fmt.Errorf("Google Drive changes response has no nextPageToken or newStartPageToken")
		}
	}
}
