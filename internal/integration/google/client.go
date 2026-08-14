package google

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"tasmane/internal/application"
	"tasmane/internal/domain"
	"tasmane/internal/repository"
)

const defaultAPIBase = "https://tasks.googleapis.com/tasks/v1"
const defaultTokenURL = "https://oauth2.googleapis.com/token"

type Client struct {
	HTTP         *http.Client
	Store        *repository.Postgres
	Tasks        application.Tasks
	Cipher       TokenCipher
	ClientID     string
	ClientSecret string
	APIBase      string
	TokenURL     string

	mu     sync.Mutex
	tokens map[string]cachedToken
}

type cachedToken struct {
	Value     string
	ExpiresAt time.Time
}
type taskList struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}
type googleTask struct {
	ID, Title, Notes, Due, Status, Updated string
	Deleted                                bool
}

func (t *googleTask) UnmarshalJSON(data []byte) error {
	type wire struct {
		ID      string `json:"id"`
		Title   string `json:"title"`
		Notes   string `json:"notes"`
		Due     string `json:"due"`
		Status  string `json:"status"`
		Updated string `json:"updated"`
		Deleted bool   `json:"deleted"`
	}
	var value wire
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	t.ID, t.Title, t.Notes, t.Due, t.Status, t.Updated, t.Deleted = value.ID, value.Title, value.Notes, value.Due, value.Status, value.Updated, value.Deleted
	return nil
}

func (c *Client) defaults() {
	if c.HTTP == nil {
		c.HTTP = &http.Client{Timeout: 30 * time.Second}
	}
	if c.APIBase == "" {
		c.APIBase = defaultAPIBase
	}
	if c.TokenURL == "" {
		c.TokenURL = defaultTokenURL
	}
	if c.tokens == nil {
		c.tokens = map[string]cachedToken{}
	}
}

func (c *Client) Refresh(ctx context.Context, userID string) error {
	c.defaults()
	integration, err := c.Store.GetGoogleIntegration(ctx, userID)
	if err != nil {
		return err
	}
	refreshToken, err := c.Cipher.Decrypt(integration.EncryptedRefreshToken)
	if err != nil {
		return err
	}
	form := url.Values{"client_id": {c.ClientID}, "client_secret": {c.ClientSecret}, "refresh_token": {refreshToken}, "grant_type": {"refresh_token"}}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := c.HTTP.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return decodeAPIError(response)
	}
	var payload struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&payload); err != nil {
		return err
	}
	if payload.AccessToken == "" {
		return errors.New("google oauth response omitted access_token")
	}
	if payload.ExpiresIn <= 0 {
		payload.ExpiresIn = 3600
	}
	c.mu.Lock()
	c.tokens[userID] = cachedToken{Value: payload.AccessToken, ExpiresAt: time.Now().Add(time.Duration(payload.ExpiresIn)*time.Second - time.Minute)}
	c.mu.Unlock()
	return nil
}

func (c *Client) accessToken(ctx context.Context, userID string) (string, error) {
	c.defaults()
	c.mu.Lock()
	cached := c.tokens[userID]
	c.mu.Unlock()
	if cached.Value != "" && time.Now().Before(cached.ExpiresAt) {
		return cached.Value, nil
	}
	if err := c.Refresh(ctx, userID); err != nil {
		return "", err
	}
	c.mu.Lock()
	cached = c.tokens[userID]
	c.mu.Unlock()
	return cached.Value, nil
}

func (c *Client) Execute(ctx context.Context, job domain.GoogleJob) error {
	if c.Store == nil || c.ClientID == "" || c.ClientSecret == "" {
		return errors.New("google client is not configured")
	}
	token, err := c.accessToken(ctx, job.UserID)
	if err != nil {
		return err
	}
	integration, err := c.Store.GetGoogleIntegration(ctx, job.UserID)
	if err != nil {
		return err
	}
	listID, err := c.ensureDedicatedList(ctx, token, integration)
	if err != nil {
		return err
	}
	switch job.Action {
	case domain.GooglePullUser:
		return c.pull(ctx, token, job.UserID, listID)
	case domain.GoogleSyncTask, domain.GoogleRecreateTask:
		return c.push(ctx, token, job, listID)
	case domain.GoogleDeleteTask:
		return c.delete(ctx, token, job, listID)
	default:
		return fmt.Errorf("unsupported google job action %q", job.Action)
	}
}

func (c *Client) ensureDedicatedList(ctx context.Context, token string, integration repository.GoogleIntegration) (string, error) {
	if integration.TaskListID != nil {
		var existing taskList
		err := c.requestJSON(ctx, token, http.MethodGet, "/users/@me/lists/"+url.PathEscape(*integration.TaskListID), nil, &existing)
		if err != nil {
			return "", fmt.Errorf("dedicated Google Task List unavailable: %w", err)
		}
		return existing.ID, nil
	}
	var created taskList
	if err := c.requestJSON(ctx, token, http.MethodPost, "/users/@me/lists", map[string]string{"title": domain.DedicatedGoogleTaskListName}, &created); err != nil {
		return "", err
	}
	if created.ID == "" {
		return "", errors.New("google created task list without id")
	}
	if err := c.Store.SetGoogleTaskListID(ctx, integration.UserID, created.ID); err != nil {
		return "", err
	}
	return created.ID, nil
}

func (c *Client) push(ctx context.Context, token string, job domain.GoogleJob, listID string) error {
	task, err := c.Tasks.Get(ctx, job.UserID, job.TaskID)
	if err != nil {
		return err
	}
	payload := map[string]any{"title": task.Title, "notes": nullableString(task.Body), "status": "needsAction"}
	if task.Status == domain.Completed {
		payload["status"] = "completed"
	}
	if task.EndAt != nil {
		payload["due"] = task.EndAt.UTC().Format("2006-01-02") + "T00:00:00.000Z"
	}
	sync, err := c.Store.GetGoogleTaskSync(ctx, job.UserID, job.TaskID)
	path, method := "/lists/"+url.PathEscape(listID)+"/tasks", http.MethodPost
	if err == nil && job.Action != domain.GoogleRecreateTask {
		path += "/" + url.PathEscape(sync.GoogleTaskID)
		method = http.MethodPatch
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	var remote googleTask
	if err := c.requestJSON(ctx, token, method, path, payload, &remote); err != nil {
		return err
	}
	snapshot, _ := json.Marshal(snapshotOf(remote))
	if job.Action == domain.GoogleRecreateTask && err == nil {
		_ = c.Store.DisconnectGoogleTask(ctx, job.UserID, job.TaskID)
	}
	return c.Store.SaveGoogleTaskSync(ctx, repository.GoogleTaskSync{UserID: job.UserID, TaskID: job.TaskID, GoogleTaskListID: listID, GoogleTaskID: remote.ID, Snapshot: snapshot})
}

func (c *Client) delete(ctx context.Context, token string, job domain.GoogleJob, listID string) error {
	sync, err := c.Store.GetGoogleTaskSync(ctx, job.UserID, job.TaskID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := c.requestJSON(ctx, token, http.MethodDelete, "/lists/"+url.PathEscape(listID)+"/tasks/"+url.PathEscape(sync.GoogleTaskID), nil, nil); err != nil {
		return err
	}
	return c.Store.DisconnectGoogleTask(ctx, job.UserID, job.TaskID)
}

func (c *Client) pull(ctx context.Context, token, userID, listID string) error {
	pageToken := ""
	for {
		path := "/lists/" + url.PathEscape(listID) + "/tasks?showCompleted=true&showDeleted=true&maxResults=100"
		if pageToken != "" {
			path += "&pageToken=" + url.QueryEscape(pageToken)
		}
		var page struct {
			Items         []googleTask `json:"items"`
			NextPageToken string       `json:"nextPageToken"`
		}
		if err := c.requestJSON(ctx, token, http.MethodGet, path, nil, &page); err != nil {
			return err
		}
		for _, remote := range page.Items {
			if err := c.ingestRemote(ctx, userID, listID, remote); err != nil {
				return err
			}
		}
		if page.NextPageToken == "" {
			return nil
		}
		pageToken = page.NextPageToken
	}
}

func (c *Client) ingestRemote(ctx context.Context, userID, listID string, remote googleTask) error {
	sync, err := c.Store.FindGoogleTaskSync(ctx, userID, listID, remote.ID)
	if errors.Is(err, sql.ErrNoRows) {
		snapshot, _ := json.Marshal(snapshotOf(remote))
		return c.Store.SaveGoogleTaskSync(ctx, repository.GoogleTaskSync{
			UserID: userID, GoogleTaskListID: listID, GoogleTaskID: remote.ID, Snapshot: snapshot,
		})
	}
	if err != nil {
		return err
	}
	if remote.Deleted {
		return c.conflict(ctx, userID, sync.TaskID, "GOOGLE_DELETED", errors.New("Google task deleted; user resolution required"))
	}
	if sync.TaskID == "" {
		return nil
	}
	return c.mergeRemote(ctx, userID, sync, remote)
}

func (c *Client) mergeRemote(ctx context.Context, userID string, sync repository.GoogleTaskSync, remote googleTask) error {
	ctx = repository.WithGoogleInbound(ctx)
	local, err := c.Tasks.Get(ctx, userID, sync.TaskID)
	if err != nil {
		return err
	}
	base := map[string]string{}
	_ = json.Unmarshal(sync.Snapshot, &base)
	remoteSnapshot := snapshotOf(remote)
	localSnapshot := map[string]string{"title": local.Title, "notes": nullableString(local.Body), "status": googleStatus(local.Status)}
	if local.EndAt != nil {
		localSnapshot["due"] = local.EndAt.Format("2006-01-02")
	}
	merged := domain.MergeGoogleSnapshot(base, localSnapshot, remoteSnapshot)
	if len(merged.Conflicts) != 0 {
		return c.conflict(ctx, userID, sync.TaskID, "FIELD_CONFLICT", fmt.Errorf("field conflict: %s", strings.Join(merged.Conflicts, ",")))
	}
	updated := local
	updated.Title = merged.Merged["title"]
	notes := merged.Merged["notes"]
	updated.Body = &notes
	if due := merged.Merged["due"]; due != "" {
		var endAt time.Time
		var err error
		if local.EndAt == nil {
			endAt, err = time.Parse("2006-01-02", due)
		} else {
			endAt, err = domain.MergeGoogleDue(*local.EndAt, due)
		}
		if err != nil {
			return err
		}
		changes, err := c.Store.PlanGoogleDueUpdate(ctx, userID, local.ID, &endAt)
		if err != nil {
			return c.conflict(ctx, userID, sync.TaskID, "DUE_HIERARCHY", err)
		}
		updated.EndAt = &endAt
		if _, err = c.Tasks.Update(ctx, userID, domain.TaskUpdate{Task: updated, DateChanges: changes}, local.Version); err != nil {
			return err
		}
		local.Version++
	} else if local.EndAt != nil {
		changes, err := c.Store.PlanGoogleDueUpdate(ctx, userID, local.ID, nil)
		if err != nil {
			return c.conflict(ctx, userID, sync.TaskID, "DUE_HIERARCHY", err)
		}
		updated.EndAt = nil
		if _, err = c.Tasks.Update(ctx, userID, domain.TaskUpdate{Task: updated, DateChanges: changes}, local.Version); err != nil {
			return err
		}
		local.Version++
	} else if updated.Title != local.Title || nullableString(updated.Body) != nullableString(local.Body) {
		if _, err := c.Tasks.Update(ctx, userID, domain.TaskUpdate{Task: updated}, local.Version); err != nil {
			return err
		}
		local.Version++
	}
	if remote.Status == "completed" && local.Status != domain.Completed {
		if _, err := c.Tasks.GoogleComplete(ctx, userID, local.ID, local.Version); err != nil {
			return c.conflict(ctx, userID, sync.TaskID, "COMPLETE_BLOCKED", err)
		}
	}
	if remote.Status != "completed" && local.Status == domain.Completed {
		if _, err := c.Tasks.GoogleUncomplete(ctx, userID, local.ID, local.Version); err != nil {
			return err
		}
	}
	snapshot, _ := json.Marshal(remoteSnapshot)
	sync.Snapshot = snapshot
	return c.Store.SaveGoogleTaskSync(ctx, sync)
}

func (c *Client) conflict(ctx context.Context, userID, taskID, code string, cause error) error {
	if taskID != "" {
		if err := c.Store.RecordGoogleConflict(ctx, userID, taskID, code, cause.Error()); err != nil {
			return err
		}
	}
	return application.GoogleAPIError{Status: http.StatusConflict, DomainConflict: true, Err: cause}
}

func snapshotOf(task googleTask) map[string]string {
	value := map[string]string{"title": task.Title, "notes": task.Notes, "status": task.Status}
	if len(task.Due) >= 10 {
		value["due"] = task.Due[:10]
	}
	return value
}
func nullableString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
func googleStatus(status domain.Status) string {
	if status == domain.Completed {
		return "completed"
	}
	return "needsAction"
}

func (c *Client) requestJSON(ctx context.Context, token, method, path string, input, output any) error {
	var body io.Reader
	if input != nil {
		encoded, err := json.Marshal(input)
		if err != nil {
			return err
		}
		body = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(c.APIBase, "/")+path, body)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+token)
	if input != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := c.HTTP.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return decodeAPIError(response)
	}
	if output == nil || response.StatusCode == http.StatusNoContent {
		return nil
	}
	return json.NewDecoder(io.LimitReader(response.Body, 4<<20)).Decode(output)
}

func decodeAPIError(response *http.Response) error {
	payload, _ := io.ReadAll(io.LimitReader(response.Body, 64<<10))
	return application.GoogleAPIError{Status: response.StatusCode, Err: fmt.Errorf("Google API %d: %s", response.StatusCode, strings.TrimSpace(string(payload)))}
}
