package domain

import (
	"errors"
	"strings"
	"testing"
)

// メモリンクは http(s) の絶対URLのみ。http はローカル開発ホストに限る
func TestNormalizeMemoLink(t *testing.T) {
	url, title, err := NormalizeMemoLink(" https://memo.kokage-studio.com/memo/a%20b?mode=edit ", "  設計メモ ")
	if err != nil || url != "https://memo.kokage-studio.com/memo/a%20b?mode=edit" || title != "設計メモ" {
		t.Fatalf("url=%q title=%q err=%v", url, title, err)
	}
	if _, title, err := NormalizeMemoLink("http://localhost:3001/memo/x", ""); err != nil || title != "メモ" {
		t.Fatalf("localhost http must be allowed with default title: %q %v", title, err)
	}
	for _, invalid := range []string{"", "memo/x", "javascript:alert(1)", "ftp://memo.example/x", "http://memo.kokage-studio.com/x", "https://user:pass@memo.example/x", "https:///x", "https://memo.example/" + strings.Repeat("a", 2100)} {
		if _, _, err := NormalizeMemoLink(invalid, "t"); !errors.Is(err, ErrInvalidMemoLink) {
			t.Fatalf("%q must be rejected, got %v", invalid, err)
		}
	}
	if _, title, _ := NormalizeMemoLink("https://memo.example/x", strings.Repeat("あ", 300)); len([]rune(title)) != 200 {
		t.Fatalf("title must be truncated to 200 runes, got %d", len([]rune(title)))
	}
}

// メモアプリのメモID（memo-yyyymmdd-名前）由来のタイトルは名前だけにし、日付はURLから取り出す
func TestMemoLinkTitleAndDateFromMemoAppID(t *testing.T) {
	const raw = "https://memo.kokage-studio.com/memo/memo-20260520-%E3%83%86%E3%82%B9%E3%83%88-2?mode=edit"
	_, title, err := NormalizeMemoLink(raw, "memo-20260520-テスト-2")
	if err != nil || title != "テスト-2" {
		t.Fatalf("title=%q err=%v", title, err)
	}
	// メモアプリからの保存通知の本来のタイトルはそのまま
	if _, title, _ := NormalizeMemoLink(raw, "5月定例ミーティング議事録"); title != "5月定例ミーティング議事録" {
		t.Fatalf("title=%q", title)
	}
	// 日付だけのIDや不正な日付は名前扱いしない
	if _, title, _ := NormalizeMemoLink(raw, "memo-20261340-x"); title != "memo-20261340-x" {
		t.Fatalf("title=%q", title)
	}
	date := MemoLinkDate(raw)
	if date == nil || date.Format("2006-01-02") != "2026-05-20" {
		t.Fatalf("date=%v", date)
	}
	if MemoLinkDate("https://example.com/notes/abc") != nil {
		t.Fatal("non memo-app URL must not have a date")
	}
}
