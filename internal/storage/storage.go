package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"time"
	"tque/internal/engine"
	"tque/internal/model"
	"tque/internal/strictjson"
)

type Store struct {
	Dir         string
	fault       func(string) error
	replaceFile func(string, string) error // Test seam for OS replacement failures.
}

func (s *Store) fail(stage string) error {
	if s.fault != nil {
		return s.fault(stage)
	}
	return nil
}

func Decode(data []byte) (model.Document, error) {
	d := model.Document{}
	var raw map[string]json.RawMessage
	bad := func(e error) (model.Document, error) { return d, model.Err("CorruptData", "%v", e) }
	if err := strictjson.Decode(data, &raw); err != nil {
		return bad(err)
	}
	if err := strictjson.Required(data, "schema_version"); err != nil {
		return bad(err)
	}
	var schema int
	if err := json.Unmarshal(raw["schema_version"], &schema); err != nil {
		return bad(err)
	}
	if schema != model.SchemaVersion && schema != 2 {
		return d, model.Err("UnsupportedSchema", "schema_version=%d", schema)
	}
	if err := strictjson.Required(data, "revision", "tasks"); err != nil {
		return bad(err)
	}
	if err := strictjson.Allowed(data, "schema_version", "revision", "tasks"); err != nil {
		return bad(err)
	}
	var tasks []json.RawMessage
	if err := json.Unmarshal(raw["tasks"], &tasks); err != nil {
		return bad(err)
	}
	for i, t := range tasks {
		allowed := []string{"id", "title", "status", "progress", "progress_mode", "created_at", "completed_at", "start_date", "due_date"}
		if schema == 2 {
			allowed = append(allowed, "parent_id", "depends_on")
		} else {
			allowed = append(allowed, "internal_id", "parent_internal_id", "depends_on_internal_ids")
		}
		if err := strictjson.Allowed(t, allowed...); err != nil {
			return bad(err)
		}
		if err := strictjson.Required(t, "id", "title", "status", "progress", "progress_mode", "created_at"); err != nil {
			return bad(fmt.Errorf("tasks[%d]: %w", i, err))
		}
		if schema == model.SchemaVersion {
			if err := strictjson.Required(t, "internal_id"); err != nil {
				return bad(err)
			}
		}
	}
	if schema == 2 {
		if err := strictjson.Decode(data, &d); err != nil {
			return bad(err)
		}
		d.SchemaVersion = model.SchemaVersion
		// Deterministic identities keep read-only v2 loads stable until first save.
		for i := range d.Tasks {
			d.Tasks[i].InternalID = fmt.Sprintf("legacy-%d", d.Tasks[i].ID)
		}
	} else {
		var err error
		d, err = decodeCurrent(data)
		if err != nil {
			return bad(err)
		}
	}
	if err := engine.Validate(d); err != nil {
		return bad(err)
	}
	return d, nil
}

func (s *Store) read() (model.Document, []byte, error) {
	path := filepath.Join(s.Dir, "tasks.json")
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return model.Empty(), nil, nil
	}
	if err != nil {
		return model.Document{}, nil, model.Err("StorageFailure", "read %s: %v", path, err)
	}
	d, err := Decode(b)
	if err != nil {
		return d, nil, fmt.Errorf("%s: %w", path, err)
	}
	return d, b, nil
}

func (s *Store) acquire(ctx context.Context) (func() error, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	f, err := os.OpenFile(filepath.Join(s.Dir, "lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, model.Err("StorageFailure", "open lock in %s: %v", s.Dir, err)
	}
	for {
		if err = ctx.Err(); err != nil {
			_ = f.Close()
			return nil, model.Err("LockTimeout", "%s: %v", s.Dir, err)
		}
		ok, e := tryLock(f)
		if e != nil {
			_ = f.Close()
			return nil, model.Err("StorageFailure", "lock %s: %v", s.Dir, e)
		}
		if ok {
			return func() error { return errors.Join(unlock(f), f.Close(), s.fail("unlock")) }, nil
		}
		timer := time.NewTimer(25 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
		case <-timer.C:
		}
	}
}

func (s *Store) Read(ctx context.Context) (d model.Document, err error) {
	if err := ctx.Err(); err != nil {
		return d, err
	}
	info, e := os.Stat(s.Dir)
	if os.IsNotExist(e) {
		return model.Empty(), nil
	}
	if e != nil {
		return d, model.Err("StorageFailure", "%s: %v", s.Dir, e)
	}
	if !info.IsDir() {
		return d, model.Err("StorageFailure", "%s is not a directory", s.Dir)
	}
	release, e := s.acquire(ctx)
	if e != nil {
		return d, e
	}
	defer func() {
		if e := release(); e != nil {
			err = errors.Join(err, model.Err("StorageFailure", "read completed; unlock %s: %v", s.Dir, e))
		}
	}()
	d, _, err = s.read()
	return
}

func (s *Store) Update(ctx context.Context, expected *int64, mutate func(*model.Document) error) (d model.Document, err error) {
	if err := ctx.Err(); err != nil {
		return d, err
	}
	if e := os.MkdirAll(s.Dir, 0700); e != nil {
		return d, model.Err("StorageFailure", "mkdir %s: %v", s.Dir, e)
	}
	release, e := s.acquire(ctx)
	if e != nil {
		return d, e
	}
	saved := false
	defer func() {
		if e := release(); e != nil {
			label := "unlock failed"
			if saved {
				label = "saved; unlock failed"
			}
			err = errors.Join(err, model.Err("StorageFailure", "%s: %s: %v", s.Dir, label, e))
		}
	}()
	d, raw, e := s.read()
	if e != nil {
		return d, e
	}
	if expected != nil && *expected != d.Revision {
		return d, model.Err("RevisionConflict", "expected %d, found %d", *expected, d.Revision)
	}
	next := d.Clone()
	if e := mutate(&next); e != nil {
		return d, e
	}
	if next.SchemaVersion != d.SchemaVersion || next.Revision != d.Revision {
		return d, model.Err("InvalidInput", "transaction changed protected fields")
	}
	if e := engine.Validate(next); e != nil {
		return d, e
	}
	beforeJSON, beforeErr := encode(d)
	if beforeErr != nil {
		return d, beforeErr
	}
	afterJSON, marshalErr := encode(next)
	if marshalErr != nil {
		return d, marshalErr
	}
	if bytes.Equal(beforeJSON, afterJSON) {
		return d, nil
	}
	if d.Revision == math.MaxInt64 {
		return d, model.Err("InvalidInput", "revision exhausted")
	}
	next.Revision++
	b, e := encode(next)
	if e != nil {
		return d, e
	}
	b = append(b, '\n')
	if e := ctx.Err(); e != nil {
		return d, e
	}
	if raw != nil {
		var header struct {
			SchemaVersion int `json:"schema_version"`
		}
		if json.Unmarshal(raw, &header) == nil && header.SchemaVersion == 2 {
			// Retain the migration source separately from the rolling backup.
			path := filepath.Join(s.Dir, "tasks.schema2.backup")
			if existing, err := os.ReadFile(path); err == nil {
				if !bytes.Equal(existing, raw) {
					return d, model.Err("StorageFailure", "migration backup already exists with different contents: %s", path)
				}
			} else if !os.IsNotExist(err) {
				return d, err
			} else if _, err := s.atomic("tasks.schema2.backup", raw); err != nil {
				return d, model.Err("StorageFailure", "migration backup failed: %v", err)
			}
		}
		if _, e := s.atomic("tasks.json.backup", raw); e != nil {
			return d, model.Err("StorageFailure", "backup failed; tasks.json unchanged: %v", e)
		}
	}
	if e := ctx.Err(); e != nil {
		return d, e
	}
	uncertain, e := s.atomic("tasks.json", b)
	if e != nil {
		kind := "StorageFailure"
		if uncertain {
			kind = "CommitUncertain"
		}
		return d, model.Err(kind, "%s: %v", filepath.Join(s.Dir, "tasks.json"), e)
	}
	saved = true
	return next, nil
}

func (s *Store) atomic(name string, b []byte) (bool, error) {
	path := filepath.Join(s.Dir, name)
	tmp := path + ".tmp"
	if st, e := os.Lstat(tmp); e == nil && !st.Mode().IsRegular() {
		return false, fmt.Errorf("temporary path is not a regular file: %s", tmp)
	} else if e != nil && !os.IsNotExist(e) {
		return false, e
	}
	if e := s.fail(name + ":open"); e != nil {
		return false, e
	}
	f, e := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if e != nil {
		return false, e
	}
	clean := func(err error) error {
		if e := os.Remove(tmp); e != nil && !os.IsNotExist(e) {
			return errors.Join(err, fmt.Errorf("temporary cleanup: %w", e))
		}
		return err
	}
	e = s.fail(name + ":write")
	if e == nil {
		_, e = f.Write(b)
	}
	if e == nil {
		e = s.fail(name + ":sync")
	}
	if e == nil {
		e = f.Sync()
	}
	closeErr := f.Close()
	e = errors.Join(e, closeErr, s.fail(name+":close"))
	if e != nil {
		return false, clean(e)
	}
	if e = s.fail(name + ":replace"); e != nil {
		return false, clean(e)
	}
	replaceFn := s.replaceFile
	if replaceFn == nil {
		replaceFn = replace
	}
	if e = replaceFn(tmp, path); e != nil {
		return true, e
	} // Conservative: do not retry or remove tmp.
	if e = s.fail(name + ":durability"); e == nil {
		e = syncDirectory(s.Dir)
	}
	if e != nil {
		return true, e
	}
	return false, nil
}
