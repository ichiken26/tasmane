package domain

import "strings"

// Google Tasksの詳細（notes）に追記するメモ欄の見出し
const googleMemoSectionHeader = "── memo(tasmane) ──"

// Google Tasksの詳細の組立（本文＋「・メモ名：URL」の行。詳細は書式なしのテキストでURLは各アプリが自動でリンク化する）
func ComposeGoogleNotes(body string, links []MemoLink) string {
	if len(links) == 0 {
		return body
	}
	var b strings.Builder
	if trimmed := strings.TrimRight(body, " \t\r\n"); trimmed != "" {
		b.WriteString(trimmed)
		b.WriteString("\n\n")
	}
	b.WriteString(googleMemoSectionHeader)
	for _, link := range links {
		title := strings.TrimSpace(link.Title)
		if _, name, ok := splitMemoAppID(title); ok {
			title = name
		}
		if title == "" {
			title = defaultMemoTitle
		}
		b.WriteString("\n・")
		b.WriteString(title)
		b.WriteString("：")
		b.WriteString(link.URL)
	}
	return b.String()
}

// Google Tasksの詳細から本文だけを取り出す（メモ欄はタスマネのメモリンクが正）
func GoogleNotesBody(notes string) string {
	if strings.HasPrefix(notes, googleMemoSectionHeader) {
		return ""
	}
	if index := strings.Index(notes, "\n"+googleMemoSectionHeader); index >= 0 {
		return strings.TrimRight(notes[:index], " \t\r\n")
	}
	return notes
}
