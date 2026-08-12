package application

import (
	"context"
	"errors"
	"testing"
)

type queueStub struct{ jobs []GoogleJob }

func (q *queueStub) Enqueue(_ context.Context, j GoogleJob) error {
	q.jobs = append(q.jobs, j)
	return nil
}

type clientStub struct {
	err     error
	refresh int
}

func (c *clientStub) Execute(context.Context, GoogleJob) error { return c.err }
func (c *clientStub) Refresh(context.Context, string) error    { c.refresh++; return nil }

type syncStoreStub struct{ state string }

func (s *syncStoreStub) MarkSyncing(context.Context, GoogleJob) error {
	s.state = "SYNCING"
	return nil
}
func (s *syncStoreStub) MarkSynced(context.Context, GoogleJob) error { s.state = "SYNCED"; return nil }
func (s *syncStoreStub) MarkFailed(context.Context, GoogleJob, string) error {
	s.state = "FAILED"
	return nil
}
func (s *syncStoreStub) MarkConflict(context.Context, GoogleJob, string) error {
	s.state = "CONFLICT"
	return nil
}
func TestGoogleQueueProducerReturnsWithoutClientCall(t *testing.T) {
	q := &queueStub{}
	if err := (GoogleJobs{Queue: q}).Enqueue(context.Background(), "u", "t", GoogleSyncTask); err != nil || len(q.jobs) != 1 {
		t.Fatal(err, q.jobs)
	}
}
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
			if err := consumer.Consume(context.Background(), GoogleJob{UserID: "u", TaskID: "t"}); err != nil {
				t.Fatal(err)
			}
			if store.state != tc.wantState || len(q.jobs) != tc.wantJobs || client.refresh != tc.refresh {
				t.Fatal(store.state, q.jobs, client.refresh)
			}
		})
	}
}
