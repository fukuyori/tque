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

func TestTreeSpacingAndScrolling(t *testing.T) {
	m := testModel(t)
	one, two := int64(1), int64(2)
	m.doc.Tasks = []model.Task{
		{ID: 1, Title: "RootA", Status: "todo"},
		{ID: 2, Title: "ChildA", ParentID: &one, Status: "todo"},
		{ID: 3, Title: "Grandchild", ParentID: &two, Status: "todo"},
		{ID: 4, Title: "ChildB", ParentID: &one, Status: "todo"},
		{ID: 5, Title: "RootC", Status: "todo"},
		{ID: 6, Title: "RootD", Status: "todo"},
	}
	m.refresh()
	view := m.View().Content
	lines := strings.Split(view, "\n")
	if lines[7] != "" || !strings.Contains(lines[8], "RootC") || !strings.Contains(lines[9], "RootD") {
		t.Fatal(view)
	}
	for _, guide := range []string{"├─ ", "│  └─ ", "└─ "} {
		if !strings.Contains(view, guide) {
			t.Fatal(view)
		}
	}
	m.height = 10
	for _, delta := range []int{1, 1, 1, 1, 1, -1, -1, -1, -1, -1} {
		m.move(delta)
		view = m.View().Content
		if !strings.Contains(view, m.row(m.rows[m.index()])) {
			t.Fatalf("selected task hidden: %d\n%s", m.selected, view)
		}
		if len(strings.Split(view, "\n")) > m.height {
			t.Fatal("screen overflow")
		}
	}
	m.collapsed[1] = true
	m.refresh()
	slots := m.listSlots()
	if len(slots) != 3 {
		t.Fatalf("collapsed group should have no separator: %v", slots)
	}
	m.collapsed[1] = false
	m.search = "Grandchild"
	m.refresh()
	if len(m.rows) != 3 || !strings.Contains(m.View().Content, "└─ ") {
		t.Fatal(m.View().Content)
	}
}

func TestTaskRowColors(t *testing.T) {
	m := testModel(t)
	m.noColor = false
	m.selected = 999
	m.children[1] = []int64{2}
	m.treePrefixes[2] = "└─ "
	for _, tc := range []struct {
		name  string
		row   engine.View
		color string
	}{
		{"parent", engine.View{Task: model.Task{ID: 1, Title: "親タスク", Status: "todo"}}, "96"},
		{"done", engine.View{Task: model.Task{ID: 2, Title: "完了", Status: "done"}}, "90"},
		{"overdue", engine.View{Task: model.Task{ID: 3, Title: "期限切れ", Status: "todo", DueDate: "2026-10-20"}}, "91"},
		{"today", engine.View{Task: model.Task{ID: 3, Title: "今日", Status: "todo", DueDate: m.today}}, "93"},
		{"blocked", engine.View{Task: model.Task{ID: 3, Title: "待機", Status: "todo"}, Blocked: true}, "93"},
		{"child", engine.View{Task: model.Task{ID: 2, Title: "子タスク", Status: "todo"}}, "90"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, width := range []int{40, 60, 80, 120} {
				m.width = width
				m.noColor = false
				colored := m.row(tc.row)
				m.noColor = true
				plain := m.row(tc.row)
				if ansi.Strip(colored) != plain || strings.Contains(plain, "\x1b") || ansi.StringWidth(colored) > width {
					t.Fatalf("color changed layout: %q / %q", colored, plain)
				}
				if width >= 60 && !strings.Contains(colored, tc.color) {
					t.Fatalf("missing color %s: %q", tc.color, colored)
				}
			}
		})
	}
	m.noColor = false
	m.selected = 1
	selected := m.row(engine.View{Task: model.Task{ID: 1, Title: "選択", Status: "done"}})
	if !strings.Contains(selected, "\x1b[7m") || strings.Contains(selected, "90m") {
		t.Fatalf("selection priority: %q", selected)
	}
}

func TestNoColorEnvironment(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	m := testModel(t)
	if !m.noColor || strings.Contains(m.View().Content, "\x1b") {
		t.Fatal("NO_COLOR not respected")
	}
}

func TestSortCycleIncludesName(t *testing.T) {
	m := testModel(t)
	m.doc.Tasks = []model.Task{{ID: 1, Title: "Zebra", Status: "todo"}, {ID: 2, Title: "Apple", Status: "todo"}}
	m.refresh()
	for _, want := range []string{"due", "name", "id"} {
		press(m, "o")
		if m.sort != want {
			t.Fatalf("sort=%s want=%s", m.sort, want)
		}
		if want == "name" && m.rows[0].ID != 2 {
			t.Fatal("not sorted by name")
		}
		if m.selected != 1 {
			t.Fatal("selection changed")
		}
	}
}

func TestTUIRenumberAndExternalRefresh(t *testing.T) {
	m := testModel(t)
	for _, title := range []string{"second", "third"} {
		_, err := m.service.Execute(context.Background(), nil, engine.Request{Op: "add", Patch: engine.Patch{Title: &title}})
		if err != nil {
			t.Fatal(err)
		}
	}
	m.Update(m.read()())
	m.selected = 3
	keyBefore := m.views[3].InternalID
	press(m, "i")
	if m.mode != "renumber" {
		t.Fatal(m.mode)
	}
	m.input.set("0")
	if c := press(m, "enter"); c != nil {
		t.Fatal("invalid number saved")
	}
	press(m, "esc")
	if m.views[3].InternalID != keyBefore {
		t.Fatal("cancel changed task")
	}
	press(m, "i")
	m.input.set("1")
	c := press(m, "enter")
	if c == nil {
		t.Fatal(m.notice)
	}
	m.Update(c())
	if m.selected != 1 || m.views[1].InternalID != keyBefore || !strings.Contains(m.notice, "3→1") {
		t.Fatal(m.selected, m.notice)
	}
	m.collapsed[1] = true
	_, err := m.service.Execute(context.Background(), nil, engine.Request{Op: "renumber", ID: 1, NewID: 3})
	if err != nil {
		t.Fatal(err)
	}
	m.Update(m.read()())
	if m.selected != 3 || !m.collapsed[3] || m.views[3].InternalID != keyBefore {
		t.Fatal("reload lost identity")
	}
	press(m, "i")
	m.input.set("2")
	_, err = m.service.Execute(context.Background(), nil, engine.Request{Op: "renumber", ID: 3, NewID: 1})
	if err != nil {
		t.Fatal(err)
	}
	c = press(m, "enter")
	if c == nil {
		t.Fatal(m.notice)
	}
	m.Update(c())
	if m.mode != "list" || !strings.Contains(m.notice, "編集中止") || m.selected != 1 || m.views[1].InternalID != keyBefore {
		t.Fatal(m.mode, m.notice, m.selected)
	}
	d, err := m.service.Repo.Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if engine.Views(d)[1].InternalID != keyBefore {
		t.Fatal("stale edit changed task")
	}
}
