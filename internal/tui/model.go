package tui

import (
	tea "charm.land/bubbletea/v2"
	"context"
	"errors"
	"fmt"
	"github.com/charmbracelet/x/term"
	"os"
	"strconv"
	"strings"
	"time"
	"tque/internal/engine"
	"tque/internal/model"
)

type Model struct {
	service                        engine.Service
	ctx                            context.Context
	dir                            string
	doc                            model.Document
	views                          map[int64]engine.View
	rows                           []engine.View
	treePrefixes                   map[int64]string
	children                       map[int64][]int64
	selected                       int64
	width, height, offset, matched int
	idWidth                        int
	collapsed                      map[int64]bool
	filter                         int
	search, sort, today            string
	mode, helpReturn               string
	input                          editor
	draftID, revision              int64
	previousSearch                 string
	busy, exitPending, uncertain   bool
	notice                         string
	page                           int
	noColor                        bool
}
type resultMsg struct {
	result    engine.Result
	err       error
	latest    *model.Document
	reloadErr error
}
type readMsg struct {
	doc model.Document
	err error
}
type dayMsg time.Time

func Run(ctx context.Context, s engine.Service, dir string) error {
	if !term.IsTerminal(os.Stdin.Fd()) || !term.IsTerminal(os.Stdout.Fd()) {
		return model.Err("InvalidInput", "TUI requires an interactive terminal")
	}
	d, e := s.Repo.Read(ctx)
	if e != nil {
		return e
	}
	m := New(s, dir, d)
	m.ctx = ctx
	_, e = tea.NewProgram(m, tea.WithContext(ctx)).Run()
	return e
}
func New(s engine.Service, dir string, d model.Document) *Model {
	m := &Model{service: s, ctx: context.Background(), dir: dir, doc: d, width: 80, height: 24, collapsed: map[int64]bool{}, sort: "id", mode: "list", noColor: os.Getenv("NO_COLOR") != ""}
	m.today = m.now().Format(time.DateOnly)
	m.refresh()
	return m
}
func (m *Model) now() time.Time {
	if m.service.Now != nil {
		return m.service.Now()
	}
	return time.Now()
}
func tick() tea.Cmd             { return tea.Tick(time.Second, func(t time.Time) tea.Msg { return dayMsg(t) }) }
func (m *Model) Init() tea.Cmd  { return tick() }
func (m *Model) filtered() bool { return m.search != "" || m.filter != 0 }
func (m *Model) refresh() {
	q := engine.Query{All: m.filter == 0, Tree: true, Search: m.search, Sort: m.sort, Today: m.today, DueToday: m.filter == 2, Overdue: m.filter == 3, Ready: m.filter == 4}
	all, n := engine.Select(m.doc, q)
	m.matched = n
	m.views = engine.Views(m.doc)
	m.children = map[int64][]int64{}
	highest := int64(1)
	for _, t := range m.doc.Tasks {
		highest = max(highest, t.ID)
		if t.ParentID != nil {
			m.children[*t.ParentID] = append(m.children[*t.ParentID], t.ID)
		}
	}
	m.idWidth = len(strconv.FormatInt(highest, 10))
	m.rows = nil
	hide := -1
	for _, r := range all {
		if hide >= 0 && r.Depth > hide {
			continue
		}
		hide = -1
		m.rows = append(m.rows, r)
		if !m.filtered() && m.collapsed[r.ID] {
			hide = r.Depth
		}
	}
	m.treePrefixes = engine.TreePrefixes(m.rows)
	if m.index() < 0 {
		m.selected = 0
		for _, r := range m.rows {
			if !r.Context {
				m.selected = r.ID
				break
			}
		}
	}
	m.ensureVisible()
}
func (m *Model) index() int {
	for i, r := range m.rows {
		if r.ID == m.selected {
			return i
		}
	}
	return -1
}
func (m *Model) ensureVisible() {
	slots := m.listSlots()
	i := -1
	for line, row := range slots {
		if row >= 0 && m.rows[row].ID == m.selected {
			i = line
			break
		}
	}
	count := m.listHeight()
	if i < 0 {
		m.offset = 0
		return
	}
	if i < m.offset {
		m.offset = i
	}
	if i >= m.offset+count {
		m.offset = i - count + 1
	}
	m.offset = max(0, min(m.offset, max(0, len(slots)-count)))
}

// Blank separators occupy screen space but are not selectable task rows.
func (m *Model) listSlots() []int {
	slots := make([]int, 0, len(m.rows))
	for i := range m.rows {
		if engine.TreeGap(m.rows, i) {
			slots = append(slots, -1)
		}
		slots = append(slots, i)
	}
	return slots
}
func (m *Model) move(delta int) {
	if len(m.rows) == 0 {
		return
	}
	i := max(0, min(len(m.rows)-1, m.index()+delta))
	m.selected = m.rows[i].ID
	m.ensureVisible()
}

// Scroll the viewport while keeping selection on the same screen line.
// Skip separator lines, and leave both viewport and selection unchanged if
// no task can occupy that line before reaching the viewport boundary.
func (m *Model) scrollFixed(delta int) {
	slots := m.listSlots()
	line := -1
	for i, row := range slots {
		if row >= 0 && m.rows[row].ID == m.selected {
			line = i
			break
		}
	}
	anchor := line - m.offset
	if line < 0 || anchor < 0 || anchor >= m.listHeight() || delta == 0 {
		return
	}
	step := 1
	if delta < 0 {
		step = -1
	}
	last := max(0, len(slots)-m.listHeight())
	for offset := m.offset + step; offset >= 0 && offset <= last; offset += step {
		row := slots[offset+anchor]
		if row < 0 {
			continue
		}
		m.offset = offset
		m.selected = m.rows[row].ID
		return
	}
}
func (m *Model) begin(mode, value string) {
	m.mode = mode
	m.input.set(value)
	m.draftID = m.selected
	m.revision = m.doc.Revision
}
func (m *Model) save(r engine.Request) tea.Cmd {
	if m.uncertain {
		m.notice = "保存結果が不明です。rで再読込し内容を確認してください"
		return nil
	}
	m.busy = true
	if !m.editing() && m.mode != "delete" {
		m.draftID = r.ID
	}
	m.notice = "保存中..."
	rev := m.revision
	s := m.service
	ctx := m.ctx
	return func() tea.Msg {
		res, e := s.Execute(ctx, &rev, r)
		msg := resultMsg{result: res, err: e}
		var typed *model.Error
		if errors.As(e, &typed) && typed.Kind == "RevisionConflict" {
			d, re := s.Repo.Read(ctx)
			msg.reloadErr = re
			if re == nil {
				msg.latest = &d
			}
		}
		return msg
	}
}
func (m *Model) read() tea.Cmd {
	m.busy = true
	return func() tea.Msg { d, e := m.service.Repo.Read(m.ctx); return readMsg{d, e} }
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ensureVisible()
		return m, nil
	case dayMsg:
		day := m.now().Format(time.DateOnly)
		if day != m.today {
			m.today = day
			m.refresh()
		}
		return m, tick()
	case readMsg:
		m.busy = false
		exitPending := m.exitPending
		m.exitPending = false
		if msg.err != nil {
			m.notice = msg.err.Error()
			return m, nil
		}
		wasUncertain := m.uncertain
		m.acceptDocument(msg.doc)
		m.uncertain = false
		m.mode = "list"
		m.refresh()
		m.notice = "再読込しました"
		if wasUncertain {
			m.notice += "。保存結果が不明だった操作は内容を確認してください"
		}
		if exitPending {
			return m, tea.Quit
		}
		return m, nil
	case resultMsg:
		m.busy = false
		if msg.err != nil {
			m.exitPending = false
			m.notice = msg.err.Error()
			var typed *model.Error
			if errors.As(msg.err, &typed) && typed.Kind == "CommitUncertain" {
				m.uncertain = true
				m.notice += " / rで再読込して確認。再送しません"
			}
			if msg.latest != nil {
				numbersChanged := m.acceptDocument(*msg.latest)
				m.refresh()
				m.revision = m.doc.Revision
				if numbersChanged && m.mode != "search" {
					m.notice += " / 番号が変更されたため編集中止。入力案: " + m.input.text
					m.mode = "list"
				} else if _, ok := m.views[m.draftID]; !ok && m.mode != "add" && m.mode != "search" {
					m.notice += " / 対象が消えたため編集中止。入力案: " + m.input.text
					m.mode = "list"
				} else {
					m.notice += " / 最新値は下部詳細を確認。入力案を保持しています"
				}
			}
			if msg.reloadErr != nil {
				m.notice += " / 再読込失敗: " + msg.reloadErr.Error()
				m.uncertain = true
			}
			if msg.result.Changed {
				m.acceptDocument(msg.result.Document)
				m.refresh()
				m.mode = "list"
			}
			return m, nil
		}
		oldIndex := m.index()
		id := msg.result.ID
		m.acceptDocument(msg.result.Document)
		m.mode = "list"
		m.input.set("")
		// Expand the new/changed task's ancestors before rebuilding the view.
		byID := engine.Views(m.doc)
		if v, ok := byID[id]; ok {
			for p := v.ParentID; p != nil; p = byID[*p].ParentID {
				delete(m.collapsed, *p)
			}
		}
		m.selected = id
		m.refresh()
		visible := false
		for _, r := range m.rows {
			if r.ID == id {
				visible = true
				break
			}
		}
		m.notice = fmt.Sprintf("#%d 保存済み", id)
		if len(msg.result.Renumbered) > 0 {
			m.notice = "番号変更: " + engine.FormatIDChanges(msg.result.Renumbered)
		}
		if !msg.result.Changed {
			m.notice = fmt.Sprintf("#%d 変更なし", id)
		}
		if !visible {
			m.notice += "（削除済み、または現在の条件では非表示）"
			if len(m.rows) > 0 {
				m.selected = m.rows[max(0, min(oldIndex, len(m.rows)-1))].ID
			}
			m.ensureVisible()
		}
		if m.exitPending {
			return m, tea.Quit
		}
		return m, nil
	case tea.PasteMsg:
		if !m.busy && !m.uncertain && m.width >= 40 && m.height >= 10 && m.editing() {
			m.input.insert(msg.Content)
			if m.mode == "search" {
				m.search = m.input.text
				m.refresh()
			}
		}
		return m, nil
	case tea.KeyPressMsg:
		k := msg.String()
		if m.busy {
			if k == "ctrl+c" {
				m.exitPending = true
				m.notice = "処理中。保存成功後に終了します"
			}
			return m, nil
		}
		if k == "ctrl+c" {
			return m, tea.Quit
		}
		if m.width < 40 || m.height < 10 {
			if k == "q" {
				return m, tea.Quit
			}
			return m, nil
		}
		if m.mode == "help" || m.mode == "detail" {
			if k == "esc" || m.mode == "help" && k == "?" {
				if m.mode == "help" {
					m.mode = m.helpReturn
				} else {
					m.mode = "list"
				}
				m.page = 0
				return m, nil
			}
			switch k {
			case "up", "k":
				m.page = max(0, m.page-1)
			case "down", "j":
				m.page = min(m.maxPage(), m.page+1)
			case "pgdown":
				m.page = min(m.maxPage(), m.page+max(1, m.height-3))
			case "pgup":
				m.page = max(0, m.page-max(1, m.height-3))
			case "q":
				return m, tea.Quit
			}
			return m, nil
		}
		if m.uncertain {
			switch k {
			case "r":
				return m, m.read()
			case "q":
				return m, tea.Quit
			case "?":
				m.helpReturn = m.mode
				m.mode = "help"
				m.page = 0
			case "enter":
				if m.selected != 0 {
					m.mode = "detail"
					m.page = 0
				}
			}
			return m, nil
		}
		if m.mode == "delete" {
			switch k {
			case "y":
				return m, m.save(engine.Request{Op: "rm", ID: m.draftID, Cascade: true})
			case "n", "q", "esc", "enter":
				m.mode = "list"
			case "?":
				m.helpReturn = m.mode
				m.mode = "help"
				m.page = 0
			}
			return m, nil
		}
		if m.editing() {
			if k == "esc" {
				if m.mode == "search" {
					m.search = m.previousSearch
					m.refresh()
				}
				m.mode = "list"
				return m, nil
			}
			if k == "enter" {
				if m.mode == "search" {
					m.mode = "list"
					return m, nil
				}
				r, e := m.request()
				if e != nil {
					m.notice = e.Error()
					return m, nil
				}
				return m, m.save(r)
			}
			switch k {
			case "left", "right", "home", "end", "backspace", "delete", "ctrl+a", "ctrl+e":
				m.input.key(k)
			default:
				if msg.Text != "" {
					m.input.insert(msg.Text)
				}
			}
			if m.mode == "search" {
				m.search = m.input.text
				m.refresh()
			}
			return m, nil
		}
		switch k {
		case "q":
			return m, tea.Quit
		case "up", "k":
			m.move(-1)
		case "down", "j":
			m.move(1)
		case "J", "shift+j", "shift+J":
			m.scrollFixed(1)
		case "K", "shift+k", "shift+K":
			m.scrollFixed(-1)
		case "pgup":
			m.move(-m.listHeight())
		case "pgdown":
			m.move(m.listHeight())
		case "?":
			m.helpReturn = m.mode
			m.mode = "help"
			m.page = 0
		case "r":
			return m, m.read()
		case "/":
			m.previousSearch = m.search
			m.begin("search", m.search)
		case "esc":
			m.search = ""
			m.refresh()
		case "f":
			m.filter = (m.filter + 1) % 5
			m.refresh()
		case "o":
			if m.sort == "id" {
				m.sort = "due"
			} else if m.sort == "due" {
				m.sort = "name"
			} else {
				m.sort = "id"
			}
			m.refresh()
		case "a":
			m.begin("add", "")
		default:
			v, ok := m.views[m.selected]
			if !ok {
				m.notice = "対象タスクがありません"
				return m, nil
			}
			switch k {
			case "enter":
				m.mode = "detail"
				m.page = 0
			case "left":
				if !m.filtered() && !m.collapsed[v.ID] && len(m.children[v.ID]) > 0 {
					m.collapsed[v.ID] = true
					m.refresh()
				} else if v.ParentID != nil {
					m.selected = *v.ParentID
					m.ensureVisible()
				}
			case "right":
				if !m.filtered() && m.collapsed[v.ID] {
					delete(m.collapsed, v.ID)
					m.refresh()
				} else {
					i := m.index()
					if i >= 0 && i+1 < len(m.rows) && m.rows[i+1].ParentID != nil && *m.rows[i+1].ParentID == v.ID {
						m.move(1)
					}
				}
			case "space":
				m.revision = m.doc.Revision
				op := "done"
				if v.Status == "done" {
					op = "undone"
				}
				return m, m.save(engine.Request{Op: op, ID: v.ID})
			case "+", "-":
				if v.Status == "done" {
					m.notice = "先にSpaceで未完了に戻してください"
					return m, nil
				}
				n := 10
				if k == "-" {
					n = -10
				}
				p := strconv.Itoa(max(0, min(100, v.EffectiveProgress+n)))
				m.revision = m.doc.Revision
				return m, m.save(engine.Request{Op: "edit", ID: v.ID, Patch: engine.Patch{Progress: &p}})
			case "A":
				m.begin("child", "")
			case "e":
				m.begin("title", v.Title)
			case "i":
				m.begin("renumber", strconv.FormatInt(v.ID, 10))
			case "s":
				m.begin("start", v.StartDate)
			case "d":
				m.begin("due", v.DueDate)
			case "p":
				value := v.ProgressMode
				if value == "manual" {
					value = strconv.Itoa(v.Progress)
				}
				m.begin("progress", value)
			case "l":
				ids := []string{}
				for _, id := range v.DependsOn {
					ids = append(ids, strconv.FormatInt(id, 10))
				}
				m.begin("deps", strings.Join(ids, ","))
			case "x", "delete":
				ids, e := engine.Removal(m.doc, v.ID, true)
				if e != nil {
					m.notice = e.Error()
					return m, nil
				}
				m.begin("delete", "")
				m.notice = fmt.Sprintf("#%d と子孫を含む計%d件を削除（画面に出ていない子孫も対象）", v.ID, len(ids))
			}
		}
	}
	return m, nil
}
func (m *Model) editing() bool {
	switch m.mode {
	case "add", "child", "title", "start", "due", "progress", "deps", "search", "renumber":
		return true
	}
	return false
}
func (m *Model) request() (engine.Request, error) {
	r := engine.Request{Op: "edit", ID: m.draftID}
	s := m.input.text
	switch m.mode {
	case "renumber":
		id, err := engine.ID(s)
		if err != nil {
			return r, err
		}
		r.Op, r.NewID = "renumber", id
	case "add", "child":
		r.Op = "add"
		r.Patch.Title = &s
		if m.mode == "child" {
			p := m.draftID
			r.Patch.ParentSet = true
			r.Patch.Parent = &p
		}
	case "title":
		r.Patch.Title = &s
	case "start":
		r.Patch.Start = &s
	case "due":
		r.Patch.Due = &s
	case "progress":
		r.Patch.Progress = &s
	case "deps":
		r.Patch.DepsSet = true
		if s != "" {
			for _, v := range strings.Split(s, ",") {
				id, e := engine.ID(strings.TrimSpace(v))
				if e != nil {
					return r, e
				}
				r.Patch.Deps = append(r.Patch.Deps, id)
			}
		}
	}
	return r, nil
}

// acceptDocument preserves task identity across external or local renumbering.
// A removed task must not silently select a different task reusing its number.
func (m *Model) acceptDocument(next model.Document) bool {
	oldKeys := map[int64]string{}
	newIDs := map[string]int64{}
	changed := false
	for _, t := range m.doc.Tasks {
		oldKeys[t.ID] = t.InternalID
	}
	for _, t := range next.Tasks {
		if t.InternalID != "" {
			newIDs[t.InternalID] = t.ID
		}
	}
	translate := func(id int64) int64 {
		key := oldKeys[id]
		if key == "" {
			return id
		}
		return newIDs[key]
	}
	for id, key := range oldKeys {
		if key != "" && newIDs[key] != id {
			changed = true
		}
	}
	m.selected = translate(m.selected)
	m.draftID = translate(m.draftID)
	collapsed := map[int64]bool{}
	for id, value := range m.collapsed {
		if newID := translate(id); newID != 0 {
			collapsed[newID] = value
		}
	}
	m.collapsed = collapsed
	m.doc = next
	return changed
}
