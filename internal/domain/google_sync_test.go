package domain

import (
	"reflect"
	"testing"
	"time"
)

// Google失敗分類の検証
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

// Google期限日マージ時のローカル時刻保持検証
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

// リモート削除の常時衝突判定検証
func TestGoogleDeleteIsAlwaysConflict(t *testing.T) {
	if GoogleRemoteDeleteDecision() != GoogleConflict {
		t.Fatal("remote delete must not delete local task")
	}
}

// 同一フィールド衝突検出の検証
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

// Googleのdue（日付のみ）はユーザーTimezoneでの期限日
func TestGoogleDueDateUsesUserTimezone(t *testing.T) {
	tokyo, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Skip("tzdata unavailable")
	}
	endAt := time.Date(2026, 8, 24, 15, 30, 0, 0, time.UTC) // 2026-08-25 00:30 JST
	if got := GoogleDueDate(endAt, tokyo); got != "2026-08-25" {
		t.Fatalf("due=%s", got)
	}
	if got := GoogleDueDate(endAt, nil); got != "2026-08-24" {
		t.Fatalf("utc due=%s", got)
	}
	// 新規の終日期限はユーザーTimezoneの当日終わり
	end, err := GoogleAllDayEnd("2026-08-25", tokyo)
	if err != nil || !end.Equal(time.Date(2026, 8, 25, 14, 59, 59, 999000000, time.UTC)) {
		t.Fatalf("end=%v err=%v", end, err)
	}
}
