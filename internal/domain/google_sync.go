package domain

import (
	"sort"
	"time"
)

type GoogleJobAction string

const (
	GoogleSyncTask     GoogleJobAction = "SYNC_TASK"
	GoogleDeleteTask   GoogleJobAction = "DELETE_TASK"
	GoogleRecreateTask GoogleJobAction = "RECREATE_TASK"
	GooglePullUser     GoogleJobAction = "PULL_USER"
)

type GoogleJob struct {
	UserID  string          `json:"userId"`
	TaskID  string          `json:"taskId,omitempty"`
	Action  GoogleJobAction `json:"action"`
	Attempt int             `json:"attempt"`
}

type GoogleFailureAction string

const (
	GoogleRetry           GoogleFailureAction = "RETRY"
	GoogleRefreshAndRetry GoogleFailureAction = "REFRESH_AND_RETRY"
	GoogleFail            GoogleFailureAction = "FAIL"
	GoogleConflict        GoogleFailureAction = "CONFLICT"
)

func ClassifyGoogleFailure(status int, domainConflict bool) GoogleFailureAction {
	if domainConflict {
		return GoogleConflict
	}
	if status == 401 || status == 403 {
		return GoogleRefreshAndRetry
	}
	if status == 429 || status >= 500 {
		return GoogleRetry
	}
	return GoogleFail
}

type GoogleMergeResult struct {
	Merged    map[string]string
	Conflicts []string
}

func MergeGoogleSnapshot(base, local, remote map[string]string) GoogleMergeResult {
	merged := map[string]string{}
	keys := map[string]bool{}
	for k := range base {
		keys[k] = true
	}
	for k := range local {
		keys[k] = true
	}
	for k := range remote {
		keys[k] = true
	}
	conflicts := []string{}
	for key := range keys {
		b, l, r := base[key], local[key], remote[key]
		lc, rc := l != b, r != b
		if lc && rc && l != r {
			conflicts = append(conflicts, key)
			merged[key] = l
		} else if rc {
			merged[key] = r
		} else {
			merged[key] = l
		}
	}
	sort.Strings(conflicts)
	return GoogleMergeResult{Merged: merged, Conflicts: conflicts}
}

func MergeGoogleDue(local time.Time, googleDate string) (time.Time, error) {
	date, err := time.ParseInLocation("2006-01-02", googleDate, local.Location())
	if err != nil {
		return time.Time{}, ErrInvalidDateRange
	}
	return time.Date(date.Year(), date.Month(), date.Day(), local.Hour(), local.Minute(), local.Second(), local.Nanosecond(), local.Location()), nil
}
func GoogleRemoteDeleteDecision() GoogleFailureAction { return GoogleConflict }
