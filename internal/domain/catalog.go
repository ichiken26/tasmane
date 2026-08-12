package domain

import "errors"

var (
	ErrNameRequired  = errors.New("name is required")
	ErrResourceInUse = errors.New("resource is in use")
	ErrConflict      = errors.New("resource conflict")
)

type Genre struct {
	ID     string
	UserID string
	Name   string
}

type TaskType struct {
	ID     string
	UserID string
	Name   string
}
