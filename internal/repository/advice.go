package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"tasmane/internal/domain"
	"time"
)

func (p *Postgres) FindAdvice(ctx context.Context, userID string, genreID *string, hash string) (domain.Advice, error) {
	var a domain.Advice
	var genre sql.NullString
	err := p.database.QueryRowContext(ctx, `SELECT id,user_id,genre_id,content,source_hash,generated_at,expires_at FROM ai_advices WHERE user_id=$1 AND genre_id IS NOT DISTINCT FROM $2 AND source_hash=$3 AND expires_at>now() ORDER BY generated_at DESC LIMIT 1`, userID, genreID, hash).Scan(&a.ID, &a.UserID, &genre, &a.Content, &a.SourceHash, &a.GeneratedAt, &a.ExpiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return a, domain.ErrForbidden
	}
	if err != nil {
		return a, err
	}
	if genre.Valid {
		a.GenreID = &genre.String
	}
	return a, nil
}

func (p *Postgres) LatestAdvice(ctx context.Context, userID string, genreID *string) (domain.Advice, error) {
	var advice domain.Advice
	var genre sql.NullString
	err := p.database.QueryRowContext(ctx, `SELECT id,user_id,genre_id,content,source_hash,generated_at,expires_at FROM ai_advices WHERE user_id=$1 AND genre_id IS NOT DISTINCT FROM $2 AND expires_at>now() ORDER BY generated_at DESC LIMIT 1`, userID, genreID).Scan(&advice.ID, &advice.UserID, &genre, &advice.Content, &advice.SourceHash, &advice.GeneratedAt, &advice.ExpiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return advice, domain.ErrForbidden
	}
	if err != nil {
		return advice, fmt.Errorf("get latest advice: %w", err)
	}
	if genre.Valid {
		advice.GenreID = &genre.String
	}
	return advice, nil
}
func (p *Postgres) SaveAdvice(ctx context.Context, a domain.Advice) (domain.Advice, error) {
	id, err := p.ids.New()
	if err != nil {
		return a, err
	}
	a.ID = id
	_, err = p.database.ExecContext(ctx, `INSERT INTO ai_advices(id,user_id,genre_id,content,source_hash,generated_at,expires_at)VALUES($1,$2,$3,$4,$5,$6,$7)`, a.ID, a.UserID, a.GenreID, a.Content, a.SourceHash, a.GeneratedAt, a.ExpiresAt)
	if err != nil {
		return domain.Advice{}, fmt.Errorf("save advice: %w", err)
	}
	return a, nil
}
func (p *Postgres) DeleteExpiredAdvice(ctx context.Context, now time.Time) (int64, error) {
	result, err := p.database.ExecContext(ctx, `DELETE FROM ai_advices WHERE expires_at<=$1`, now)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
func (p *Postgres) ListAdviceUsers(ctx context.Context) ([]domain.AdviceUser, error) {
	rows, err := p.database.QueryContext(ctx, `SELECT users.id,users.timezone,MAX(advice.generated_at) FROM users LEFT JOIN ai_advices advice ON advice.user_id=users.id AND advice.genre_id IS NULL GROUP BY users.id,users.timezone ORDER BY users.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.AdviceUser{}
	for rows.Next() {
		var user domain.AdviceUser
		var last sql.NullTime
		if err := rows.Scan(&user.UserID, &user.Timezone, &last); err != nil {
			return nil, err
		}
		if last.Valid {
			user.LastGeneratedAt = &last.Time
		}
		out = append(out, user)
	}
	return out, rows.Err()
}
