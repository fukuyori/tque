package config

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"tque/internal/model"
	"tque/internal/strictjson"
	"unicode"
)

type Config struct {
	DataDir string
	Path    string
}

func Load(home string) (Config, error) {
	c := Config{DataDir: filepath.Join(home, ".tque"), Path: filepath.Join(home, ".tque", "config.json")}
	bad := func(e error) (Config, error) { return Config{}, model.Err("ConfigError", "%s: %v", c.Path, e) }
	if home == "" || !filepath.IsAbs(home) {
		return bad(model.Err("InvalidInput", "home must be absolute"))
	}
	b, err := os.ReadFile(c.Path)
	if os.IsNotExist(err) {
		return c, nil
	}
	if err != nil {
		return bad(err)
	}
	b = bytes.TrimPrefix(b, []byte{0xef, 0xbb, 0xbf})
	if err := strictjson.Allowed(b, "data_dir"); err != nil {
		return bad(err)
	}
	var data struct {
		DataDir *string `json:"data_dir"`
	}
	if err := strictjson.Decode(b, &data); err != nil {
		return bad(err)
	}
	if data.DataDir == nil {
		return c, nil
	}
	v := *data.DataDir
	if strings.TrimSpace(v) == "" || strings.IndexFunc(v, unicode.IsControl) >= 0 {
		return bad(model.Err("InvalidInput", "empty path or control character"))
	}
	sep := func(c byte) bool { return c == '/' || runtime.GOOS == "windows" && c == '\\' }
	if v[0] == '~' {
		if v == "~" {
			v = home
		} else if len(v) > 1 && sep(v[1]) {
			v = filepath.Join(home, v[2:])
		} else {
			return bad(model.Err("InvalidInput", "~user expansion is unsupported"))
		}
	}
	if runtime.GOOS == "windows" && !filepath.IsAbs(v) && (filepath.VolumeName(v) != "" || sep(v[0])) {
		return bad(model.Err("InvalidInput", "drive-relative/root-relative path is unsupported"))
	}
	if !filepath.IsAbs(v) {
		v = filepath.Join(filepath.Dir(c.Path), v)
	}
	c.DataDir, err = filepath.Abs(v)
	if err != nil {
		return bad(err)
	}
	return c, nil
}
