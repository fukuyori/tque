package engine

import (
	"strings"
	"testing"
	"time"
	"tque/internal/model"
)

func TestNoteNormalization(t *testing.T) {
	for in, want := range map[string]string{
		"":                                    "",
		" \n\t ":                              "",
		"\xEF\xBB\xBF一行目\r\n二行目\r\n":          "一行目\n二行目",
		"  字下げ\n\n\tタブ　":                      "  字下げ\n\n\tタブ",
		strings.Repeat("あ", NoteLimit) + "\n": strings.Repeat("あ", NoteLimit),
	} {
		if got, err := Note(in); err != nil || got != want {
			t.Fatalf("%q: %q %v", in, got, err)
		}
	}
	for _, in := range []string{"a\rb", "a\x00b", "a\x1bb", "\xff", strings.Repeat("あ", NoteLimit+1)} {
		if _, err := Note(in); err == nil {
			t.Fatalf("accepted %q", in[:min(len(in), 12)])
		}
	}
}

func TestApplyNote(t *testing.T) {
	now := time.Date(2026, 10, 21, 0, 0, 0, 0, time.UTC)
	d := model.Empty()
	title, note := "task", "memo\r\nsecond line\n"
	if _, _, err := Apply(&d, Request{Op: "add", Patch: Patch{Title: &title, Note: &note}}, now); err != nil {
		t.Fatal(err)
	}
	if d.Tasks[0].Note != "memo\nsecond line" || Validate(d) != nil {
		t.Fatal(d.Tasks[0], Validate(d))
	}
	if _, _, err := Apply(&d, Request{Op: "edit", ID: 1}, now); err == nil {
		t.Fatal("empty patch accepted")
	}
	bad := "bell\a"
	if _, _, err := Apply(&d, Request{Op: "edit", ID: 1, Patch: Patch{Note: &bad}}, now); err == nil {
		t.Fatal("control character accepted")
	}
	empty := ""
	if _, _, err := Apply(&d, Request{Op: "edit", ID: 1, Patch: Patch{Note: &empty}}, now); err != nil || d.Tasks[0].Note != "" {
		t.Fatal(d.Tasks[0], err)
	}
	d.Tasks[0].Note = "stored with trailing newline\n"
	if Validate(d) == nil {
		t.Fatal("unnormalized note accepted")
	}
}
