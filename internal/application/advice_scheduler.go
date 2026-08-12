package application

import (
	"context"
	"tasmane/internal/domain"
	"time"
)

type AdviceUserRepository interface {
	ListAdviceUsers(context.Context) ([]domain.AdviceUser, error)
}
type AdviceScheduler struct {
	Users   AdviceUserRepository
	Tasks   Tasks
	Catalog Catalog
	Advices Advices
	Now     func() time.Time
}

func (s AdviceScheduler) RunOnce(ctx context.Context) error {
	now := time.Now().UTC()
	if s.Now != nil {
		now = s.Now()
	}
	users, err := s.Users.ListAdviceUsers(ctx)
	if err != nil {
		return err
	}
	for _, user := range users {
		if !domain.ShouldGenerateAdvice(now, user) {
			continue
		}
		tasks, err := s.Tasks.Search(ctx, user.UserID, domain.TaskSearch{})
		if err != nil {
			return err
		}
		if _, err := s.Advices.Generate(ctx, user.UserID, nil, tasks); err != nil {
			return err
		}
		genres, err := s.Catalog.ListGenres(ctx, user.UserID)
		if err != nil {
			return err
		}
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
	_, err = s.Advices.Cleanup(ctx, now)
	return err
}
