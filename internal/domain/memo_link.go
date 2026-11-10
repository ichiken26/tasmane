package domain

import (
	"errors"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// タスクに紐づく外部メモ（メモアプリ等）へのリンク
type MemoLink struct {
	ID     string
	TaskID string
	URL    string
	Title  string
	// メモアプリのメモID（memo-yyyymmdd-名前）に含まれる日付
	MemoDate  *time.Time
	CreatedAt time.Time
}

var ErrInvalidMemoLink = errors.New("invalid memo link")

const (
	maxMemoLinkURL   = 2048
	maxMemoLinkTitle = 200
	defaultMemoTitle = "メモ"
)

// メモリンクの正規化と検証（https の絶対URL。http はローカル開発ホストのみ）
func NormalizeMemoLink(rawURL, title string) (string, string, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" || len(rawURL) > maxMemoLinkURL {
		return "", "", ErrInvalidMemoLink
	}
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.Opaque != "" {
		return "", "", ErrInvalidMemoLink
	}
	switch parsed.Scheme {
	case "https":
	case "http":
		if host := parsed.Hostname(); host != "localhost" && host != "127.0.0.1" {
			return "", "", ErrInvalidMemoLink
		}
	default:
		return "", "", ErrInvalidMemoLink
	}
	title = strings.Join(strings.Fields(title), " ")
	if _, name, ok := splitMemoAppID(title); ok {
		title = name
	}
	if title == "" {
		title = defaultMemoTitle
	}
	if runes := []rune(title); len(runes) > maxMemoLinkTitle {
		title = string(runes[:maxMemoLinkTitle])
	}
	return parsed.String(), title, nil
}

// メモアプリのメモID（memo-yyyymmdd-名前）
var memoAppIDPattern = regexp.MustCompile(`^memo-(\d{8})-(.+)$`)

// メモIDの日付と名前への分解
func splitMemoAppID(value string) (time.Time, string, bool) {
	match := memoAppIDPattern.FindStringSubmatch(value)
	if match == nil {
		return time.Time{}, "", false
	}
	date, err := time.Parse("20060102", match[1])
	if err != nil {
		return time.Time{}, "", false
	}
	return date, match[2], true
}

// メモURL（…/memo/{メモID}）からのメモの日付
func MemoLinkDate(rawURL string) *time.Time {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil
	}
	segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if date, _, ok := splitMemoAppID(segments[len(segments)-1]); ok {
		return &date
	}
	return nil
}
