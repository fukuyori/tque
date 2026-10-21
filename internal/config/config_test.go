package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestConfig(t *testing.T) {
	home := t.TempDir()
	c, e := Load(home)
	if e != nil || c.DataDir != filepath.Join(home, ".tque") {
		t.Fatal(c, e)
	}
	if _, e := os.Stat(c.DataDir); !os.IsNotExist(e) {
		t.Fatal("read created directory")
	}
	if e := os.Mkdir(c.DataDir, 0700); e != nil {
		t.Fatal(e)
	}
	for _, b := range []string{`{}`, `{"data_dir":"data 日本語"}`, "\xef\xbb\xbf" + `{"data_dir":"~/tasks"}`} {
		if e := os.WriteFile(c.Path, []byte(b), 0600); e != nil {
			t.Fatal(e)
		}
		v, e := Load(home)
		if e != nil || !filepath.IsAbs(v.DataDir) {
			t.Fatal(v, e)
		}
	}
	for _, b := range []string{`null`, `[]`, `{"data_dir":null}`, `{"data_dir":""}`, `{"data_dir":" "}`, `{"other":1}`, `{"data_dir":"a","data_dir":"b"}`, `{} {}`, `{"data_dir":"~other/tasks"}`} {
		_ = os.WriteFile(c.Path, []byte(b), 0600)
		if _, e := Load(home); e == nil {
			t.Fatal("accepted", b)
		}
	}
	if runtime.GOOS == "windows" {
		for _, b := range []string{`{"data_dir":"D:tasks"}`, `{"data_dir":"\\tasks"}`} {
			_ = os.WriteFile(c.Path, []byte(b), 0600)
			if _, e := Load(home); e == nil {
				t.Fatal(b)
			}
		}
	}
}
