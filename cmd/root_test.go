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
	for _, args := range [][]string{{"add", "親", "--due", "today"}, {"add", "子", "--parent", "1", "-p", "60"}, {"edit", "1", "--dep", "2"}, {"done", "1"}, {"undone", "1"}} {
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
