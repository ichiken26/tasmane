package domain

import "testing"

// Google Tasksの詳細（notes）へのメモリンク追記と、取込時の本文への復元
func TestGoogleNotesRoundTripWithMemoLinks(t *testing.T) {
	links := []MemoLink{{Title: "議事録", URL: "https://memo.kokage-studio.com/memo/a"}, {Title: "", URL: "https://memo.kokage-studio.com/memo/b"}}
	cases := []struct {
		name, body, want string
		links            []MemoLink
	}{
		{"本文のみ", "買い物", "買い物", nil},
		{"本文とメモ", "買い物", "買い物\n\n── memo(tasmane) ──\n・議事録：https://memo.kokage-studio.com/memo/a\n・メモ：https://memo.kokage-studio.com/memo/b", links},
		{"メモのみ", "", "── memo(tasmane) ──\n・議事録：https://memo.kokage-studio.com/memo/a\n・メモ：https://memo.kokage-studio.com/memo/b", links},
		{"本文末尾の空白", "買い物\n\n", "買い物\n\n── memo(tasmane) ──\n・議事録：https://memo.kokage-studio.com/memo/a\n・メモ：https://memo.kokage-studio.com/memo/b", links},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			notes := ComposeGoogleNotes(tc.body, tc.links)
			if notes != tc.want {
				t.Fatalf("compose:\n%q\nwant\n%q", notes, tc.want)
			}
			wantBody := tc.body
			if tc.links != nil {
				wantBody = "買い物"
				if tc.body == "" {
					wantBody = ""
				}
			}
			if got := GoogleNotesBody(notes); got != wantBody {
				t.Fatalf("body %q want %q", got, wantBody)
			}
		})
	}
	// Google側で本文を編集してもメモ欄は本文に混ざらない
	if got := GoogleNotesBody("書き換えた本文\n\n── memo(tasmane) ──\n・議事録\nhttps://x"); got != "書き換えた本文" {
		t.Fatalf("got %q", got)
	}
	// 見出しを含まない本文はそのまま
	if got := GoogleNotesBody("ただのメモ\n── 区切り ──"); got != "ただのメモ\n── 区切り ──" {
		t.Fatalf("got %q", got)
	}
}

// Google側で空行が消されても見出し行以降はメモ欄として扱う
func TestGoogleNotesBodyWithoutBlankLine(t *testing.T) {
	if got := GoogleNotesBody("本文\n── memo(tasmane) ──\n・a\nhttps://x"); got != "本文" {
		t.Fatalf("got %q", got)
	}
}

// Google Tasksの詳細でもメモ名だけを出す（memo-yyyymmdd- は付けない）
func TestComposeGoogleNotesUsesMemoNameOnly(t *testing.T) {
	notes := ComposeGoogleNotes("", []MemoLink{{Title: "memo-20260520-テスト-2", URL: "https://memo.kokage-studio.com/memo/memo-20260520-x"}})
	want := googleMemoSectionHeader + "\n・テスト-2：https://memo.kokage-studio.com/memo/memo-20260520-x"
	if notes != want {
		t.Fatalf("got %q want %q", notes, want)
	}
}
