package domain

import (
	"context"
	"encoding/json"
	"sort"
	"time"
)

// Google連携ジョブの操作種別
type GoogleJobAction string

const (
	GoogleSyncTask     GoogleJobAction = "SYNC_TASK"
	GoogleDeleteTask   GoogleJobAction = "DELETE_TASK"
	GoogleRecreateTask GoogleJobAction = "RECREATE_TASK"
	GooglePullUser     GoogleJobAction = "PULL_USER"
)

// Google連携ジョブ
type GoogleJob struct {
	ID      string          `json:"-"`
	UserID  string          `json:"userId"`
	TaskID  string          `json:"taskId,omitempty"`
	Action  GoogleJobAction `json:"action"`
	Attempt int             `json:"attempt"`
}

// Google同期の衝突記録
type GoogleConflictRecord struct {
	TaskID    string `json:"taskId"`
	Code      string `json:"code"`
	Message   string `json:"message"`
	TaskTitle string `json:"taskTitle"`
	// タスマネ側の現在値（title/notes/due/status）
	Local map[string]string `json:"local"`
	// コンフリクト時点のGoogle側値（title/notes/due/status）
	Remote map[string]string `json:"remote"`
}

// Google取り込み候補
type GoogleImportCandidate struct {
	GoogleTaskListID string          `json:"googleTaskListId"`
	GoogleTaskID     string          `json:"googleTaskId"`
	Snapshot         json.RawMessage `json:"snapshot"`
}

// Google失敗時の対処種別
type GoogleFailureAction string

const (
	GoogleRetry           GoogleFailureAction = "RETRY"
	GoogleRefreshAndRetry GoogleFailureAction = "REFRESH_AND_RETRY"
	GoogleFail            GoogleFailureAction = "FAIL"
	GoogleConflict        GoogleFailureAction = "CONFLICT"
)

// Google失敗の対処分類
func ClassifyGoogleFailure(status int, domainConflict bool) GoogleFailureAction {
	// ドメイン衝突とHTTP状態の分岐
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

// Googleスナップショットのマージ結果
type GoogleMergeResult struct {
	Merged    map[string]string
	Conflicts []string
}

// 3-wayマージによるGoogleスナップショット合成
func MergeGoogleSnapshot(base, local, remote map[string]string) GoogleMergeResult {
	merged := map[string]string{}
	keys := map[string]bool{}
	// 全キー集合の構築
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
	// 同一フィールド衝突の検出と採用
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

// Google期限日とローカル時刻の合成
func MergeGoogleDue(local time.Time, googleDate string) (time.Time, error) {
	// Google日付の解析
	date, err := time.ParseInLocation("2006-01-02", googleDate, local.Location())
	if err != nil {
		return time.Time{}, ErrInvalidDateRange
	}
	return time.Date(date.Year(), date.Month(), date.Day(), local.Hour(), local.Minute(), local.Second(), local.Nanosecond(), local.Location()), nil
}

// リモート削除時の衝突判定
func GoogleRemoteDeleteDecision() GoogleFailureAction { return GoogleConflict }

type googleInboundContextKey struct{}

// Google起点の変更（Googleへの往復同期を抑止）
func WithGoogleInbound(ctx context.Context) context.Context {
	return context.WithValue(ctx, googleInboundContextKey{}, true)
}

// Google起点コンテキストの判定
func IsGoogleInbound(ctx context.Context) bool {
	value, _ := ctx.Value(googleInboundContextKey{}).(bool)
	return value
}

// Google同期コンフリクトの解決方法
type GoogleResolution string

const (
	// タスマネの値でGoogleを上書き（Google側値を採用する場合はUIで先にタスクを更新してからこれを使う）
	GoogleKeepTasmane GoogleResolution = "KEEP_TASMANE"
	// Google側で削除されたタスクをGoogleへ新規作成し再紐付け
	GoogleRecreate GoogleResolution = "RECREATE_GOOGLE"
	// タスマネに残し、以後Google同期の対象外にする
	GoogleUnlink GoogleResolution = "UNLINK"
)

// Google同期に最終失敗したタスク
type GoogleFailedTask struct {
	TaskID  string `json:"taskId"`
	Title   string `json:"title"`
	Message string `json:"message"`
}

// Google連携の同期状態
type GoogleSyncStatus struct {
	Connected          bool               `json:"connected"`
	TaskListMissing    bool               `json:"taskListMissing"`
	LastSyncFinishedAt *time.Time         `json:"lastSyncFinishedAt"`
	ConflictCount      int                `json:"conflictCount"`
	PendingJobs        int                `json:"pendingJobs"`
	Failed             []GoogleFailedTask `json:"failed"`
}

// タスマネ期限からGoogle due（日付のみ）への変換（ユーザーTimezoneでの日付）
func GoogleDueDate(endAt time.Time, location *time.Location) string {
	if location == nil {
		location = time.UTC
	}
	return endAt.In(location).Format("2006-01-02")
}

// Google due（日付のみ）から終日期限（ユーザーTimezoneの当日終わり）への変換
func GoogleAllDayEnd(googleDate string, location *time.Location) (time.Time, error) {
	if location == nil {
		location = time.UTC
	}
	day, err := time.ParseInLocation("2006-01-02", googleDate, location)
	if err != nil {
		return time.Time{}, ErrInvalidDateRange
	}
	return day.Add(24*time.Hour - time.Millisecond).UTC(), nil
}
