// Command licenses collects notices for the dependencies of a release binary.
package main

import (
	"debug/buildinfo"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"sort"
	"strings"
)

type module struct {
	Path, Version, Dir string
	Main               bool
	Replace            *module
}

func main() {
	binary := flag.String("binary", "", "release executable")
	out := flag.String("out", "", "new output directory")
	targetOS := flag.String("os", "", "release GOOS")
	arch := flag.String("arch", "", "release GOARCH")
	flag.Parse()
	if err := collect(*binary, *out, *targetOS, *arch); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func collect(binary, out, targetOS, arch string) error {
	if binary == "" || out == "" || targetOS == "" || arch == "" {
		return fmt.Errorf("binary, out, os and arch are required")
	}
	if _, err := os.Lstat(out); !os.IsNotExist(err) {
		return fmt.Errorf("output must not exist: %s", out)
	}
	info, err := buildinfo.ReadFile(binary)
	if err != nil {
		return err
	}
	if info.GoVersion != runtime.Version() {
		return fmt.Errorf("Go toolchain mismatch: binary %s, collector %s", info.GoVersion, runtime.Version())
	}
	settings := map[string]string{}
	for _, s := range info.Settings {
		settings[s.Key] = s.Value
	}
	for key, value := range map[string]string{"GOOS": targetOS, "GOARCH": arch, "CGO_ENABLED": "0"} {
		if settings[key] != value {
			return fmt.Errorf("binary %s mismatch: got %q, want %q", key, settings[key], value)
		}
	}
	// Unsupported custom build conditions must never silently change the inventory.
	if settings["-tags"] != "" {
		return fmt.Errorf("custom build tags are not supported")
	}
	cmd := exec.Command("go", "list", "-mod=readonly", "-deps", "-json", ".")
	cmd.Env = os.Environ()
	for _, entry := range []string{"GOOS=" + targetOS, "GOARCH=" + arch, "CGO_ENABLED=0"} {
		key := strings.SplitN(entry, "=", 2)[0] + "="
		filtered := cmd.Env[:0]
		for _, old := range cmd.Env {
			if !strings.HasPrefix(old, key) {
				filtered = append(filtered, old)
			}
		}
		cmd.Env = append(filtered, entry)
	}
	cmd.Stderr = os.Stderr
	data, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("list release dependencies: %w", err)
	}
	modules := map[string]module{}
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	for {
		var pkg struct{ Module *module }
		if err := decoder.Decode(&pkg); err == io.EOF {
			break
		} else if err != nil {
			return err
		}
		m := pkg.Module
		if m == nil || m.Main {
			continue
		}
		if m.Replace != nil {
			return fmt.Errorf("module replacements require explicit license review: %s", m.Path)
		}
		modules[m.Path] = *m
	}
	if err := verifyModules(modules, info.Deps); err != nil {
		return err
	}
	// Validate and read everything before creating output. Output write failures
	// abort packaging; any partial directory is rejected on subsequent runs.
	files := map[string][]byte{}
	names := make([]string, 0, len(modules))
	for name := range modules {
		names = append(names, name)
	}
	sort.Strings(names)
	var index strings.Builder
	for _, name := range names {
		m := modules[name]
		if m.Dir == "" || m.Version == "" {
			return fmt.Errorf("missing module metadata: %s", name)
		}
		folder := strings.NewReplacer("/", "_", ":", "_").Replace(name + "@" + m.Version)
		found := 0
		err := filepath.WalkDir(m.Dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !isNotice(d.Name()) {
				return nil
			}
			if !d.Type().IsRegular() {
				return fmt.Errorf("non-regular notice: %s", path)
			}
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if len(strings.TrimSpace(string(content))) == 0 {
				return fmt.Errorf("empty notice: %s", path)
			}
			rel, err := filepath.Rel(m.Dir, path)
			if err != nil {
				return err
			}
			files[filepath.Join(folder, rel)] = content
			found++
			return nil
		})
		if err != nil {
			return err
		}
		if found == 0 {
			return fmt.Errorf("missing license: %s", name)
		}
		fmt.Fprintf(&index, "%s %s\n", name, m.Version)
	}
	goLicense, err := os.ReadFile(filepath.Join(runtime.GOROOT(), "LICENSE"))
	if err != nil {
		return err
	}
	if len(strings.TrimSpace(string(goLicense))) == 0 {
		return fmt.Errorf("empty Go license")
	}
	files["GO-LICENSE"] = goLicense
	files["MODULES.txt"] = []byte(index.String())
	if err := os.MkdirAll(filepath.Dir(out), 0755); err != nil {
		return err
	}
	if err := os.Mkdir(out, 0755); err != nil {
		return err
	}
	for name, content := range files {
		path := filepath.Join(out, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(path, content, 0644); err != nil {
			return err
		}
	}
	fmt.Printf("Collected %d modules for %s/%s\n", len(modules), targetOS, arch)
	return nil
}

func isNotice(name string) bool {
	name = strings.ToUpper(name)
	for _, prefix := range []string{"LICENSE", "COPYING", "NOTICE", "COPYRIGHT"} {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

func verifyModules(modules map[string]module, deps []*debug.Module) error {
	if len(modules) != len(deps) {
		return fmt.Errorf("binary/module inventory count mismatch: %d != %d", len(deps), len(modules))
	}
	for _, dep := range deps {
		m, ok := modules[dep.Path]
		if !ok || m.Version != dep.Version || dep.Replace != nil || m.Replace != nil {
			return fmt.Errorf("binary/module mismatch: %s %s", dep.Path, dep.Version)
		}
	}
	return nil
}
