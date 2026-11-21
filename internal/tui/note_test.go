package tui

import (
	"context"
	"os"
	"strings"
	"testing"
	"tque/internal/engine"
)

// editNote presses n and replaces the temporary file as the editor would.
// The command that starts the editor is not run.
func editNote(t *testing.T, m *Model, text string) string {
	t.Helper()
	if c := press(m, "n"); c == nil || !m.busy || m.noteFile == "" {
		t.Fatal("editor not started", m.notice)
	}
	path := m.noteFile
	t.Cleanup(func() { _ = os.Remove(path) })
	if err := os.WriteFile(path, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func exists(path string) bool { _, err := os.Stat(path); return err == nil }

func TestNoteEditedInExternalEditor(t *testing.T) {
	m := testModel(t)
	path := editNote(t, m, "\xEF\xBB\xBF一行目\r\n\t二行目\r\n")
	if press(m, "q") != nil || !m.busy {
		t.Fatal("keys accepted while the editor is open")
	}
	_, c := m.Update(noteMsg{})
	if c == nil {
		t.Fatal("not saved", m.notice)
	}
	m.Update(c())
	if m.doc.Tasks[0].Note != "一行目\n\t二行目" || m.busy || m.noteFile != "" || exists(path) {
		t.Fatal(m.doc.Tasks[0], m.busy, m.noteFile, m.notice)
	}
	m.width, m.height = 80, 24
	if row := m.row(m.rows[0]); !strings.Contains(row, "[note]") {
		t.Fatal(row)
	}
	if s := strings.Join(m.summary(), "\n"); !strings.Contains(s, "備考:あり") {
		t.Fatal(s)
	}
	press(m, "enter")
	detail := strings.Join(m.pageLines(), "\n")
	if !strings.Contains(detail, "備考\n一行目\n    二行目\n") || strings.Contains(detail, "�") {
		t.Fatal(detail)
	}
	press(m, "esc")

	// Closing the editor without a change saves nothing.
	revision := m.doc.Revision
	path = editNote(t, m, "一行目\n\t二行目\n")
	if _, c := m.Update(noteMsg{}); c != nil || m.busy || exists(path) || m.doc.Revision != revision || !strings.Contains(m.notice, "変更なし") {
		t.Fatal(m.notice)
	}

	// An emptied file removes the note.
	editNote(t, m, "\n")
	_, c = m.Update(noteMsg{})
	m.Update(c())
	if m.doc.Tasks[0].Note != "" || strings.Contains(m.row(m.rows[0]), "[note]") {
		t.Fatal(m.doc.Tasks[0])
	}
}

func TestNoteEditFailuresKeepTheText(t *testing.T) {
	m := testModel(t)
	// A conflicting change by another process must not be overwritten.
	path := editNote(t, m, "私の備考")
	other := "別プロセスの備考"
	if _, err := m.service.Execute(context.Background(), nil, engine.Request{Op: "edit", ID: 1, Patch: engine.Patch{Note: &other}}); err != nil {
		t.Fatal(err)
	}
	_, c := m.Update(noteMsg{})
	m.Update(c())
	if m.doc.Tasks[0].Note != other || !m.warning() || !strings.Contains(m.notice, path) || !exists(path) || m.noteFile != "" || m.mode != "list" {
		t.Fatal(m.doc.Tasks[0], m.notice, m.noteFile)
	}
	if strings.Contains(m.notice, "入力案") {
		t.Fatal(m.notice)
	}

	// Text the note cannot hold stays in the file.
	path = editNote(t, m, "bell\a")
	if _, c := m.Update(noteMsg{}); c != nil || m.busy || !m.warning() || !strings.Contains(m.notice, path) || !exists(path) {
		t.Fatal(m.notice)
	}

	// A failed editor discards the unchanged temporary file.
	path = editNote(t, m, other)
	if _, c := m.Update(noteMsg{err: os.ErrNotExist}); c != nil || m.busy || !m.warning() || exists(path) || m.doc.Tasks[0].Note != other {
		t.Fatal(m.notice)
	}
}

func TestEditorCommand(t *testing.T) {
	t.Setenv("EDITOR", "some-editor --wait")
	if c := editorCommand("note.txt"); strings.Join(c.Args, "|") != "some-editor|--wait|note.txt" {
		t.Fatal(c.Args)
	}
	t.Setenv("EDITOR", "")
	if c := editorCommand("note.txt"); len(c.Args) != 2 || c.Args[1] != "note.txt" {
		t.Fatal(c.Args)
	}
}
