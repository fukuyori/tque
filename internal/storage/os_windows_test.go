package storage

import (
	"bytes"
	"context"
	"golang.org/x/sys/windows"
	"path/filepath"
	"testing"
)

func TestRestrictedWindowsHandleKeepsMain(t *testing.T) {
	s := &Store{Dir: t.TempDir()}
	ctx := context.Background()
	if _, e := svc(s).Execute(ctx, nil, addRequest("original")); e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(s.Dir, "tasks.json")
	before := readBytes(t, path)
	p, e := windows.UTF16PtrFromString(path)
	if e != nil {
		t.Fatal(e)
	}
	h, e := windows.CreateFile(p, windows.GENERIC_READ, windows.FILE_SHARE_READ, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if e != nil {
		t.Fatal(e)
	}
	defer windows.CloseHandle(h)
	if _, e := svc(s).Execute(ctx, nil, addRequest("blocked")); kind(e) != "CommitUncertain" {
		t.Fatal("expected uncertain OS replacement failure", e)
	}
	if _, e := Decode(readBytes(t, path+".tmp")); e != nil {
		t.Fatal("replacement failure did not preserve valid temporary file", e)
	}
	if !bytes.Equal(before, readBytes(t, path)) {
		t.Fatal("main damaged under restricted handle")
	}
}
