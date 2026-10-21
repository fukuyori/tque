package tui

import (
	tea "charm.land/bubbletea/v2"
	"context"
	"github.com/charmbracelet/x/ansi"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"tque/internal/engine"
	"tque/internal/model"
	"tque/internal/storage"
)

func testModel(t *testing.T) *Model {
	t.Helper()
	s := engine.Service{Repo: &storage.Store{Dir: t.TempDir()}, Now: func() time.Time { return time.Date(2026, 10, 21, 0, 0, 0, 0, time.UTC) }}
	title := "日本語の親タスク"
	r, e := s.Execute(context.Background(), nil, engine.Request{Op: "add", Patch: engine.Patch{Title: &title}})
	if e != nil {
		t.Fatal(e)
	}
	m := New(s, "D:/データ 保存先", r.Document)
	m.noColor = true
	return m
}
func key(s string) tea.KeyPressMsg {
	codes := map[string]rune{"enter": tea.KeyEnter, "esc": tea.KeyEsc, "space": tea.KeySpace, "left": tea.KeyLeft, "right": tea.KeyRight, "down": tea.KeyDown, "up": tea.KeyUp, "backspace": tea.KeyBackspace}
	if c, ok := codes[s]; ok {
		v := tea.KeyPressMsg{Code: c}
		if s == "space" {
			v.Text = " "
		}
		return v
	}
	if s == "ctrl+c" {
		return tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
	}
	return tea.KeyPressMsg{Code: []rune(s)[0], Text: s}
}
func press(m *Model, s string) tea.Cmd { _, c := m.Update(key(s)); return c }

func TestInputAndSearchModes(t *testing.T) {
	m := testModel(t)
	press(m, "/")
	press(m, "日")
	press(m, "本")
	press(m, "left")
	press(m, "space")
	if m.input.text != "日 本" || m.doc.Tasks[0].Status != "todo" {
		t.Fatal(m.input, m.doc)
	}
	press(m, "esc")
	if m.search != "" || m.mode != "list" {
		t.Fatal(m.search, m.mode)
	}
	press(m, "e")
	m.input.set("変更案")
	press(m, "?")
	if m.mode == "help" || m.input.text != "変更案?" {
		t.Fatal(m.mode, m.input)
	}
	press(m, "esc")
	if m.doc.Tasks[0].Title != "日本語の親タスク" {
		t.Fatal("cancel saved")
	}
	press(m, "enter")
	if m.mode != "detail" {
		t.Fatal(m.mode)
	}
	press(m, "esc")
	press(m, "A")
	if m.mode != "child" {
		t.Fatal(m.mode)
	}
}
func TestSaveConflictAndUncertainty(t *testing.T) {
	m := testModel(t)
	press(m, "e")
	m.input.set("私の入力")
	title := "別プロセスの変更"
	_, e := m.service.Execute(context.Background(), nil, engine.Request{Op: "edit", ID: 1, Patch: engine.Patch{Title: &title}})
	if e != nil {
		t.Fatal(e)
	}
	c := press(m, "enter")
	if c == nil {
		t.Fatal("missing save command")
	}
	m.Update(c())
	if m.mode != "title" || m.input.text != "私の入力" || m.doc.Tasks[0].Title != title {
		t.Fatal(m.mode, m.input, m.doc)
	}
	if !strings.Contains(m.notice, "RevisionConflict") {
		t.Fatal(m.notice)
	}
	c = press(m, "enter")
	if c == nil {
		t.Fatal("missing reconfirmation")
	}
	m.Update(c())
	if m.doc.Tasks[0].Title != "私の入力" {
		t.Fatal(m.doc)
	}
	m.Update(resultMsg{err: model.Err("CommitUncertain", "test")})
	if !m.uncertain {
		t.Fatal("uncertain not latched")
	}
	if c := press(m, "space"); c != nil {
		t.Fatal("uncertain state resubmitted update")
	}
	c = press(m, "r")
	m.Update(c())
	if m.uncertain {
		t.Fatal("read did not recover")
	}
}
func TestSavingExitAndDeleteCancel(t *testing.T) {
	m := testModel(t)
	c := press(m, "space")
	if !m.busy || c == nil {
		t.Fatal("not saving")
	}
	if press(m, "space") != nil {
		t.Fatal("queued duplicate")
	}
	press(m, "ctrl+c")
	_, quit := m.Update(c())
	if quit == nil || m.doc.Tasks[0].Status != "done" {
		t.Fatal("did not save before exit")
	}
	m.exitPending = false
	press(m, "x")
	press(m, "?")
	press(m, "esc")
	if m.mode != "delete" {
		t.Fatal(m.mode)
	}
	if press(m, "enter") != nil || m.mode != "list" || len(m.doc.Tasks) != 1 {
		t.Fatal("Enter deleted")
	}
	m.busy = true
	m.exitPending = true
	m.Update(resultMsg{err: model.Err("StorageFailure", "failure")})
	if m.exitPending || m.busy {
		t.Fatal("error exited")
	}
}
func TestRenderingSizesAndLongContent(t *testing.T) {
	m := testModel(t)
	title := strings.Repeat("日本語", 60) + "👨‍👩‍👧"
	m.doc.Tasks[0].Title = title
	m.refresh()
	for _, size := range [][2]int{{80, 24}, {120, 30}, {60, 14}, {40, 10}, {20, 5}} {
		m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		for _, mode := range []string{"list", "detail", "help", "title"} {
			m.mode = mode
			m.input.set(title)
			view := m.View()
			lines := strings.Split(view.Content, "\n")
			if len(lines) > size[1] {
				t.Fatalf("%v %s height %d", size, mode, len(lines))
			}
			for _, line := range lines {
				if ansi.StringWidth(line) > size[0] {
					t.Fatalf("%v %s width %d", size, mode, ansi.StringWidth(line))
				}
			}
			if size[0] >= 40 && mode == "title" && !strings.Contains(view.Content, "|") {
				t.Fatal("input cursor hidden", size)
			}
		}
	}
	m.width = 80
	m.height = 24
	m.mode = "detail"
	if !strings.Contains(strings.Join(m.pageLines(), ""), strings.ReplaceAll(title, "\n", "")) {
		t.Fatal("detail dropped title")
	}
}
func TestEditorGraphemes(t *testing.T) {
	e := editor{}
	e.set("日👨‍👩‍👧é")
	e.key("backspace")
	if e.text != "日👨‍👩‍👧" {
		t.Fatal(e.text)
	}
	e.key("left")
	e.key("delete")
	if e.text != "日" {
		t.Fatal(e.text)
	}
}

func TestScreenArtifact(t *testing.T) {
	path := os.Getenv("TQUE_SCREEN_ARTIFACT")
	if path == "" {
		t.Skip("opt-in render artifact")
	}
	m := testModel(t)
	parent := int64(1)
	for i, title := range []string{"設計書を確定", "保存処理を実装", "結合テスト"} {
		progress := []string{"100", "50", "0"}[i]
		r, e := m.service.Execute(context.Background(), nil, engine.Request{Op: "add", Patch: engine.Patch{Title: &title, ParentSet: true, Parent: &parent, Progress: &progress}})
		if e != nil {
			t.Fatal(e)
		}
		m.doc = r.Document
	}
	m.selected = 3
	m.refresh()
	m.notice = "#3 保存済み"
	if e := os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(path, []byte(m.View().Content+"\n"), 0600); e != nil {
		t.Fatal(e)
	}
}

func TestSavingInputDoesNotQuitOnQ(t *testing.T) {
	m := testModel(t)
	press(m, "e")
	m.input.set("saved title")
	save := press(m, "enter")
	press(m, "q")
	if m.exitPending {
		t.Fatal("q during input save requested exit")
	}
	_, quit := m.Update(save())
	if quit != nil || m.doc.Tasks[0].Title != "saved title" {
		t.Fatal("unexpected exit or unsaved title")
	}
}

func TestReloadNoticeAndNewTaskSummary(t *testing.T) {
	m := testModel(t)
	for _, uncertain := range []bool{false, true} {
		m.uncertain = uncertain
		m.Update(readMsg{doc: m.doc})
		if strings.Contains(m.notice, "保存結果が不明") != uncertain {
			t.Fatal(m.notice)
		}
	}
	for _, k := range []string{"a", "/"} {
		press(m, k)
		summary := strings.Join(m.summary(), "\n")
		if strings.Contains(summary, "最新:") || strings.Contains(summary, "(#1)") {
			t.Fatal(summary)
		}
		press(m, "esc")
	}
}
