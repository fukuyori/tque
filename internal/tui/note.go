package tui

import (
	tea "charm.land/bubbletea/v2"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"tque/internal/engine"
)

// noteMsg reports that the external editor started by startNote has exited.
type noteMsg struct{ err error }

// editorCommand opens path in $EDITOR, or in notepad (Windows) or vi when it
// is not set. A value that is not itself an executable is split at spaces so
// that arguments such as "code --wait" work.
func editorCommand(path string) *exec.Cmd {
	editor := strings.TrimSpace(os.Getenv("EDITOR"))
	if editor == "" {
		editor = "vi"
		if runtime.GOOS == "windows" {
			editor = "notepad"
		}
	}
	args := []string{editor}
	if _, e := exec.LookPath(editor); e != nil {
		args = strings.Fields(editor)
	}
	return exec.Command(args[0], append(args[1:], path)...)
}

// startNote writes the note to a temporary file and suspends the TUI while
// the external editor runs. Keys and external reloads stay blocked (busy)
// until noteMsg arrives, so the revision recorded here detects a conflict.
func (m *Model) startNote(v engine.View) tea.Cmd {
	f, e := os.CreateTemp("", "tque-note-*.txt")
	if e == nil {
		_, e = f.WriteString(v.Note)
		if ce := f.Close(); e == nil {
			e = ce
		}
		if e != nil {
			_ = os.Remove(f.Name())
		}
	}
	if e != nil {
		m.warn("備考の一時ファイルを作成できません: " + e.Error())
		return nil
	}
	m.busy = true
	m.draftID = v.ID
	m.revision = m.doc.Revision
	m.noteFile = f.Name()
	m.noteBefore = v.Note
	m.notice = fmt.Sprintf("#%d の備考をエディタで編集中。保存してエディタを閉じると反映します", v.ID)
	return tea.ExecProcess(editorCommand(m.noteFile), func(err error) tea.Msg { return noteMsg{err: err} })
}

// finishNote saves the edited note. The temporary file is removed only when
// its text is no longer needed; a rejected text stays there for the user.
func (m *Model) finishNote(msg noteMsg) tea.Cmd {
	m.busy = false
	path := m.noteFile
	if path == "" {
		return nil
	}
	discard := func() {
		_ = os.Remove(path)
		m.noteFile = ""
	}
	if msg.err != nil {
		discard()
		m.warn("エディタを実行できません: " + msg.err.Error())
		return nil
	}
	b, e := os.ReadFile(path)
	if e != nil {
		discard()
		m.warn("備考の一時ファイルを読み込めません: " + e.Error())
		return nil
	}
	text, e := engine.Note(string(b))
	if e != nil {
		m.noteFile = ""
		m.warn(e.Error() + " / 編集内容: " + path)
		return nil
	}
	if text == m.noteBefore {
		discard()
		m.notice = fmt.Sprintf("#%d 備考は変更なし", m.draftID)
		return nil
	}
	return m.save(engine.Request{Op: "edit", ID: m.draftID, Patch: engine.Patch{Note: &text}})
}

// noteSaved settles the temporary file after the save result: it is removed
// once the note is stored and otherwise kept, with its path in the notice.
func (m *Model) noteSaved(stored bool) {
	if m.noteFile == "" {
		return
	}
	if stored {
		_ = os.Remove(m.noteFile)
	} else {
		m.notice += " / 編集内容: " + m.noteFile
	}
	m.noteFile = ""
}

// noteText prepares a note for display: tabs become spaces and line feeds are
// kept, unlike safe which replaces every control character.
func noteText(s string) string {
	lines := strings.Split(strings.ReplaceAll(s, "\t", "    "), "\n")
	for i := range lines {
		lines[i] = safe(lines[i])
	}
	return strings.Join(lines, "\n")
}
