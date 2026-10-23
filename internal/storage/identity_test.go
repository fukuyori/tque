package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"tque/internal/engine"
)

const legacyTasks = `{"schema_version":2,"revision":5,"tasks":[{"id":6,"title":"six","status":"todo","progress":0,"progress_mode":"auto","created_at":"2026-10-21T00:00:00Z"},{"id":7,"title":"seven","status":"todo","progress":0,"progress_mode":"auto","created_at":"2026-10-21T00:00:00Z"},{"id":8,"title":"eight","status":"todo","progress":0,"progress_mode":"auto","created_at":"2026-10-21T00:00:00Z"},{"id":9,"title":"child","status":"todo","progress":0,"progress_mode":"auto","created_at":"2026-10-21T00:00:00Z","parent_id":8,"depends_on":[6,7]}]}`

func TestMigrationAndStableReferences(t *testing.T) {
	s := &Store{Dir: t.TempDir()}
	path := filepath.Join(s.Dir, "tasks.json")
	if err := os.WriteFile(path, []byte(legacyTasks), 0600); err != nil {
		t.Fatal(err)
	}
	d, err := s.Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	d2, err := s.Read(context.Background())
	if err != nil || !reflect.DeepEqual(d, d2) {
		t.Fatal("unstable migration", err)
	}
	if !bytes.Equal(readBytes(t, path), []byte(legacyTasks)) {
		t.Fatal("read modified source")
	}
	before, err := encode(d)
	if err != nil {
		t.Fatal(err)
	}
	r, err := svc(s).Execute(context.Background(), &d.Revision, engine.Request{Op: "renumber", ID: 8, NewID: 6})
	if err != nil || !r.Changed || r.ID != 6 || len(r.Renumbered) != 3 {
		t.Fatal(r, err)
	}
	if !bytes.Equal(readBytes(t, filepath.Join(s.Dir, "tasks.schema2.backup")), []byte(legacyTasks)) {
		t.Fatal("migration backup mismatch")
	}
	if !bytes.Equal(readBytes(t, filepath.Join(s.Dir, "tasks.json.backup")), []byte(legacyTasks)) {
		t.Fatal("rolling backup mismatch")
	}
	var old, next storedDocument
	if err := json.Unmarshal(before, &old); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(readBytes(t, path), &next); err != nil {
		t.Fatal(err)
	}
	for i, task := range next.Tasks {
		if task.Key != old.Tasks[i].Key || task.ParentKey != old.Tasks[i].ParentKey || !reflect.DeepEqual(task.DependencyKeys, old.Tasks[i].DependencyKeys) {
			t.Fatal("stored relationships changed")
		}
	}
	loaded, err := s.Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if *loaded.Tasks[3].ParentID != 6 || !reflect.DeepEqual(loaded.Tasks[3].DependsOn, []int64{7, 8}) {
		t.Fatal(loaded)
	}
	stale := d.Revision
	if _, err := svc(s).Execute(context.Background(), &stale, engine.Request{Op: "renumber", ID: 6, NewID: 8}); kind(err) != "RevisionConflict" {
		t.Fatal(err)
	}
	unchanged := readBytes(t, path)
	r, err = svc(s).Execute(context.Background(), nil, engine.Request{Op: "renumber", ID: 6, NewID: 6})
	if err != nil || r.Changed || !bytes.Equal(unchanged, readBytes(t, path)) {
		t.Fatal("no-op rewrote data", err)
	}
	if _, err := svc(s).Execute(context.Background(), nil, engine.Request{Op: "done", ID: 6}); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(readBytes(t, filepath.Join(s.Dir, "tasks.schema2.backup")), []byte(legacyTasks)) {
		t.Fatal("migration backup overwritten")
	}
}

func TestMigrationFailurePreservesSource(t *testing.T) {
	for _, stage := range []string{"tasks.schema2.backup:write", "tasks.json.backup:write", "tasks.json:replace"} {
		t.Run(stage, func(t *testing.T) {
			s := &Store{Dir: t.TempDir(), fault: func(s string) error {
				if s == stage {
					return fmt.Errorf("injected")
				}
				return nil
			}}
			p := filepath.Join(s.Dir, "tasks.json")
			if err := os.WriteFile(p, []byte(legacyTasks), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := svc(s).Execute(context.Background(), nil, engine.Request{Op: "renumber", ID: 8, NewID: 6}); err == nil {
				t.Fatal("fault ignored")
			}
			if !bytes.Equal(readBytes(t, p), []byte(legacyTasks)) {
				t.Fatal("source changed")
			}
		})
	}
}

func TestRejectInvalidInternalReferences(t *testing.T) {
	d, err := Decode([]byte(legacyTasks))
	if err != nil {
		t.Fatal(err)
	}
	b, err := encode(d)
	if err != nil {
		t.Fatal(err)
	}
	for _, corrupt := range [][]byte{
		bytes.Replace(b, []byte(`"internal_id": "legacy-7"`), []byte(`"internal_id": "legacy-6"`), 1),
		bytes.Replace(b, []byte(`"internal_id": "legacy-7"`), []byte(`"internal_id": ""`), 1),
		bytes.Replace(b, []byte(`"parent_internal_id": "legacy-8"`), []byte(`"parent_internal_id": "missing"`), 1),
		bytes.Replace(b, []byte(`"parent_internal_id": "legacy-8"`), []byte(`"parent_id": 8`), 1),
	} {
		if _, err := Decode(corrupt); err == nil {
			t.Fatal("invalid identity accepted", string(corrupt))
		}
	}
}

func TestReusedDisplayIDHasNewIdentity(t *testing.T) {
	s := &Store{Dir: t.TempDir()}
	r, err := svc(s).Execute(context.Background(), nil, addRequest("first"))
	if err != nil {
		t.Fatal(err)
	}
	key := r.Document.Tasks[0].InternalID
	if _, err := svc(s).Execute(context.Background(), nil, engine.Request{Op: "rm", ID: 1}); err != nil {
		t.Fatal(err)
	}
	r, err = svc(s).Execute(context.Background(), nil, addRequest("second"))
	if err != nil || r.ID != 1 || r.Document.Tasks[0].InternalID == key {
		t.Fatal("reused identity", r, err)
	}
}

func TestMigrationNoOpAndExistingBackup(t *testing.T) {
	s := &Store{Dir: t.TempDir()}
	p := filepath.Join(s.Dir, "tasks.json")
	if err := os.WriteFile(p, []byte(legacyTasks), 0600); err != nil {
		t.Fatal(err)
	}
	r, err := svc(s).Execute(context.Background(), nil, engine.Request{Op: "renumber", ID: 8, NewID: 8})
	if err != nil || r.Changed || !bytes.Equal(readBytes(t, p), []byte(legacyTasks)) {
		t.Fatal("no-op migrated", err)
	}
	backup := filepath.Join(s.Dir, "tasks.schema2.backup")
	if _, err := os.Stat(backup); !os.IsNotExist(err) {
		t.Fatal("no-op created backup", err)
	}
	if err := os.WriteFile(backup, []byte("existing backup"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := svc(s).Execute(context.Background(), nil, engine.Request{Op: "renumber", ID: 8, NewID: 6}); err == nil {
		t.Fatal("overwrote different migration backup")
	}
	if !bytes.Equal(readBytes(t, p), []byte(legacyTasks)) || string(readBytes(t, backup)) != "existing backup" {
		t.Fatal("data changed on backup conflict")
	}
}
