package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"tque/internal/engine"
)

func runInput(home, input string, args ...string) (string, error) {
	c := New(Options{Version: "test", Home: func() (string, error) { return home, nil }, Now: func() time.Time { return time.Date(2026, 10, 21, 0, 0, 0, 0, time.UTC) }})
	b := &bytes.Buffer{}
	c.SetOut(b)
	c.SetErr(b)
	c.SetIn(strings.NewReader(input))
	c.SetArgs(args)
	e := c.ExecuteContext(context.Background())
	return b.String(), e
}

func TestNoteCommands(t *testing.T) {
	h := t.TempDir()
	exec := func(args ...string) string {
		t.Helper()
		out, err := run(h, args...)
		if err != nil {
			t.Fatal(args, out, err)
		}
		return out
	}
	note := func(id string) string {
		t.Helper()
		var v engine.View
		if err := json.Unmarshal([]byte(exec("show", id, "--json")), &v); err != nil {
			t.Fatal(err)
		}
		return v.Note
	}
	exec("add", "親", "--note", "一行の備考")
	exec("add", "子", "--parent", "1", "--dep", "1", "--due", "today")
	if note("1") != "一行の備考" || note("2") != "" {
		t.Fatal(note("1"), note("2"))
	}
	if out := exec("ls", "--all"); !strings.Contains(out, "1 [ ] 親 [note] [") || strings.Contains(out, "子 [note]") {
		t.Fatal(out)
	}

	file := filepath.Join(t.TempDir(), "note.txt")
	if err := os.WriteFile(file, []byte("\xEF\xBB\xBF一行目\r\n\t二行目\r\n\r\n"), 0600); err != nil {
		t.Fatal(err)
	}
	exec("edit", "2", "--note-file", file)
	if note("2") != "一行目\n\t二行目" {
		t.Fatalf("%q", note("2"))
	}
	want := "2 [ ] 子 [note] [.......... 0%]  -> 2026-10-21 (blocked by #1)\n開始: \n期日: 2026-10-21\n親: #1\n依存: #1\n作成: 2026-10-21T00:00:00Z\n完了: \n\n備考:\n一行目\n\t二行目\n"
	if out := exec("show", "2"); out != want {
		t.Fatalf("%q", out)
	}
	if out, err := runInput(h, "標準入力の備考\n", "edit", "1", "--note-file", "-"); err != nil || note("1") != "標準入力の備考" {
		t.Fatal(out, err)
	}
	var rows []engine.View
	if err := json.Unmarshal([]byte(exec("ls", "--all", "--json")), &rows); err != nil || rows[0].Note != "標準入力の備考" {
		t.Fatal(rows, err)
	}

	exec("edit", "1", "--note", "none")
	if out := exec("show", "1"); note("1") != "" || strings.Contains(out, "備考") || strings.Contains(out, "[note]") {
		t.Fatal(out)
	}
	if out := exec("edit", "1", "--note", ""); !strings.Contains(out, "Unchanged") {
		t.Fatal(out)
	}

	for _, args := range [][]string{
		{"edit", "2", "--note", "a", "--note-file", file},
		{"add", "x", "--note", "none"},
		{"show"},
		{"show", "x"},
	} {
		if out, err := run(h, args...); ExitCode(err) != 2 {
			t.Fatal(args, out, err)
		}
	}
	for _, args := range [][]string{
		{"edit", "2", "--note", "bell\a"},
		{"edit", "2", "--note", strings.Repeat("あ", engine.NoteLimit+1)},
		{"edit", "2", "--note-file", filepath.Join(h, "missing.txt")},
		{"show", "9"},
	} {
		if out, err := run(h, args...); ExitCode(err) != 1 {
			t.Fatal(args, out, err)
		}
	}
	if note("2") != "一行目\n\t二行目" {
		t.Fatal("rejected edit changed the note")
	}
}
