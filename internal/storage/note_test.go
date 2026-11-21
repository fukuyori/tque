package storage

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"tque/internal/engine"
)

// schema3Tasks returns a schema 3 file: the current encoding of the legacy
// tasks with only the version number changed.
func schema3Tasks(t *testing.T) []byte {
	t.Helper()
	d, err := Decode([]byte(legacyTasks))
	if err != nil {
		t.Fatal(err)
	}
	b, err := encode(d)
	if err != nil {
		t.Fatal(err)
	}
	old := bytes.Replace(b, []byte(`"schema_version": 4`), []byte(`"schema_version": 3`), 1)
	if bytes.Equal(old, b) {
		t.Fatal("schema version not found")
	}
	return old
}

func noteRequest(id int64, note string) engine.Request {
	return engine.Request{Op: "edit", ID: id, Patch: engine.Patch{Note: &note}}
}

func TestSchema3MigratesOnFirstChange(t *testing.T) {
	s := &Store{Dir: t.TempDir()}
	source := schema3Tasks(t)
	path := filepath.Join(s.Dir, "tasks.json")
	if err := os.WriteFile(path, source, 0600); err != nil {
		t.Fatal(err)
	}
	backup := filepath.Join(s.Dir, "tasks.schema3.backup")
	d, err := s.Read(context.Background())
	if err != nil || len(d.Tasks) != 4 || *d.Tasks[3].ParentID != 8 {
		t.Fatal(d, err)
	}
	r, err := svc(s).Execute(context.Background(), nil, noteRequest(6, ""))
	if err != nil || r.Changed || !bytes.Equal(readBytes(t, path), source) {
		t.Fatal("no-op migrated", err)
	}
	if _, err := os.Stat(backup); !os.IsNotExist(err) {
		t.Fatal("no-op created backup", err)
	}
	r, err = svc(s).Execute(context.Background(), nil, noteRequest(6, "一行目\r\n\t二行目\n\n"))
	if err != nil || !r.Changed {
		t.Fatal(r, err)
	}
	if !bytes.Equal(readBytes(t, backup), source) || !bytes.Equal(readBytes(t, filepath.Join(s.Dir, "tasks.json.backup")), source) {
		t.Fatal("backup mismatch")
	}
	saved := readBytes(t, path)
	if !bytes.Contains(saved, []byte(`"schema_version": 4`)) || !bytes.Contains(saved, []byte(`"note": "一行目\n\t二行目"`)) {
		t.Fatal(string(saved))
	}
	loaded, err := s.Read(context.Background())
	if err != nil || loaded.Tasks[0].Note != "一行目\n\t二行目" || *loaded.Tasks[3].ParentID != 8 {
		t.Fatal(loaded, err)
	}
	// Removing the note drops the member again.
	if _, err := svc(s).Execute(context.Background(), nil, noteRequest(6, "")); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(readBytes(t, path), []byte(`"note"`)) {
		t.Fatal("empty note stored")
	}
	if !bytes.Equal(readBytes(t, backup), source) {
		t.Fatal("migration backup overwritten")
	}
}

func TestSchema3MigrationFailurePreservesSource(t *testing.T) {
	for _, stage := range []string{"tasks.schema3.backup:write", "tasks.json.backup:write", "tasks.json:replace"} {
		t.Run(stage, func(t *testing.T) {
			s := &Store{Dir: t.TempDir(), fault: func(s string) error {
				if s == stage {
					return os.ErrInvalid
				}
				return nil
			}}
			source := schema3Tasks(t)
			p := filepath.Join(s.Dir, "tasks.json")
			if err := os.WriteFile(p, source, 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := svc(s).Execute(context.Background(), nil, noteRequest(6, "memo")); err == nil {
				t.Fatal("fault ignored")
			}
			if !bytes.Equal(readBytes(t, p), source) {
				t.Fatal("source changed")
			}
		})
	}
}

func TestNoteStorageValidation(t *testing.T) {
	old := schema3Tasks(t)
	title := []byte(`"title": "six",`)
	withNote := func(b []byte, note string) []byte {
		out := bytes.Replace(b, title, append(append([]byte{}, title...), []byte(`"note": `+note+`,`)...), 1)
		if bytes.Equal(out, b) {
			t.Fatal("title not found")
		}
		return out
	}
	current := bytes.Replace(old, []byte(`"schema_version": 3`), []byte(`"schema_version": 4`), 1)
	if d, err := Decode(withNote(current, `"a\nb"`)); err != nil || d.Tasks[0].Note != "a\nb" {
		t.Fatal(d, err)
	}
	for name, b := range map[string][]byte{
		"note in schema 3":    withNote(old, `"a"`),
		"trailing whitespace": withNote(current, `"a\n"`),
		"carriage return":     withNote(current, `"a\r\nb"`),
		"control character":   withNote(current, `"a\u0007b"`),
		"null":                withNote(current, `null`),
	} {
		if _, err := Decode(b); kind(err) != "CorruptData" {
			t.Fatal(name, err)
		}
	}
	future := bytes.Replace(old, []byte(`"schema_version": 3`), []byte(`"schema_version": 5`), 1)
	if _, err := Decode(future); kind(err) != "UnsupportedSchema" {
		t.Fatal(err)
	}
}
