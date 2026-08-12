package domain

import (
	"reflect"
	"testing"
	"time"
)

func TestClassifyGoogleFailure(t *testing.T) {
	cases := map[int]GoogleFailureAction{400: GoogleFail, 401: GoogleRefreshAndRetry, 403: GoogleRefreshAndRetry, 429: GoogleRetry, 500: GoogleRetry, 503: GoogleRetry}
	for status, want := range cases {
		if got := ClassifyGoogleFailure(status, false); got != want {
			t.Fatalf("%d got %s want %s", status, got, want)
		}
	}
	if got := ClassifyGoogleFailure(409, true); got != GoogleConflict {
		t.Fatalf("conflict got %s", got)
	}
}

func TestMergeGoogleDuePreservesLocalClock(t *testing.T) {
	local := time.Date(2026, 8, 10, 15, 30, 0, 0, time.FixedZone("JST", 9*3600))
	got, err := MergeGoogleDue(local, "2026-08-20")
	if err != nil || got.Day() != 20 || got.Hour() != 15 || got.Minute() != 30 || got.Location() != local.Location() {
		t.Fatal(got, err)
	}
	if _, err := MergeGoogleDue(local, "bad"); err == nil {
		t.Fatal("invalid due accepted")
	}
}
func TestGoogleDeleteIsAlwaysConflict(t *testing.T) {
	if GoogleRemoteDeleteDecision() != GoogleConflict {
		t.Fatal("remote delete must not delete local task")
	}
}

func TestMergeGoogleSnapshotDetectsSameFieldConflict(t *testing.T) {
	base := map[string]string{"title": "old", "due": "2026-08-10", "status": "needsAction"}
	local := map[string]string{"title": "local", "due": "2026-08-10", "status": "needsAction"}
	remote := map[string]string{"title": "remote", "due": "2026-08-12", "status": "needsAction"}
	result := MergeGoogleSnapshot(base, local, remote)
	if !reflect.DeepEqual(result.Conflicts, []string{"title"}) || result.Merged["due"] != "2026-08-12" {
		t.Fatalf("%+v", result)
	}
	remote["title"] = "old"
	result = MergeGoogleSnapshot(base, local, remote)
	if len(result.Conflicts) != 0 || result.Merged["title"] != "local" || result.Merged["due"] != "2026-08-12" {
		t.Fatalf("%+v", result)
	}
}
