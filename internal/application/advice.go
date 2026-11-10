package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"tasmane/internal/domain"
	"time"
)

// 助言テキスト生成の外部プロバイダ境界
type AdviceProvider interface {
	Generate(context.Context, string) (string, error)
}

// 助言の永続化境界
type AdviceRepository interface {
	FindAdvice(context.Context, string, *string, string) (domain.Advice, error)
	LatestAdvice(context.Context, string, *string) (domain.Advice, error)
	SaveAdvice(context.Context, domain.Advice) (domain.Advice, error)
	DeleteExpiredAdvice(context.Context, time.Time) (int64, error)
}

// テナント単位の最新助言取得
func (a Advices) Current(ctx context.Context, userID string, genreID *string) (domain.Advice, error) {
	// 未認証ユーザーの拒否
	if userID == "" {
		return domain.Advice{}, domain.ErrForbidden
	}
	// 最新助言の読み出し
	return a.Repo.LatestAdvice(ctx, userID, genreID)
}

// 助言生成・保存のユースケース
type Advices struct {
	Provider AdviceProvider
	Repo     AdviceRepository
	Now      func() time.Time
}

// ソースハッシュ付き助言の生成または再利用
func (a Advices) Generate(ctx context.Context, userID string, genreID *string, source any) (domain.Advice, error) {
	// 未認証ユーザーの拒否
	if userID == "" {
		return domain.Advice{}, domain.ErrForbidden
	}
	// 入力ソースの正規化JSON化
	canonical, err := json.Marshal(source)
	if err != nil {
		return domain.Advice{}, err
	}
	// ソースハッシュの算出
	sum := sha256.Sum256(canonical)
	hash := hex.EncodeToString(sum[:])
	// 同一ハッシュ助言の再利用
	if existing, err := a.Repo.FindAdvice(ctx, userID, genreID, hash); err == nil {
		return existing, nil
	}
	// プロバイダによる助言生成
	text, err := a.Provider.Generate(ctx, "タスク状況から、簡潔で実行可能な日本語の助言を作成してください。\n"+string(canonical))
	if err != nil {
		return domain.Advice{}, err
	}
	// 生成時刻の決定
	now := time.Now().UTC()
	if a.Now != nil {
		now = a.Now()
	}
	// 30日後期限付き助言の保存
	return a.Repo.SaveAdvice(ctx, domain.Advice{UserID: userID, GenreID: genreID, Content: text, SourceHash: hash, GeneratedAt: now, ExpiresAt: now.AddDate(0, 0, 30)})
}

// 期限切れ助言の削除
func (a Advices) Cleanup(ctx context.Context, now time.Time) (int64, error) {
	return a.Repo.DeleteExpiredAdvice(ctx, now)
}

var _ = errors.Is
