package domain

import "errors"

// カタログドメインのエラー群
var (
	ErrNameRequired  = errors.New("name is required")
	ErrResourceInUse = errors.New("resource is in use")
	ErrConflict      = errors.New("resource conflict")
)

// ジャンル
type Genre struct {
	ID     string
	UserID string
	Name   string
}

// タスク種別
type TaskType struct {
	ID     string
	UserID string
	Name   string
}
