package tui

import (
	tea "charm.land/bubbletea/v2"
	"context"
	"fmt"
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
	if lines[8] != "" || !strings.Contains(lines[9], "RootC") || !strings.Contains(lines[10], "RootD") {
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

func TestStartupCollapsesAllParents(t *testing.T) {
	m := testModel(t)
	one, two := int64(1), int64(2)
	m.doc.Tasks = []model.Task{
		{ID: 1, Title: "Root", Status: "todo"},
		{ID: 2, Title: "Child", ParentID: &one, Status: "todo"},
		{ID: 3, Title: "Grandchild", ParentID: &two, Status: "todo"},
		{ID: 4, Title: "Other", Status: "todo"},
	}
	m = New(m.service, m.dir, m.doc)
	ids := func() []int64 {
		out := []int64{}
		for _, r := range m.rows {
			out = append(out, r.ID)
		}
		return out
	}
	if got := ids(); len(got) != 2 || got[0] != 1 || got[1] != 4 || m.selected != 1 {
		t.Fatal("startup rows", got, m.selected)
	}
	press(m, "right")
	if got := ids(); len(got) != 3 || got[1] != 2 {
		t.Fatal("root expansion should keep grandchildren collapsed", got)
	}
	m.search = "Grandchild"
	m.refresh()
	if len(m.rows) != 3 {
		t.Fatal("search should reveal collapsed path", ids())
	}
	m.search = ""
	m.refresh()
	five := int64(4)
	m.doc.Tasks = append(m.doc.Tasks, model.Task{ID: 5, Title: "Later", ParentID: &five, Status: "todo"})
	m.Update(readMsg{doc: m.doc})
	if got := ids(); len(got) != 4 || got[3] != 5 || !m.collapsed[2] || m.collapsed[4] {
		t.Fatal("reload should keep state and leave new parents expanded", got)
	}
}

// rootModel stores:
// 1 Blog
// └─ 2 Design
//
//	├─ 3 Draft
//	│  └─ 4 Outline
//	└─ 5 Review
//
// 6 Work
// └─ 7 design
func rootModel(t *testing.T) *Model {
	t.Helper()
	s := engine.Service{Repo: &storage.Store{Dir: t.TempDir()}, Now: func() time.Time { return time.Date(2026, 10, 21, 0, 0, 0, 0, time.UTC) }}
	var d model.Document
	for _, item := range []struct {
		title  string
		parent int64
	}{{"Blog", 0}, {"Design", 1}, {"Draft", 2}, {"Outline", 3}, {"Review", 2}, {"Work", 0}, {"design", 6}} {
		r := engine.Request{Op: "add", Patch: engine.Patch{Title: &item.title}}
		if item.parent != 0 {
			p := item.parent
			r.Patch.ParentSet, r.Patch.Parent = true, &p
		}
		res, e := s.Execute(context.Background(), nil, r)
		if e != nil {
			t.Fatal(e)
		}
		d = res.Document
	}
	m := New(s, "D:/data", d)
	m.noColor = true
	return m
}
func rowIDs(m *Model) []int64 {
	out := []int64{}
	for _, r := range m.rows {
		out = append(out, r.ID)
	}
	return out
}

func TestRootFocus(t *testing.T) {
	m := rootModel(t)
	if e := m.chooseRoot(" design "); e != nil || m.mode != "pick" || len(m.pick) != 2 {
		t.Fatal("ambiguous title should open the picker", e, m.mode, m.pick)
	}
	view := m.View().Content
	for _, want := range []string{`起点を選択: "design"  2件`, "> #2 [ ] Blog > Design", "#7 [ ] Work > design", "Esc:全体表示で開く"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q\n%s", want, view)
		}
	}
	press(m, "down")
	press(m, "up")
	press(m, "enter")
	if m.mode != "list" || m.rootID != 2 || m.selected != 2 {
		t.Fatal(m.mode, m.rootID, m.selected)
	}
	// The root and all its descendants are expanded.
	if got := rowIDs(m); len(got) != 4 || got[0] != 2 || got[1] != 3 || got[2] != 4 || got[3] != 5 || m.rows[0].Depth != 0 {
		t.Fatal("root view", got)
	}
	if !m.collapsed[1] || !m.collapsed[6] {
		t.Fatal("tasks outside the root keep the startup collapse", m.collapsed)
	}
	lines := strings.Split(m.View().Content, "\n")
	if !strings.HasPrefix(lines[0], "tque  起点: Blog > Design (#2)  保存先: D:/data") || strings.Contains(m.View().Content, "Blog  ") {
		t.Fatal(lines[0])
	}
	press(m, "left") // collapse the root
	press(m, "left") // must not move above the root
	if m.selected != 2 || len(m.rows) != 1 {
		t.Fatal("left escaped the root", m.selected, rowIDs(m))
	}
	press(m, "right")

	press(m, "a")
	if m.mode != "child" || m.draftID != 2 || !strings.Contains(strings.Join(m.summary(), "\n"), "#2 の子を追加（起点）") {
		t.Fatal(m.mode, m.draftID, m.summary())
	}
	m.input.set("Publish")
	m.Update(press(m, "enter")())
	added := m.views[m.selected]
	if added.Title != "Publish" || added.ParentID == nil || *added.ParentID != 2 {
		t.Fatal("a must add a child of the root", added)
	}

	// Renumbering the root keeps the focus by internal identity.
	if _, e := m.service.Execute(context.Background(), nil, engine.Request{Op: "renumber", ID: 2, NewID: 20}); e != nil {
		t.Fatal(e)
	}
	m.Update(m.read()())
	if m.rootID != 20 || m.rows[0].ID != 20 || !strings.Contains(m.titleLine(), "(#20)") {
		t.Fatal(m.rootID, rowIDs(m), m.titleLine())
	}
	m.mode = "help"
	if !strings.Contains(strings.Join(m.pageLines(), "\n"), "Blog > Design (#20)") {
		t.Fatal("help should show the root path")
	}
	m.mode = "list"

	// Deleting the root returns to the full tree with a notice.
	if _, e := m.service.Execute(context.Background(), nil, engine.Request{Op: "rm", ID: 20, Cascade: true}); e != nil {
		t.Fatal(e)
	}
	m.Update(m.read()())
	if m.rootID != 0 || !strings.Contains(m.notice, "起点 #20 は削除されたため全体表示") || len(m.rows) != 2 || m.rows[0].ID != 1 || m.rows[1].ID != 6 {
		t.Fatal(m.rootID, m.notice, rowIDs(m))
	}
	if strings.Contains(m.titleLine(), "起点") {
		t.Fatal(m.titleLine())
	}
}

func TestRootChoiceOutcomes(t *testing.T) {
	m := rootModel(t)
	if e := m.chooseRoot("missing"); e == nil || !strings.Contains(e.Error(), "NotFound") {
		t.Fatal(e)
	}
	if e := m.chooseRoot("  "); e == nil {
		t.Fatal("empty root accepted")
	}
	if e := m.chooseRoot("3"); e != nil || m.mode != "list" || m.rootID != 3 || len(m.rows) != 2 {
		t.Fatal("single match opens directly", e, m.mode, rowIDs(m))
	}

	m = rootModel(t)
	_ = m.chooseRoot("design")
	press(m, "esc")
	if got := rowIDs(m); m.mode != "list" || m.rootID != 0 || len(got) != 2 || got[0] != 1 || got[1] != 6 {
		t.Fatal("Esc should open the full tree collapsed", m.mode, m.rootID, got)
	}
	m = rootModel(t)
	_ = m.chooseRoot("design")
	if press(m, "r") != nil || m.mode != "pick" {
		t.Fatal("picker must not reload")
	}
	if press(m, "q") == nil {
		t.Fatal("q should quit from the picker")
	}
	for _, size := range [][2]int{{40, 10}, {80, 24}} {
		m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		lines := strings.Split(m.View().Content, "\n")
		if len(lines) > size[1] {
			t.Fatal("picker height", size, len(lines))
		}
		for _, line := range lines {
			if ansi.StringWidth(line) > size[0] {
				t.Fatal("picker width", size, line)
			}
		}
	}
}

func TestRootTitleLineKeepsRootVisible(t *testing.T) {
	m := rootModel(t)
	title := "Outline"
	_ = m.chooseRoot("4")
	for _, width := range []int{40, 50, 60, 120} {
		m.width = width
		line := m.titleLine()
		if ansi.StringWidth(line) > width || !strings.Contains(line, title) || !strings.Contains(line, "(#4)") {
			t.Fatalf("width %d: %q", width, line)
		}
	}
	m.width = 40
	if line := m.titleLine(); strings.Contains(line, "保存先") || !strings.Contains(line, "..") {
		t.Fatalf("narrow header should drop storage and ancestors first: %q", line)
	}
}

func TestExternalChangesRefreshTUI(t *testing.T) {
	dir := t.TempDir()
	s := engine.Service{Repo: &storage.Store{Dir: dir}, Now: func() time.Time { return time.Date(2026, 10, 21, 0, 0, 0, 0, time.UTC) }}
	exec := func(r engine.Request) {
		t.Helper()
		if _, e := s.Execute(context.Background(), nil, r); e != nil {
			t.Fatal(e)
		}
	}
	add := func(title string) { exec(engine.Request{Op: "add", Patch: engine.Patch{Title: &title}}) }
	add("first")
	d, e := s.Repo.Read(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	m := New(s, dir, d)
	m.noColor = true
	if m.watch() != nil {
		t.Fatal("unchanged file must not be read")
	}

	add("from CLI")
	c := m.watch()
	if c == nil {
		t.Fatal("external change not detected")
	}
	m.Update(c())
	if len(m.doc.Tasks) != 2 || m.notice != "外部の変更を反映しました" || m.selected != 1 {
		t.Fatal(len(m.doc.Tasks), m.notice, m.selected)
	}
	if m.watch() != nil {
		t.Fatal("same stamp read twice")
	}

	// Our own save changes the file but is not reported as external.
	m.Update(press(m, "space")())
	m.notice = "own"
	if c := m.watch(); c != nil {
		m.Update(c())
	}
	if m.notice != "own" {
		t.Fatal(m.notice)
	}

	// While editing, the change waits until the list is shown again.
	press(m, "e")
	add("while editing")
	m.Update(m.watch()())
	if len(m.doc.Tasks) != 2 || m.pending == nil || m.notice != "外部で更新されました。一覧に戻ると反映します" {
		t.Fatal(len(m.doc.Tasks), m.notice)
	}
	press(m, "esc")
	if len(m.doc.Tasks) != 3 || m.pending != nil || m.notice != "外部の変更を反映しました" {
		t.Fatal(len(m.doc.Tasks), m.notice)
	}

	// Saving and uncertain states are not refreshed automatically.
	add("while saving")
	m.busy = true
	if m.watch() != nil {
		t.Fatal("watched while saving")
	}
	m.busy, m.uncertain = false, true
	if m.watch() != nil {
		t.Fatal("watched while uncertain")
	}
	m.uncertain = false
	m.Update(m.watch()())
	if len(m.doc.Tasks) != 4 {
		t.Fatal("change after saving was missed")
	}

	// Read errors keep the view and are reported once.
	if e := os.WriteFile(filepath.Join(dir, "tasks.json"), []byte("{"), 0600); e != nil {
		t.Fatal(e)
	}
	m.Update(m.watch()())
	if len(m.doc.Tasks) != 4 || !strings.HasPrefix(m.notice, "外部の変更を読み込めません") {
		t.Fatal(m.notice)
	}
	m.notice = "other"
	m.Update(m.watch()())
	if m.notice != "other" || len(m.doc.Tasks) != 4 {
		t.Fatal("repeated error notice", m.notice)
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
		{"parent", engine.View{Task: model.Task{ID: 1, Title: "親タスク", Status: "todo"}}, "38;2;255;255;255"},
		{"done", engine.View{Task: model.Task{ID: 2, Title: "完了", Status: "done"}, Depth: 1}, "38;2;176;176;176"},
		{"overdue", engine.View{Task: model.Task{ID: 3, Title: "期限切れ", Status: "todo", DueDate: "2026-10-20"}}, "91"},
		{"today", engine.View{Task: model.Task{ID: 3, Title: "今日", Status: "todo", DueDate: m.today}}, "93"},
		{"blocked", engine.View{Task: model.Task{ID: 3, Title: "待機", Status: "todo"}, Blocked: true}, "93"},
		{"child", engine.View{Task: model.Task{ID: 2, Title: "子タスク", Status: "todo"}, Depth: 1}, "38;2;0;215;255"},
		{"grandchild", engine.View{Task: model.Task{ID: 2, Title: "孫", Status: "todo"}, Depth: 2}, "38;2;0;215;175"},
		{"great grandchild", engine.View{Task: model.Task{ID: 2, Title: "ひ孫", Status: "todo"}, Depth: 3}, "38;2;95;159;255"},
		{"repeat", engine.View{Task: model.Task{ID: 2, Title: "深い子", Status: "todo"}, Depth: 5}, "38;2;0;215;255"},
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
	if !strings.Contains(selected, "\x1b[7m") || strings.Contains(selected, "38;2;") {
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

func TestFixedPositionScroll(t *testing.T) {
	m := testModel(t)
	one := int64(1)
	m.doc.Tasks = []model.Task{
		{ID: 1, Title: "root", Status: "todo"},
		{ID: 2, Title: "child", ParentID: &one, Status: "todo"},
		{ID: 3, Title: "next", Status: "todo"},
		{ID: 4, Title: "four", Status: "todo"},
		{ID: 5, Title: "five", Status: "todo"},
		{ID: 6, Title: "six", Status: "todo"},
	}
	m.height = 11 // three visible list lines
	m.refresh()
	m.move(1)
	anchor := func() int {
		for i, row := range m.listSlots() {
			if row >= 0 && m.rows[row].ID == m.selected {
				return i - m.offset
			}
		}
		return -1
	}
	if anchor() != 1 {
		t.Fatal("initial anchor")
	}
	press(m, "J") // skip the group separator
	if m.offset != 2 || m.selected != 3 || anchor() != 1 {
		t.Fatal(m.offset, m.selected, anchor())
	}
	press(m, "K")
	if m.offset != 0 || m.selected != 2 || anchor() != 1 {
		t.Fatal(m.offset, m.selected, anchor())
	}
	press(m, "K")
	if m.offset != 0 || m.selected != 2 {
		t.Fatal("scrolled past start")
	}
	for range 10 {
		press(m, "J")
		if anchor() != 1 {
			t.Fatal("anchor moved")
		}
	}
	if m.offset != 4 || m.selected != 5 {
		t.Fatal("end boundary", m.offset, m.selected)
	}
	press(m, "j")
	if m.selected != 6 {
		t.Fatal("lowercase navigation changed")
	}
	press(m, "k")
	if m.selected != 5 {
		t.Fatal("lowercase navigation changed")
	}
	m.height = 30
	m.ensureVisible()
	before := m.selected
	press(m, "J")
	if m.offset != 0 || m.selected != before {
		t.Fatal("scrolled fully visible list")
	}
	m.doc.Tasks = nil
	m.refresh()
	press(m, "J")
	press(m, "K")
	if m.offset != 0 || m.selected != 0 {
		t.Fatal("empty list moved")
	}
}

func TestScrollKeysAreTextWhenEditing(t *testing.T) {
	m := testModel(t)
	press(m, "e")
	m.input.set("")
	press(m, "J")
	press(m, "K")
	if m.input.text != "JK" || m.offset != 0 || m.selected != 1 {
		t.Fatal(m.input.text, m.offset, m.selected)
	}
}

func TestListHeaderColumnAlignment(t *testing.T) {
	m := testModel(t)
	m.noColor = true
	m.selected = 0
	column := func(s, token string) int {
		i := strings.Index(s, token)
		if i < 0 {
			t.Fatalf("missing %q in %q", token, s)
		}
		return ansi.StringWidth(s[:i])
	}
	for _, width := range []int{40, 59, 60, 79, 80, 100, 120} {
		for _, digits := range []int{2, 3, 19} {
			m.width, m.idWidth = width, digits
			header := m.listHeader()
			for _, tagged := range []bool{false, true} {
				v := engine.View{Task: model.Task{ID: 1, Title: "日本語タスク", Status: "todo", DueDate: "2026-10-24"}, EffectiveProgress: 37, Context: tagged, Blocked: tagged}
				if tagged {
					v.ProgressMode = "manual"
				}
				row := m.row(v)
				if ansi.StringWidth(header) != width || ansi.StringWidth(row) != width {
					t.Fatalf("width=%d digits=%d\n%q\n%q", width, digits, header, row)
				}
				if column(header, "状態") != column(row, "[ ]") {
					t.Fatal("status misaligned")
				}
				if column(header, "タスク") != column(row, "日") {
					t.Fatal("task misaligned")
				}
				progress := " 37%"
				if m.columns().bar {
					progress = "[###"
				}
				if column(header, "進捗") != column(row, progress) {
					t.Fatalf("progress misaligned: %q / %q", header, row)
				}
				if width >= 60 && column(header, "期日") != column(row, "2026-10-24") {
					t.Fatal("date misaligned")
				}
				if width < 60 && strings.Contains(header, "期日") {
					t.Fatal("hidden date has header")
				}
			}
		}
	}
}

// Indentation, branch markers, and tags must stay inside the task column even
// when large IDs leave little room for titles.
func TestIndentedRowAlignmentWithLargeIDs(t *testing.T) {
	m := testModel(t)
	m.noColor = true
	m.selected = 0
	column := func(s, token string) int {
		i := strings.Index(s, token)
		if i < 0 {
			t.Fatalf("missing %q in %q", token, s)
		}
		return ansi.StringWidth(s[:i])
	}
	prefixes := []string{"", "└─ ", "│  ├─ ", strings.Repeat("│  ", 12) + "└─ "}
	for _, width := range []int{40, 59, 60, 79, 80, 100, 120} {
		for _, digits := range []int{2, 3, 19} {
			m.width, m.idWidth = width, digits
			header := m.listHeader()
			for _, prefix := range prefixes {
				for _, branch := range []string{"none", "expanded", "collapsed"} {
					for _, tagged := range []bool{false, true} {
						m.treePrefixes = map[int64]string{1: prefix}
						m.children = map[int64][]int64{}
						m.collapsed = map[int64]bool{}
						if branch != "none" {
							m.children[1] = []int64{2}
							m.collapsed[1] = branch == "collapsed"
						}
						v := engine.View{Task: model.Task{ID: 1, Title: "日本語タスクのタイトル", Status: "todo", DueDate: "2026-10-24"}, EffectiveProgress: 37, Context: tagged, Blocked: tagged, Depth: strings.Count(prefix, "─")}
						if tagged {
							v.ProgressMode = "manual"
						}
						row := m.row(v)
						name := fmt.Sprintf("width=%d digits=%d prefix=%q branch=%s tagged=%v", width, digits, prefix, branch, tagged)
						if ansi.StringWidth(row) != width {
							t.Fatalf("%s: row width %d\n%q", name, ansi.StringWidth(row), row)
						}
						if column(header, "状態") != column(row, "[ ]") {
							t.Fatalf("%s: status misaligned\n%q\n%q", name, header, row)
						}
						progress := " 37%"
						if m.columns().bar {
							progress = "[###"
						}
						if column(header, "進捗") != column(row, progress) {
							t.Fatalf("%s: progress misaligned\n%q\n%q", name, header, row)
						}
						if width >= 60 && column(header, "期日") != column(row, "2026-10-24") {
							t.Fatalf("%s: date misaligned\n%q\n%q", name, header, row)
						}
					}
				}
			}
		}
	}
}
