package tui

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"fmt"
	"github.com/charmbracelet/x/ansi"
	"strings"
	"tque/internal/engine"
	"unicode"
)

func safe(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return '�'
		}
		return r
	}, s)
}
func fit(s string, w int) string {
	if w <= 0 {
		return ""
	}
	return ansi.Truncate(s, w, "...")
}
func pad(s string, w int) string {
	s = fit(s, w)
	return s + strings.Repeat(" ", max(0, w-ansi.StringWidth(s)))
}
func (m *Model) detailHeight() int {
	if m.height >= 20 {
		return 4
	}
	if m.editing() || m.mode == "delete" {
		return 2
	}
	if m.height >= 14 {
		return 1
	}
	return 0
}
func (m *Model) listHeight() int { return max(1, m.height-7-m.detailHeight()) }

func (m *Model) View() tea.View {
	lines := []string{}
	if m.width < 40 || m.height < 10 {
		v := tea.NewView(fit(fmt.Sprintf("画面が小さすぎます: %dx%d (40x10以上) q:終了", m.width, m.height), m.width))
		v.AltScreen = true
		return v
	}
	if m.mode == "detail" || m.mode == "help" {
		content := m.pageLines()
		m.page = min(m.page, m.maxPage())
		title := "全文詳細（読み取り専用）"
		if m.mode == "help" {
			title = "操作ヘルプ"
		}
		lines = append(lines, title)
		for i := 0; i < m.height-2; i++ {
			idx := m.page + i
			if idx < len(content) {
				lines = append(lines, content[idx])
			} else {
				lines = append(lines, "")
			}
		}
		lines = append(lines, fmt.Sprintf("↑↓:スクロール  Esc:戻る  %d/%d", m.page+1, max(1, len(content))))
	} else {
		filters := []string{"全件", "未完了", "今日まで", "期限超過", "着手可能"}
		lines = append(lines, "tque  保存先: "+safe(m.dir), fmt.Sprintf("表示:%s  並び:%s  一致:%d件  検索:%s", filters[m.filter], m.sort, m.matched, safe(m.search)), " ID  タスク / 状態                         進捗・期日")
		count := m.listHeight()
		slots := m.listSlots()
		for i := 0; i < count; i++ {
			idx := m.offset + i
			if idx < len(slots) {
				if slots[idx] < 0 {
					lines = append(lines, "")
				} else {
					lines = append(lines, m.row(m.rows[slots[idx]]))
				}
			} else if i == 0 {
				if len(m.doc.Tasks) == 0 {
					lines = append(lines, "タスクはまだありません。a:追加")
				} else {
					hint := "一致するタスクがありません。"
					if m.search != "" {
						hint += " Esc:検索解除"
					}
					if m.filter != 0 {
						hint += " f:条件切替"
					}
					lines = append(lines, hint)
				}
			} else {
				lines = append(lines, "")
			}
		}
		lines = append(lines, strings.Repeat("-", m.width))
		details := m.summary()
		for i := 0; i < m.detailHeight(); i++ {
			if i < len(details) {
				lines = append(lines, details[i])
			} else {
				lines = append(lines, "")
			}
		}
		lines = append(lines, strings.Repeat("-", m.width), "通知: "+safe(m.notice), m.footer())
	}
	for i := range lines {
		lines[i] = fit(lines[i], m.width)
	}
	v := tea.NewView(strings.Join(lines, "\n"))
	v.AltScreen = true
	return v
}

func (m *Model) row(v engine.View) string {
	mark := " "
	if v.ID == m.selected {
		mark = ">"
	}
	check := "[ ]"
	if v.Status == "done" {
		check = "[x]"
	}
	prefix := fmt.Sprintf("%s %*d %s ", mark, m.idWidth, v.ID, check)
	tags := ""
	if v.Context {
		tags += " [context]"
	}
	if v.ProgressMode == "manual" {
		tags += " [manual]"
	}
	if v.Blocked {
		tags += " !dep"
	}
	progress := fmt.Sprintf("%3d%%", v.EffectiveProgress)
	date := ""
	if m.width >= 60 {
		date = " " + pad(v.DueDate, 10)
		if v.Status == "todo" && v.DueDate != "" && v.DueDate < m.today {
			date += "!"
		} else {
			date += " "
		}
	}
	bar := fmt.Sprintf("[%s%s] ", strings.Repeat("#", v.EffectiveProgress/10), strings.Repeat(".", 10-v.EffectiveProgress/10))
	if m.width < 80 || m.width-ansi.StringWidth(prefix+tags+bar+progress+date)-1 < 16 {
		bar = ""
	}
	suffix := tags + " " + bar + progress + date
	budget := max(1, m.width-ansi.StringWidth(prefix+suffix))
	indent := m.treePrefixes[v.ID]
	// Preserve the immediate branch guide even when deep ancestors do not fit.
	limit := max(3, budget-16)
	if ansi.StringWidth(indent) > limit {
		indent = ".." + ansi.Cut(indent, ansi.StringWidth(indent)-max(3, limit-2), ansi.StringWidth(indent))
	}
	branch := "  "
	if len(m.children[v.ID]) > 0 {
		branch = "v "
		if m.collapsed[v.ID] && !m.filtered() {
			branch = "> "
		}
	}
	title := safe(v.Title)
	// Selection takes priority over semantic colors so reverse video remains
	// uniform and readable across the whole selected row.
	colored := !m.noColor && v.ID != m.selected
	if colored {
		// Bright, saturated cool colors keep each level visible on dark backgrounds.
		palette := [...]string{"#00D7FF", "#00D7AF", "#5F9FFF", "#AF87FF"}
		color := "#FFFFFF"
		if v.Depth > 0 {
			color = palette[(v.Depth-1)%len(palette)]
		}
		style := lipgloss.NewStyle().Foreground(lipgloss.Color(color))
		title = style.Bold(len(m.children[v.ID]) > 0).Render(title)
		indent = style.Render(indent)
		branch = style.Render(branch)
		if v.Blocked && v.Status != "done" {
			tags = strings.Replace(tags, "!dep", lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Render("!dep"), 1)
		}
		if v.Status == "todo" && v.DueDate != "" {
			if v.DueDate < m.today {
				date = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render(date)
			} else if v.DueDate == m.today {
				date = lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Render(date)
			}
		}
	}
	suffix = tags + " " + bar + progress + date
	line := prefix + pad(indent+branch+title, budget) + suffix
	line = fit(line, m.width)
	if !m.noColor && v.ID == m.selected {
		return lipgloss.NewStyle().Reverse(true).Render(line)
	}
	return line
}
func (m *Model) summary() []string {
	if m.editing() {
		label := map[string]string{"add": "新規（親なし）", "child": fmt.Sprintf("#%d の子を追加", m.draftID), "title": "タイトル", "renumber": "番号変更", "start": "開始日", "due": "期日", "progress": "進捗", "deps": "依存ID", "search": "タイトル検索"}[m.mode]
		line := safe(m.input.text[:m.input.pos]) + "|" + safe(m.input.text[m.input.pos:])
		pos := ansi.StringWidth(safe(m.input.text[:m.input.pos]))
		left := max(0, pos-(m.width-4))
		line = ansi.Cut(line, left, left+m.width)
		hint := "Enter:保存  Esc:取消"
		switch m.mode {
		case "search":
			hint = "Enter:検索確定  Esc:開始前に戻す"
		case "start", "due":
			hint = "YYYY-MM-DD / today / +Nd / 空欄で解除"
		case "progress":
			hint = "0..100:手動固定 / auto:自動集計"
		case "renumber":
			hint = "新しい表示用ID（正の整数）。非表示のタスクも番号変更の対象"
		case "deps":
			hint = "カンマ区切りID / 空欄で解除"
		}
		latest := ""
		if v, ok := m.views[m.draftID]; ok && m.mode != "add" && m.mode != "search" {
			latest = fmt.Sprintf("最新: #%d %s / %d%% / %s -> %s", v.ID, safe(v.Title), v.EffectiveProgress, v.StartDate, v.DueDate)
		}
		if m.mode != "add" && m.mode != "search" && m.mode != "child" {
			label = fmt.Sprintf("%s (#%d)", label, m.draftID)
		}
		return []string{label, line, hint, latest}
	}
	if m.mode == "delete" {
		ids, e := engine.Removal(m.doc, m.draftID, true)
		message := fmt.Sprintf("#%d と子孫を含む計%d件（非表示の子孫も対象）", m.draftID, len(ids))
		if e != nil {
			message = e.Error()
		}
		return []string{"削除確認（取消が既定）", safe(message), "y:削除  n/Esc/Enter:取消  ?:ヘルプ"}
	}
	v, ok := m.views[m.selected]
	if !ok {
		return []string{"選択タスクなし"}
	}
	context := ""
	for _, r := range m.rows {
		if r.ID == v.ID && r.Context {
			context = " [context: 条件に不一致]"
		}
	}
	mode := ""
	if v.ProgressMode == "manual" {
		mode = " [manual]"
	}
	return []string{fmt.Sprintf("#%d %s%s", v.ID, safe(v.Title), context), fmt.Sprintf("状態:%s  進捗:%d%%%s  Enter:全文詳細", v.Status, v.EffectiveProgress, mode), fmt.Sprintf("開始:%s  期日:%s", v.StartDate, v.DueDate), m.relationship(v)}
}
func (m *Model) relationship(v engine.View) string {
	p := "なし"
	if v.ParentID != nil {
		p = fmt.Sprintf("#%d", *v.ParentID)
	}
	s := fmt.Sprintf("親:%s  依存:%d件", p, len(v.DependsOn))
	if v.Blocked {
		ids := []string{}
		for _, id := range v.BlockedBy {
			ids = append(ids, fmt.Sprintf("#%d", id))
		}
		s += " (blocked by " + strings.Join(ids, ", ") + ")"
	}
	return s
}
func (m *Model) footer() string {
	if m.busy {
		return "保存中: 更新キーは無効  Ctrl+C:成功後に終了"
	}
	if m.uncertain {
		return "r:再読込して確認  ?:ヘルプ・通知  q:終了（再送しません）"
	}
	if m.mode == "delete" {
		return "y:削除  n/Esc/Enter:取消  ?:ヘルプ"
	}
	if m.editing() {
		return "Enter:確定  Esc:取消  左右:文字移動"
	}
	if m.width < 60 {
		return "a:追加 /:検索 f:条件 Enter:詳細 ?:他 q:終了"
	}
	return "↑↓:移動 ←→:階層 Space:完了 a:追加 /:検索 f:条件 Enter:詳細 ?:他 q:終了"
}
func (m *Model) pageLines() []string {
	var s string
	if m.mode == "help" {
		s = "移動\n上下/j/k:選択  左右:折り畳み・親子移動\nJ/K:選択位置固定で下/上へスクロール（空行は飛ばす）\nEnter:全文詳細  Esc:戻る\n\n追加・編集\na:親なしで追加  A:子として追加\ni:番号変更 e:タイトル s:開始日 d:期日 p:進捗 l:依存先\nSpace:完了/未完了  +/-:進捗±10（手動固定）\nx/Delete:削除確認 y:削除 n/Esc/Enter:取消\n\n検索・表示\n/:タイトル検索 Enter:条件保持 Esc:開始前へ\n入力中は左右:文字移動、Space・j/k・?:入力文字\n検索適用後は左右:表示上の親子移動\nf:全件→未完了→今日まで→期限超過→着手可能\no:ID順/期日順/名前順 r:データ再読込\n一覧のEscは検索だけ解除（表示条件は維持）\n[manual]:手動固定 [context]:文脈用祖先 !dep:依存待ち\n期日の!:未完了で期限超過\n\n終了\nq/Ctrl+C:終了。入力中のqは文字\n保存中のCtrl+C:成功後に終了、失敗時は画面に戻る\n\n保存先\n" + safe(m.dir) + "\n\n最新の通知\n" + safe(m.notice)
	} else {
		v, ok := m.views[m.selected]
		if !ok {
			return []string{"対象タスクなし"}
		}
		s = fmt.Sprintf("#%d %s\n状態: %s\n進捗: %d%% [%s]（保存値: %d%%）\n開始: %s\n期日: %s\n作成: %s\n完了: %s\n%s\n", v.ID, safe(v.Title), v.Status, v.EffectiveProgress, v.ProgressMode, v.Progress, v.StartDate, v.DueDate, v.CreatedAt, v.CompletedAt, m.relationship(v))
		if v.ParentID != nil {
			s += "親タイトル: " + safe(m.views[*v.ParentID].Title) + "\n"
		}
		s += "\n依存先（全件）\n"
		for _, id := range v.DependsOn {
			dep := m.views[id]
			s += fmt.Sprintf("#%d [%s] %s\n", id, dep.Status, safe(dep.Title))
		}
		if len(v.DependsOn) == 0 {
			s += "なし\n"
		}
		s += "\n最新の通知\n" + safe(m.notice) + "\n\n保存先\n" + safe(m.dir)
	}
	return strings.Split(ansi.Hardwrap(s, max(1, m.width), true), "\n")
}
func (m *Model) maxPage() int { return max(0, len(m.pageLines())-max(1, m.height-2)) }
