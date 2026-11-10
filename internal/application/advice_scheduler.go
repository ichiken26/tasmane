package application

import (
	"context"
	"tasmane/internal/domain"
	"time"
)

// 助言対象ユーザーの永続化境界
type AdviceUserRepository interface {
	ListAdviceUsers(context.Context) ([]domain.AdviceUser, error)
}

// 定期助言生成のスケジューラ
type AdviceScheduler struct {
	Users   AdviceUserRepository
	Tasks   Tasks
	Catalog Catalog
	Advices Advices
	Now     func() time.Time
}

// 対象ユーザーとジャンル単位の助言生成一巡
func (s AdviceScheduler) RunOnce(ctx context.Context) error {
	// 現在時刻の決定
	now := time.Now().UTC()
	if s.Now != nil {
		now = s.Now()
	}
	// 対象ユーザー一覧の取得
	users, err := s.Users.ListAdviceUsers(ctx)
	if err != nil {
		return err
	}
	for _, user := range users {
		// 生成タイミング判定
		if !domain.ShouldGenerateAdvice(now, user) {
			continue
		}
		// 全体タスクからの助言生成
		tasks, err := s.Tasks.Search(ctx, user.UserID, domain.TaskSearch{})
		if err != nil {
			return err
		}
		if _, err := s.Advices.Generate(ctx, user.UserID, nil, tasks); err != nil {
			return err
		}
		// ジャンル一覧の取得
		genres, err := s.Catalog.ListGenres(ctx, user.UserID)
		if err != nil {
			return err
		}
		// ジャンル別タスクからの助言生成
		for _, genre := range genres {
			genreTasks, err := s.Tasks.Search(ctx, user.UserID, domain.TaskSearch{GenreIDs: []string{genre.ID}})
			if err != nil {
				return err
			}
			genreID := genre.ID
			if _, err := s.Advices.Generate(ctx, user.UserID, &genreID, genreTasks); err != nil {
				return err
			}
		}
	}
	// 期限切れ助言の掃除
	_, err = s.Advices.Cleanup(ctx, now)
	return err
}
