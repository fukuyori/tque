package storage

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"tque/internal/engine"
	"tque/internal/model"
)

func addRequest(title string) engine.Request {
	return engine.Request{Op: "add", Patch: engine.Patch{Title: &title}}
}
func svc(s *Store) engine.Service {
	return engine.Service{Repo: s, Now: func() time.Time { return time.Date(2026, 10, 21, 0, 0, 0, 0, time.UTC) }}
}
func readBytes(t *testing.T, p string) []byte {
	t.Helper()
	b, e := os.ReadFile(p)
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func kind(e error) string {
	var v *model.Error
	if errors.As(e, &v) {
		return v.Kind
	}
	return ""
}

func TestTransactionBackupAndConflict(t *testing.T) {
	ctx := context.Background()
	s := &Store{Dir: filepath.Join(t.TempDir(), "未作成")}
	d, e := s.Read(ctx)
	if e != nil || len(d.Tasks) != 0 {
		t.Fatal(d, e)
	}
	if _, e := os.Stat(s.Dir); !os.IsNotExist(e) {
		t.Fatal("read created directory")
	}
	r, e := svc(s).Execute(ctx, nil, addRequest("初回"))
	if e != nil || r.Document.Revision != 1 {
		t.Fatal(r, e)
	}
	path := filepath.Join(s.Dir, "tasks.json")
	first := readBytes(t, path)
	if _, e := os.Stat(path + ".backup"); !os.IsNotExist(e) {
		t.Fatal("initial save created backup")
	}
	_, e = svc(s).Execute(ctx, nil, engine.Request{Op: "done", ID: 1})
	if e != nil {
		t.Fatal(e)
	}
	if !bytes.Equal(first, readBytes(t, path+".backup")) {
		t.Fatal("backup is not prior bytes")
	}
	second := readBytes(t, path)
	r, e = svc(s).Execute(ctx, nil, engine.Request{Op: "done", ID: 1})
	if e != nil || r.Changed {
		t.Fatal(r, e)
	}
	if !bytes.Equal(second, readBytes(t, path)) || !bytes.Equal(first, readBytes(t, path+".backup")) {
		t.Fatal("no-op changed files")
	}
	old := int64(1)
	_, e = svc(s).Execute(ctx, &old, addRequest("conflict"))
	if kind(e) != "RevisionConflict" {
		t.Fatal(e)
	}
	badTitle := "bad"
	_, e = svc(s).Execute(ctx, nil, engine.Request{Op: "edit", ID: 1, Patch: engine.Patch{Title: &badTitle, ParentSet: true, Parent: &old}})
	if kind(e) != "CycleDetected" || !bytes.Equal(second, readBytes(t, path)) {
		t.Fatal("partial update", e)
	}
}

func TestFailureStages(t *testing.T) {
	for _, stage := range []string{"tasks.json.backup:open", "tasks.json.backup:write", "tasks.json.backup:sync", "tasks.json.backup:close", "tasks.json.backup:replace", "tasks.json:open", "tasks.json:write", "tasks.json:sync", "tasks.json:close", "tasks.json:replace", "tasks.json:durability"} {
		t.Run(stage, func(t *testing.T) {
			s := &Store{Dir: t.TempDir()}
			ctx := context.Background()
			if _, e := svc(s).Execute(ctx, nil, addRequest("before")); e != nil {
				t.Fatal(e)
			}
			p := filepath.Join(s.Dir, "tasks.json")
			before := readBytes(t, p)
			s.fault = func(at string) error {
				if at == stage {
					return fmt.Errorf("injected %s", at)
				}
				return nil
			}
			_, e := svc(s).Execute(ctx, nil, addRequest("after"))
			if e == nil {
				t.Fatal("expected failure")
			}
			if strings.HasSuffix(stage, ":durability") {
				if kind(e) != "CommitUncertain" {
					t.Fatal(e)
				}
				d, e := Decode(readBytes(t, p))
				if e != nil || len(d.Tasks) != 2 {
					t.Fatal(d, e)
				}
			} else if !bytes.Equal(before, readBytes(t, p)) {
				t.Fatal("failed write damaged main")
			}
		})
	}
}

func TestCorruptionIsPreserved(t *testing.T) {
	s := &Store{Dir: t.TempDir()}
	ctx := context.Background()
	_, e := svc(s).Execute(ctx, nil, addRequest("valid"))
	if e != nil {
		t.Fatal(e)
	}
	p := filepath.Join(s.Dir, "tasks.json")
	valid := readBytes(t, p)
	for _, b := range [][]byte{[]byte(`{}`), []byte(`{"schema_version":2,"revision":0,"next_id":1,"tasks":[]}`), bytes.Replace(valid, []byte(`"progress": 0,`), nil, 1), bytes.Replace(valid, []byte(`"revision": 1`), []byte(`"revision": 1,"revision":2`), 1), append(append([]byte{}, valid...), []byte(`{}`)...), bytes.Replace(valid, []byte(`"status": "todo"`), []byte(`"status": null`), 1)} {
		_ = os.WriteFile(p, b, 0600)
		_ = os.WriteFile(p+".tmp", []byte("leave me"), 0600)
		if _, e := svc(s).Execute(ctx, nil, addRequest("no")); e == nil {
			t.Fatal("accepted corrupt JSON", string(b))
		}
		if !bytes.Equal(b, readBytes(t, p)) || string(readBytes(t, p+".tmp")) != "leave me" {
			t.Fatal("corruption overwritten")
		}
	}
}

func TestProcessHelper(t *testing.T) {
	dir := os.Getenv("TQUE_TEST_DIR")
	if dir == "" {
		return
	}
	s := &Store{Dir: dir}
	if os.Getenv("TQUE_TEST_HOLD") == "1" {
		release, e := s.acquire(context.Background())
		if e != nil {
			t.Fatal(e)
		}
		defer release()
		fmt.Println("LOCKED")
		for {
			time.Sleep(time.Second)
		}
	}
	for i := 0; i < 8; i++ {
		if _, e := svc(s).Execute(context.Background(), nil, addRequest("parallel")); e != nil {
			t.Fatal(e)
		}
	}
}

func TestMultipleProcesses(t *testing.T) {
	s := &Store{Dir: t.TempDir()}
	children := []*exec.Cmd{}
	outputs := []*bytes.Buffer{}
	for i := 0; i < 4; i++ {
		c := exec.Command(os.Args[0], "-test.run=^TestProcessHelper$")
		c.Env = append(os.Environ(), "TQUE_TEST_DIR="+s.Dir)
		b := &bytes.Buffer{}
		c.Stdout = b
		c.Stderr = b
		if e := c.Start(); e != nil {
			t.Fatal(e)
		}
		children = append(children, c)
		outputs = append(outputs, b)
	}
	for i, c := range children {
		if e := c.Wait(); e != nil {
			t.Fatal(e, outputs[i].String())
		}
	}
	d, e := s.Read(context.Background())
	if e != nil || len(d.Tasks) != 32 || d.Revision != 32 {
		t.Fatal(d, e)
	}
	seen := map[int64]bool{}
	for _, task := range d.Tasks {
		if task.ID < 1 || task.ID > 32 || seen[task.ID] {
			t.Fatal("concurrent additions produced unexpected ID", task.ID)
		}
		seen[task.ID] = true
	}
}

func TestSchemaVersion1IsUnsupported(t *testing.T) {
	v1 := []byte(`{"schema_version":1,"revision":1,"next_id":2,"tasks":[{"id":1,"title":"old","status":"todo","progress":0,"progress_mode":"auto","created_at":"2026-10-21T00:00:00Z"}]}`)
	if _, e := Decode(v1); kind(e) != "UnsupportedSchema" {
		t.Fatal(e)
	}
}

func TestSavedDocumentHasNoIDCounter(t *testing.T) {
	s := &Store{Dir: t.TempDir()}
	if _, e := svc(s).Execute(context.Background(), nil, addRequest("task")); e != nil {
		t.Fatal(e)
	}
	b := readBytes(t, filepath.Join(s.Dir, "tasks.json"))
	if bytes.Contains(b, []byte("next_id")) || !bytes.Contains(b, []byte(`"schema_version": 2`)) {
		t.Fatal(string(b))
	}
}

func TestLockCancellation(t *testing.T) {
	s := &Store{Dir: t.TempDir()}
	release, e := s.acquire(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	defer release()
	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	start := time.Now()
	if _, e := s.Read(ctx); kind(e) != "LockTimeout" {
		t.Fatal(e)
	}
	if time.Since(start) > time.Second {
		t.Fatal("cancellation ignored")
	}
}

func TestLockReleasedAfterProcessDeath(t *testing.T) {
	s := &Store{Dir: t.TempDir()}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestProcessHelper$")
	c.Env = append(os.Environ(), "TQUE_TEST_DIR="+s.Dir, "TQUE_TEST_HOLD=1")
	pipe, e := c.StdoutPipe()
	if e != nil {
		t.Fatal(e)
	}
	if e = c.Start(); e != nil {
		t.Fatal(e)
	}
	defer func() {
		if c.ProcessState == nil {
			_ = c.Process.Kill()
			_ = c.Wait()
		}
	}()
	scanner := bufio.NewScanner(pipe)
	if !scanner.Scan() || scanner.Text() != "LOCKED" {
		t.Fatal("child failed to acquire lock", scanner.Err())
	}
	short, stop := context.WithTimeout(context.Background(), 60*time.Millisecond)
	defer stop()
	if _, e := s.Read(short); kind(e) != "LockTimeout" {
		t.Fatal("second process ignored lock", e)
	}
	if e := c.Process.Kill(); e != nil {
		t.Fatal(e)
	}
	_ = c.Wait()
	if _, e := svc(s).Execute(context.Background(), nil, addRequest("after crash")); e != nil {
		t.Fatal("OS did not release lock", e)
	}
}

func TestNoOpEmptyDependenciesAndExhaustion(t *testing.T) {
	s := &Store{Dir: t.TempDir()}
	ctx := context.Background()
	if _, e := svc(s).Execute(ctx, nil, addRequest("task")); e != nil {
		t.Fatal(e)
	}
	r, e := svc(s).Execute(ctx, nil, engine.Request{Op: "edit", ID: 1, Patch: engine.Patch{DepsSet: true}})
	if e != nil || r.Changed {
		t.Fatal("empty dependency replacement changed revision", r, e)
	}
	stale := int64(0)
	r, e = svc(s).Execute(ctx, &stale, addRequest("conflict"))
	if e == nil || r.Changed {
		t.Fatal("conflict falsely marked changed", r, e)
	}
	d := r.Document
	d.Revision = 9223372036854775807
	b, _ := json.Marshal(d)
	p := filepath.Join(s.Dir, "tasks.json")
	_ = os.WriteFile(p, b, 0600)
	if _, e := svc(s).Execute(ctx, nil, addRequest("overflow")); e == nil {
		t.Fatal("revision overflow accepted")
	}
	if !bytes.Equal(b, readBytes(t, p)) {
		t.Fatal("overflow changed main")
	}
}

func FuzzDecode(f *testing.F) {
	f.Add([]byte(`{"schema_version":2,"revision":0,"tasks":[]}`))
	f.Add([]byte(`{"schema_version":2,"revision":null,"tasks":[]}`))
	f.Add([]byte(`{"schema_version":2,"revision":1,"tasks":[{"id":1,"title":"seed","status":"todo","progress":0,"progress_mode":"auto","created_at":"2026-10-21T00:00:00Z","due_date":"2026-10-22"}]}`))
	f.Add([]byte(`{"schema_version":2,"revision":1,"tasks":[{"id":1,"title":"seed","status":"todo","progress":0,"progress_mode":"auto","created_at":"2026-10-21T00:00:00Z","due_date":"+00000001d"}]}`))
	f.Fuzz(func(t *testing.T, b []byte) {
		d, e := Decode(b)
		if e == nil {
			encoded, err := json.Marshal(d)
			if err != nil {
				t.Fatal(err)
			}
			roundTrip, err := Decode(encoded)
			if err != nil {
				t.Fatal("re-encoded document rejected", err)
			}
			encodedAgain, err := json.Marshal(roundTrip)
			if err != nil || !bytes.Equal(encoded, encodedAgain) {
				t.Fatal("round trip changed document", err)
			}
			for _, task := range d.Tasks {
				for _, date := range []string{task.StartDate, task.DueDate} {
					if date != "" {
						parsed, err := time.Parse(time.DateOnly, date)
						if err != nil || parsed.Year() < 1 || parsed.Format(time.DateOnly) != date {
							t.Fatalf("noncanonical stored date %q", date)
						}
					}
				}
			}
		}
	})
}

func TestStoredDatesMustBeAbsolute(t *testing.T) {
	for _, field := range []string{"start_date", "due_date"} {
		for _, date := range []string{"+00000001d", "today", "0000-01-01", "2026-02-30", "2026-1-01"} {
			input := fmt.Sprintf(`{"schema_version":2,"revision":1,"tasks":[{"id":1,"title":"test","status":"todo","progress":0,"progress_mode":"auto","created_at":"2026-10-21T00:00:00Z","%s":"%s"}]}`, field, date)
			if _, err := Decode([]byte(input)); kind(err) != "CorruptData" {
				t.Fatalf("%s=%s: %v", field, date, err)
			}
		}
	}
}

func TestReplacementFailurePreservesMainAndTemporaryFile(t *testing.T) {
	s := &Store{Dir: t.TempDir()}
	ctx := context.Background()
	if _, err := svc(s).Execute(ctx, nil, addRequest("before")); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(s.Dir, "tasks.json")
	before := readBytes(t, path)
	s.replaceFile = func(from, to string) error {
		if to == path {
			return fmt.Errorf("OS replacement failure")
		}
		return replace(from, to)
	}
	result, err := svc(s).Execute(ctx, nil, addRequest("after"))
	if kind(err) != "CommitUncertain" || result.Changed {
		t.Fatal(result, err)
	}
	if !bytes.Equal(before, readBytes(t, path)) {
		t.Fatal("main changed")
	}
	candidate, err := Decode(readBytes(t, path+".tmp"))
	if err != nil || len(candidate.Tasks) != 2 {
		t.Fatal(candidate, err)
	}
}

func TestSavedResultSurvivesUnlockFailure(t *testing.T) {
	s := &Store{Dir: t.TempDir(), fault: func(stage string) error {
		if stage == "unlock" {
			return fmt.Errorf("injected unlock failure")
		}
		return nil
	}}
	result, err := svc(s).Execute(context.Background(), nil, addRequest("saved"))
	if kind(err) != "StorageFailure" || !result.Changed || result.ID != 1 || !strings.Contains(err.Error(), "saved; unlock failed") {
		t.Fatal(result, err)
	}
	s.fault = nil
	data, err := s.Read(context.Background())
	if err != nil || len(data.Tasks) != 1 || data.Tasks[0].ID != 1 {
		t.Fatal(data, err)
	}
}
