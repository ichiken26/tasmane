package httpapi

import (
	"net/http"

	"tasmane/internal/domain"
)

// PATCH入力の既存タスクへの適用
func applyTaskPatch(current domain.Task, input patchTaskRequest) (domain.Task, error) {
	next := current
	if input.Title.Set {
		if input.Title.Value == nil {
			return domain.Task{}, errBadRequest("title cannot be null")
		}
		next.Title = *input.Title.Value
	}
	if input.Body.Set {
		next.Body = input.Body.Value
	}
	if input.GenreID.Set {
		if input.GenreID.Value == nil {
			return domain.Task{}, errBadRequest("genreId cannot be null")
		}
		next.GenreID = *input.GenreID.Value
	}
	if input.TaskTypeID.Set {
		next.TaskTypeID = input.TaskTypeID.Value
	}
	if input.ParentTaskID.Set {
		next.ParentTaskID = input.ParentTaskID.Value
	}
	if input.StartAt.Set {
		next.StartAt = input.StartAt.Value
	}
	if input.EndAt.Set {
		next.EndAt = input.EndAt.Value
	}
	if input.IsAllDay.Set {
		if input.IsAllDay.Value == nil {
			return domain.Task{}, errBadRequest("isAllDay cannot be null")
		}
		next.IsAllDay = *input.IsAllDay.Value
	}
	if input.ManualPriority.Set {
		next.ManualPriority = input.ManualPriority.Value
	}
	return next, nil
}

// 不正リクエストを表すエラー
type badRequestError struct{ message string }

// 不正リクエストエラーの生成
func errBadRequest(message string) error { return badRequestError{message: message} }

// エラーメッセージの返却
func (e badRequestError) Error() string { return e.message }

// 不正リクエストエラーのHTTP変換
func writeBadRequest(w http.ResponseWriter, err error) {
	message := err.Error()
	status := http.StatusBadRequest
	code := "INVALID_REQUEST"
	if message == "relation repository unavailable" {
		status = http.StatusInternalServerError
		code = "INTERNAL_ERROR"
	}
	writeError(w, status, code, message)
}
