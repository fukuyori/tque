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
	"tque/internal/model"
)

func run(home string, args ...string) (string, error) {
	c := New(Options{Version: "test", Home: func() (string, error) { return home, nil }, Now: func() time.Time { return time.Date(2026, 10, 21, 0, 0, 0, 0, time.UTC) }})
	b := &bytes.Buffer{}
	c.SetOut(b)
	c.SetErr(b)
	c.SetArgs(args)
	e := c.ExecuteContext(context.Background())
	return b.String(), e
}
func TestCommands(t *testing.T) {
	h := t.TempDir()
	for _, args := range [][]string{{"add", "親", "--due", "today"}, {"add", "子", "--parent", "1", "-p", "60"}, {"edit", "1", "--dep", "2"}, {"done", "2"}, {"done", "1"}, {"undone", "1"}, {"undone", "2"}, {"edit", "2", "-p", "60"}} {
		if out, e := run(h, args...); e != nil {
			t.Fatal(args, out, e)
		}
	}
	out, e := run(h, "ls", "--all", "--json")
	if e != nil {
		t.Fatal(e)
	}
	var rows []map[string]any
	if e := json.Unmarshal([]byte(out), &rows); e != nil {
		t.Fatal(out, e)
	}
	if len(rows) != 2 || rows[0]["effective_progress"] != float64(60) || rows[0]["blocked"] != true {
		t.Fatal(rows)
	}
	if strings.Contains(out, ": null") {
		t.Fatal(out)
	}
	if _, e := run(h, "rm", "2"); ExitCode(e) != 1 {
		t.Fatal(e)
	}
	if out, e := run(h, "ls", "--tree", "--search", "子"); e != nil || !strings.Contains(out, "[context]") {
		t.Fatal(out, e)
	}
	if _, e := run(h, "rm", "1", "--cascade"); e != nil {
		t.Fatal(e)
	}
	if out, e := run(h, "add", "after"); e != nil || !strings.Contains(out, "#1") {
		t.Fatal(out, e)
	}
}
func TestParentCompletionAndManualProgress(t *testing.T) {
	h := t.TempDir()
	exec := func(args ...string) string {
		t.Helper()
		out, err := run(h, args...)
		if err != nil {
			t.Fatal(args, out, err)
		}
		return out
	}
	rows := func() []engine.View {
		t.Helper()
		var rows []engine.View
		if err := json.Unmarshal([]byte(exec("ls", "--all", "--json")), &rows); err != nil {
			t.Fatal(err)
		}
		return rows
	}
	exec("add", "parent")
	exec("add", "child", "--parent", "1", "-p", "100")
	for _, progress := range []string{"0", "100"} {
		exec("edit", "1", "-p", progress)
		parent := rows()[0]
		if parent.ProgressMode != "manual" || parent.Status != "todo" || parent.EffectiveProgress != parent.Progress {
			t.Fatal(parent)
		}
		before := exec("ls", "--all", "--json")
		if out, err := run(h, "done", "1"); err == nil || !strings.Contains(err.Error(), "child #2 is incomplete") {
			t.Fatal(out, err)
		}
		if after := exec("ls", "--all", "--json"); before != after {
			t.Fatal("rejected completion changed tasks")
		}
	}
	exec("edit", "2", "-p", "40")
	exec("edit", "1", "-p", "auto")
	if parent := rows()[0]; parent.ProgressMode != "auto" || parent.EffectiveProgress != 40 || parent.Status != "todo" {
		t.Fatal(parent)
	}
	exec("done", "2")
	if rows()[0].Status != "todo" {
		t.Fatal("parent completed automatically")
	}
	exec("edit", "1", "-p", "50")
	exec("done", "1")
	if parent := rows()[0]; parent.Status != "done" || parent.EffectiveProgress != 100 || parent.ProgressMode != "auto" {
		t.Fatal(parent)
	}
	// An incomplete descendant must also block completion of its ancestors.
	exec("add", "grandchild", "--parent", "2")
	exec("undone", "1")
	if out, err := run(h, "done", "1"); err == nil || !strings.Contains(err.Error(), "child #3 is incomplete") {
		t.Fatal(out, err)
	}
}

func TestOfflineHelpAndUsage(t *testing.T) {
	h := t.TempDir()
	if e := os.Mkdir(filepath.Join(h, ".tque"), 0700); e != nil {
		t.Fatal(e)
	}
	_ = os.WriteFile(filepath.Join(h, ".tque", "config.json"), []byte("broken"), 0600)
	for _, args := range [][]string{{"--help"}, {"--version"}, {"completion", "powershell"}, {"completion", "bash"}, {"completion", "zsh"}, {"completion", "fish"}} {
		if out, e := run(h, args...); e != nil || out == "" {
			t.Fatal(args, e)
		}
	}
	for _, args := range [][]string{{"ls", "--tree", "--json"}, {"edit", "1"}, {"add", "x", "--due", "none"}, {"edit", "1", "--dep", "none,2"}, {"--tui", "ls"}, {"ls", "--sort", "bad"}, {"ls", "--ready", "--blocked"}} {
		if _, e := run(h, args...); ExitCode(e) != 2 {
			t.Fatal(args, e)
		}
	}
	if _, e := run(h, "ls"); ExitCode(e) != 1 {
		t.Fatal(e)
	}
}

func TestTreeBranches(t *testing.T) {
	one, two := int64(1), int64(2)
	rows := []engine.View{{Task: model.Task{ID: 1}}, {Task: model.Task{ID: 2, ParentID: &one}, Depth: 1}, {Task: model.Task{ID: 3, ParentID: &two}, Depth: 2}, {Task: model.Task{ID: 4, ParentID: &one}, Depth: 1}}
	p := treePrefixes(rows)
	if p[2] != "├─ " || p[3] != "│  └─ " || p[4] != "└─ " {
		t.Fatal(p)
	}
}

func TestUnknownCommandsUseUsageExitCode(t *testing.T) {
	for _, args := range [][]string{{"lss"}, {"--tui", "extra"}} {
		if _, err := run(t.TempDir(), args...); ExitCode(err) != 2 {
			t.Fatalf("%v: exit=%d error=%v", args, ExitCode(err), err)
		}
	}
}

func TestReportCommittedResultWithCleanupError(t *testing.T) {
	result := engine.Result{ID: 7, Changed: true, Document: model.Document{Tasks: []model.Task{{ID: 7, Title: "saved"}}}}
	out := &bytes.Buffer{}
	cleanup := model.Err("StorageFailure", "saved; unlock failed")
	err := reportResult(out, engine.Request{Op: "add"}, result, cleanup)
	if ExitCode(err) != 1 || !strings.Contains(out.String(), "Created task #7: saved") || !strings.Contains(err.Error(), "saved; unlock failed") {
		t.Fatal(out.String(), err)
	}
	out.Reset()
	result.Changed = false
	err = reportResult(out, engine.Request{Op: "add"}, result, model.Err("CommitUncertain", "replace failed"))
	if ExitCode(err) != 3 || out.Len() != 0 {
		t.Fatal(out.String(), err)
	}
}

func TestTreeGroupSpacing(t *testing.T) {
	h := t.TempDir()
	for _, args := range [][]string{{"add", "RootA"}, {"add", "ChildA", "--parent", "1"}, {"add", "RootC"}, {"add", "RootD"}} {
		if out, err := run(h, args...); err != nil {
			t.Fatal(out, err)
		}
	}
	out, err := run(h, "ls", "--all", "--tree")
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	if len(lines) != 5 || lines[2] != "" || !strings.HasPrefix(lines[1], "└─ ") || !strings.Contains(lines[3], "RootC") || !strings.Contains(lines[4], "RootD") {
		t.Fatal(out)
	}
	out, err = run(h, "ls", "--all")
	if err != nil || strings.Contains(out, "\n\n") {
		t.Fatal(out, err)
	}
}

func TestNameSort(t *testing.T) {
	h := t.TempDir()
	for _, args := range [][]string{{"add", "Zebra"}, {"add", "Apple"}, {"add", "あ"}, {"add", "Apple"}, {"add", "子B", "--parent", "2"}, {"add", "子A", "--parent", "2"}} {
		if out, err := run(h, args...); err != nil {
			t.Fatal(out, err)
		}
	}
	out, err := run(h, "ls", "--sort", "name", "--json")
	if err != nil {
		t.Fatal(out, err)
	}
	var rows []engine.View
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		t.Fatal(err)
	}
	want := []int64{2, 4, 1, 3, 6, 5}
	for i, id := range want {
		if rows[i].ID != id {
			t.Fatalf("name order: %v", rows)
		}
	}
	out, err = run(h, "ls", "--sort", "name", "--tree")
	if err != nil {
		t.Fatal(out, err)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if !strings.Contains(lines[0], "Apple") || !strings.Contains(lines[1], "子A") || !strings.Contains(lines[2], "子B") || lines[3] != "" || !strings.Contains(lines[4], "Apple") {
		t.Fatal(out)
	}
}

func TestRenumberCLI(t *testing.T) {
	h := t.TempDir()
	for _, args := range [][]string{{"add", "first"}, {"add", "second"}, {"add", "third"}, {"add", "child", "--parent", "3", "--dep", "1,2"}, {"done", "2"}} {
		if out, err := run(h, args...); err != nil {
			t.Fatal(out, err)
		}
	}
	out, err := run(h, "renumber", "3", "1")
	if err != nil || !strings.Contains(out, "1→2, 2→3, 3→1") {
		t.Fatal(out, err)
	}
	out, err = run(h, "ls", "--all", "--json")
	if err != nil || strings.Contains(out, "internal_id") {
		t.Fatal(out, err)
	}
	var rows []engine.View
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		t.Fatal(err)
	}
	if rows[0].Title != "third" || rows[2].Status != "done" || *rows[3].ParentID != 1 || len(rows[3].DependsOn) != 2 || rows[3].DependsOn[0] != 2 || rows[3].DependsOn[1] != 3 {
		t.Fatal(out)
	}
	if out, err := run(h, "edit", "1", "--title", "changed", "--dep", "2"); err != nil {
		t.Fatal(out, err)
	}
	out, err = run(h, "ls", "--all", "--json")
	if err != nil || !strings.Contains(out, "changed") {
		t.Fatal(out, err)
	}
	for _, args := range [][]string{{"renumber", "1"}, {"renumber", "1", "0"}, {"renumber", "1", "-1"}, {"renumber", "x", "1"}} {
		if _, err := run(h, args...); ExitCode(err) != 2 {
			t.Fatal(args, err)
		}
	}
	if out, err := run(h, "renumber", "99", "1"); ExitCode(err) != 1 {
		t.Fatal(out, err)
	}
}
