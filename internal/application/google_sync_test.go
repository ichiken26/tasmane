package application

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// Googleキューのテスト用スタブ
type queueStub struct{ jobs []GoogleJob }

// 投入ジョブの蓄積
func (q *queueStub) Enqueue(_ context.Context, j GoogleJob) error {
	q.jobs = append(q.jobs, j)
	return nil
}

// Googleクライアントのテスト用スタブ
type clientStub struct {
	err     error
	refresh int
}

// 固定エラーの返却
func (c *clientStub) Execute(context.Context, GoogleJob) error { return c.err }

// Refresh呼出回数の記録
func (c *clientStub) Refresh(context.Context, string) error { c.refresh++; return nil }

// 同期状態ストアのテスト用スタブ
type syncStoreStub struct{ state string }

// 同期中状態への更新
func (s *syncStoreStub) MarkSyncing(context.Context, GoogleJob) error {
	s.state = "SYNCING"
	return nil
}

// 同期完了状態への更新
func (s *syncStoreStub) MarkSynced(context.Context, GoogleJob) error { s.state = "SYNCED"; return nil }

// 失敗状態への更新
func (s *syncStoreStub) MarkFailed(context.Context, GoogleJob, string) error {
	s.state = "FAILED"
	return nil
}

// 競合状態への更新
func (s *syncStoreStub) MarkConflict(context.Context, GoogleJob, string) error {
	s.state = "CONFLICT"
	return nil
}

// プロデューサのクライアント非呼出確認
func TestGoogleQueueProducerReturnsWithoutClientCall(t *testing.T) {
	q := &queueStub{}
	if err := (GoogleJobs{Queue: q}).Enqueue(context.Background(), "u", "t", GoogleSyncTask); err != nil || len(q.jobs) != 1 {
		t.Fatal(err, q.jobs)
	}
}

// 再試行・Refresh・永続失敗の分類確認
func TestGoogleConsumerRetryRefreshAndPermanentFailure(t *testing.T) {
	for _, tc := range []struct {
		name              string
		status            int
		wantState         string
		wantJobs, refresh int
	}{{"retry", 429, "SYNCING", 1, 0}, {"refresh", 401, "SYNCING", 1, 1}, {"fail", 400, "FAILED", 0, 0}} {
		t.Run(tc.name, func(t *testing.T) {
			q := &queueStub{}
			client := &clientStub{err: GoogleAPIError{Status: tc.status, Err: errors.New("api")}}
			store := &syncStoreStub{}
			consumer := GoogleConsumer{Client: client, Store: store, Queue: q}
			// ジョブ消費の実行
			if err := consumer.Consume(context.Background(), GoogleJob{UserID: "u", TaskID: "t"}); err != nil {
				t.Fatal(err)
			}
			// 状態・再投入・Refresh回数の確認
			if store.state != tc.wantState || len(q.jobs) != tc.wantJobs || client.refresh != tc.refresh {
				t.Fatal(store.state, q.jobs, client.refresh)
			}
		})
	}
}

// 失敗記録に失敗するストア（同期行の無い初回送信など）
type failingRecordStore struct{ syncStoreStub }

// 失敗記録の拒否
func (s *failingRecordStore) MarkFailed(context.Context, GoogleJob, string) error {
	return errors.New("no sync row")
}

// 失敗の記録に失敗しても元のGoogleエラーを返す（ワーカーのログで原因が分かること）
func TestGoogleConsumerKeepsOriginalErrorWhenRecordingFails(t *testing.T) {
	client := &clientStub{err: GoogleAPIError{Status: 403, Err: errors.New("Google Tasks API has not been used")}}
	consumer := GoogleConsumer{Client: client, Store: &failingRecordStore{}, Queue: &queueStub{}}
	err := consumer.Consume(context.Background(), GoogleJob{UserID: "u", TaskID: "t", Attempt: 4})
	if err == nil || !strings.Contains(err.Error(), "Google Tasks API has not been used") || !strings.Contains(err.Error(), "no sync row") {
		t.Fatalf("got %v", err)
	}
}
