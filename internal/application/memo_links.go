package application

import (
	"context"

	"tasmane/internal/domain"
)

// メモリンクの永続化境界（タスクの所有者・未削除の確認はリポジトリが行う）
type MemoLinkRepository interface {
	ListMemoLinks(ctx context.Context, userID, taskID string) ([]domain.MemoLink, error)
	UpsertMemoLink(ctx context.Context, userID, taskID, url, title string) (domain.MemoLink, error)
	DeleteMemoLink(ctx context.Context, userID, taskID, linkID string) error
}

// タスクとメモアプリのメモを紐付けるユースケース
type MemoLinks struct{ Repo MemoLinkRepository }

// 一覧
func (u MemoLinks) List(ctx context.Context, userID, taskID string) ([]domain.MemoLink, error) {
	if userID == "" || taskID == "" || u.Repo == nil {
		return nil, domain.ErrForbidden
	}
	return u.Repo.ListMemoLinks(ctx, userID, taskID)
}

// 登録（同じURLは重複させずタイトルを更新）
func (u MemoLinks) Add(ctx context.Context, userID, taskID, rawURL, title string) (domain.MemoLink, error) {
	if userID == "" || taskID == "" || u.Repo == nil {
		return domain.MemoLink{}, domain.ErrForbidden
	}
	url, normalizedTitle, err := domain.NormalizeMemoLink(rawURL, title)
	if err != nil {
		return domain.MemoLink{}, err
	}
	return u.Repo.UpsertMemoLink(ctx, userID, taskID, url, normalizedTitle)
}

// 紐付け解除（メモ自体は削除しない）
func (u MemoLinks) Remove(ctx context.Context, userID, taskID, linkID string) error {
	if userID == "" || taskID == "" || linkID == "" || u.Repo == nil {
		return domain.ErrForbidden
	}
	return u.Repo.DeleteMemoLink(ctx, userID, taskID, linkID)
}

// メモリンクの応答形式
type MemoLinkView struct {
	ID     string `json:"id"`
	TaskID string `json:"taskId"`
	URL    string `json:"url"`
	Title  string `json:"title"`
	// メモの日付（YYYY-MM-DD。メモアプリのメモIDから。不明ならnull）
	MemoDate  *string `json:"memoDate"`
	CreatedAt string  `json:"createdAt"`
}

// 応答形式への変換
func NewMemoLinkView(link domain.MemoLink) MemoLinkView {
	view := MemoLinkView{ID: link.ID, TaskID: link.TaskID, URL: link.URL, Title: link.Title, CreatedAt: link.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00")}
	if link.MemoDate != nil {
		date := link.MemoDate.Format("2006-01-02")
		view.MemoDate = &date
	}
	return view
}
