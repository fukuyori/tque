package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/spf13/cobra"
	"io"
	"os"
	"strings"
	"time"
	"tque/internal/config"
	"tque/internal/engine"
	"tque/internal/model"
	"tque/internal/storage"
)

type Options struct {
	Version string
	Home    func() (string, error)
	Now     func() time.Time
	// RunTUI receives the normalized --root value, or "" for all tasks.
	RunTUI func(ctx context.Context, s engine.Service, dataDir, root string) error
}
type usageError struct{ error }

func usage(format string, args ...any) error { return usageError{fmt.Errorf(format, args...)} }
func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	var u usageError
	if errors.As(err, &u) {
		return 2
	}
	var e *model.Error
	if errors.As(err, &e) && e.Kind == "CommitUncertain" {
		return 3
	}
	return 1
}

func New(o Options) *cobra.Command {
	if o.Home == nil {
		o.Home = os.UserHomeDir
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	var forceTUI bool
	root := &cobra.Command{Use: "tque", Short: "階層・依存関係・進捗を管理するタスクキュー", Version: o.Version, SilenceUsage: true, SilenceErrors: true, CompletionOptions: cobra.CompletionOptions{DisableDefaultCmd: true}}
	root.PersistentFlags().BoolVar(&forceTUI, "tui", false, "TUIを起動")
	root.Args = func(_ *cobra.Command, args []string) error {
		if len(args) > 0 {
			return usage("unknown command %q", args[0])
		}
		return nil
	}
	root.SetFlagErrorFunc(func(_ *cobra.Command, e error) error { return usage("%v", e) })
	root.Flags().String("root", "", "起点タスクのIDまたはタイトル（その子孫だけを表示）")
	rootValue := func(c *cobra.Command) (string, error) {
		if !c.Flags().Changed("root") {
			return "", nil
		}
		raw, _ := c.Flags().GetString("root")
		v, e := engine.RootValue(raw)
		if e != nil {
			return "", usage("%v", e)
		}
		return v, nil
	}
	service := func() (engine.Service, string, error) {
		home, e := o.Home()
		if e != nil {
			return engine.Service{}, "", model.Err("ConfigError", "home: %v", e)
		}
		c, e := config.Load(home)
		if e != nil {
			return engine.Service{}, "", e
		}
		return engine.Service{Repo: &storage.Store{Dir: c.DataDir}, Now: o.Now}, c.DataDir, nil
	}
	root.RunE = func(c *cobra.Command, args []string) error {
		if len(args) > 0 {
			return usage("unknown command %q", args[0])
		}
		rv, e := rootValue(c)
		if e != nil {
			return e
		}
		if o.RunTUI == nil {
			return model.Err("InvalidInput", "TUI runner is unavailable")
		}
		s, path, e := service()
		if e != nil {
			return e
		}
		return o.RunTUI(c.Context(), s, path, rv)
	}
	root.PersistentPreRunE = func(c *cobra.Command, _ []string) error {
		if forceTUI && c != root {
			return usage("--tui cannot be combined with a subcommand")
		}
		return nil
	}
	for _, op := range []string{"add", "edit", "done", "undone", "rm"} {
		c := &cobra.Command{Use: op + " <id>", Short: map[string]string{"add": "タスクを追加", "edit": "属性を編集", "done": "完了にする", "undone": "未完了に戻す", "rm": "タスクを削除"}[op]}
		if op == "add" {
			c.Use = "add <title>"
		}
		c.Args = func(_ *cobra.Command, a []string) error {
			if len(a) != 1 {
				return usage("exactly one argument is required")
			}
			return nil
		}
		if op == "add" || op == "edit" {
			attributeFlags(c, op == "edit")
		}
		if op == "rm" {
			c.Flags().Bool("cascade", false, "子孫も削除")
		}
		c.RunE = func(c *cobra.Command, a []string) error {
			r := engine.Request{Op: op}
			if op == "add" {
				r.Patch.Title = &a[0]
			} else {
				id, e := engine.ID(a[0])
				if e != nil {
					return usage("%v", e)
				}
				r.ID = id
			}
			if op == "add" || op == "edit" {
				p, e := parsePatch(c, op == "add")
				if e != nil {
					return e
				}
				if op == "add" {
					p.Title = &a[0]
				}
				r.Patch = p
			}
			if op == "rm" {
				r.Cascade, _ = c.Flags().GetBool("cascade")
			}
			s, _, e := service()
			if e != nil {
				return e
			}
			res, e := s.Execute(c.Context(), nil, r)
			return reportResult(c.OutOrStdout(), r, res, e)
		}
		root.AddCommand(c)
	}
	renumber := &cobra.Command{Use: "renumber <id> <new-id>", Short: "表示用IDを付け直す", Args: func(_ *cobra.Command, a []string) error {
		if len(a) != 2 {
			return usage("renumber requires an ID and a new ID")
		}
		return nil
	}, RunE: func(c *cobra.Command, a []string) error {
		from, err := engine.ID(a[0])
		if err != nil {
			return usage("%v", err)
		}
		to, err := engine.ID(a[1])
		if err != nil {
			return usage("%v", err)
		}
		s, _, err := service()
		if err != nil {
			return err
		}
		r := engine.Request{Op: "renumber", ID: from, NewID: to}
		res, err := s.Execute(c.Context(), nil, r)
		return reportResult(c.OutOrStdout(), r, res, err)
	}}
	root.AddCommand(renumber)
	list := &cobra.Command{Use: "ls", Short: "タスク一覧", Args: func(_ *cobra.Command, a []string) error {
		if len(a) > 0 {
			return usage("ls takes no arguments")
		}
		return nil
	}}
	for _, f := range []string{"all", "tree", "blocked", "json", "due-today", "overdue", "ready"} {
		list.Flags().Bool(f, false, f)
	}
	list.Flags().String("search", "", "タイトルの部分一致検索")
	list.Flags().String("root", "", "起点タスクのIDまたはタイトル（その子孫だけを表示）")
	list.Flags().String("sort", "id", "id、due または name")
	_ = list.RegisterFlagCompletionFunc("sort", fixed("id", "due", "name"))
	list.RunE = func(c *cobra.Command, _ []string) error {
		b := func(n string) bool { v, _ := c.Flags().GetBool(n); return v }
		str := func(n string) string { v, _ := c.Flags().GetString(n); return v }
		q := engine.Query{All: b("all"), Tree: b("tree"), Blocked: b("blocked"), DueToday: b("due-today"), Overdue: b("overdue"), Ready: b("ready"), Search: str("search"), Sort: str("sort"), Today: o.Now().Format(time.DateOnly)}
		if q.Tree && b("json") {
			return usage("--tree and --json cannot be combined")
		}
		if q.Blocked && q.Ready {
			return usage("--blocked and --ready cannot be combined")
		}
		if q.Sort != "id" && q.Sort != "due" && q.Sort != "name" {
			return usage("--sort must be id, due or name")
		}
		rv, e := rootValue(c)
		if e != nil {
			return e
		}
		s, _, e := service()
		if e != nil {
			return e
		}
		d, e := s.Repo.Read(c.Context())
		if e != nil {
			return e
		}
		if rv != "" {
			candidates := engine.RootCandidates(d, rv)
			switch len(candidates) {
			case 0:
				return model.Err("NotFound", "no task matches --root %q", rv)
			case 1:
				q.Root = candidates[0].ID
			default:
				return model.Err("InvalidInput", "--root %q matches %d tasks; specify an ID: %s", rv, len(candidates), engine.FormatRootCandidates(d, candidates))
			}
		}
		rows, _ := engine.Select(d, q)
		if b("json") {
			enc := json.NewEncoder(c.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(rows)
		}
		if len(rows) == 0 {
			_, e = fmt.Fprintln(c.OutOrStdout(), "対象なし")
			return e
		}
		prefixes := treePrefixes(rows)
		for i, v := range rows {
			if q.Tree && engine.TreeGap(rows, i) {
				if _, e = fmt.Fprintln(c.OutOrStdout()); e != nil {
					return e
				}
			}
			prefix := ""
			if q.Tree {
				prefix = prefixes[v.ID]
			}
			if _, e = fmt.Fprintln(c.OutOrStdout(), prefix+FormatTask(v)); e != nil {
				return e
			}
		}
		return nil
	}
	root.AddCommand(list)
	completion := &cobra.Command{Use: "completion <powershell|bash|zsh|fish>", Short: "シェル補完を生成", ValidArgs: []string{"powershell", "bash", "zsh", "fish"}, Args: func(_ *cobra.Command, a []string) error {
		if len(a) != 1 {
			return usage("specify one shell")
		}
		return nil
	}, RunE: func(c *cobra.Command, a []string) error {
		switch a[0] {
		case "powershell":
			return root.GenPowerShellCompletionWithDesc(c.OutOrStdout())
		case "bash":
			return root.GenBashCompletionV2(c.OutOrStdout(), true)
		case "zsh":
			return root.GenZshCompletion(c.OutOrStdout())
		case "fish":
			return root.GenFishCompletion(c.OutOrStdout(), true)
		}
		return usage("unsupported shell %q", a[0])
	}}
	root.AddCommand(completion)
	return root
}

func fixed(values ...string) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return values, cobra.ShellCompDirectiveNoFileComp
	}
}
func attributeFlags(c *cobra.Command, edit bool) {
	if edit {
		c.Flags().StringP("title", "t", "", "タイトル")
	}
	c.Flags().StringP("start", "s", "", "開始日")
	c.Flags().StringP("due", "d", "", "期日")
	c.Flags().StringP("progress", "p", "", "0..100 または auto")
	c.Flags().String("parent", "", "親ID")
	c.Flags().StringArray("dep", nil, "依存ID（カンマ区切り・繰返し可）")
	_ = c.RegisterFlagCompletionFunc("progress", fixed("auto"))
	if edit {
		for _, f := range []string{"parent", "dep", "start", "due"} {
			_ = c.RegisterFlagCompletionFunc(f, fixed("none"))
		}
	}
}
func parsePatch(c *cobra.Command, add bool) (engine.Patch, error) {
	p := engine.Patch{}
	f := c.Flags()
	for _, item := range []struct {
		name string
		dst  **string
	}{{"title", &p.Title}, {"progress", &p.Progress}, {"start", &p.Start}, {"due", &p.Due}} {
		if !f.Changed(item.name) {
			continue
		}
		v, _ := f.GetString(item.name)
		if (item.name == "start" || item.name == "due") && v == "none" {
			if add {
				return p, usage("none is only valid with edit")
			}
			v = ""
		}
		*item.dst = &v
	}
	if f.Changed("parent") {
		p.ParentSet = true
		v, _ := f.GetString("parent")
		if v == "none" && !add {
		} else {
			id, e := engine.ID(v)
			if e != nil {
				return p, usage("%v", e)
			}
			p.Parent = &id
		}
	}
	if f.Changed("dep") {
		p.DepsSet = true
		vals, _ := f.GetStringArray("dep")
		if len(vals) == 1 && vals[0] == "none" && !add {
			p.Deps = []int64{}
		} else {
			for _, v := range vals {
				for _, part := range strings.Split(v, ",") {
					id, e := engine.ID(part)
					if e != nil {
						return p, usage("%v", e)
					}
					p.Deps = append(p.Deps, id)
				}
			}
		}
	}
	if !add && p.Title == nil && p.Progress == nil && p.Start == nil && p.Due == nil && !p.ParentSet && !p.DepsSet {
		return p, usage("edit requires at least one attribute flag")
	}
	return p, nil
}

func FormatTask(v engine.View) string {
	status := "[ ]"
	if v.Status == "done" {
		status = "[x]"
	}
	s := fmt.Sprintf("%d %s %s", v.ID, status, v.Title)
	if v.Context {
		s += " [context]"
	}
	if v.ProgressMode == "manual" {
		s += " [manual]"
	}
	s += fmt.Sprintf(" [%s%s %d%%]", strings.Repeat("#", v.EffectiveProgress/10), strings.Repeat(".", 10-v.EffectiveProgress/10), v.EffectiveProgress)
	if v.StartDate != "" || v.DueDate != "" {
		s += " " + v.StartDate + " -> " + v.DueDate
	}
	if v.Blocked {
		ids := []string{}
		for _, id := range v.BlockedBy {
			ids = append(ids, fmt.Sprintf("#%d", id))
		}
		s += " (blocked by " + strings.Join(ids, ", ") + ")"
	}
	return s
}

func treePrefixes(rows []engine.View) map[int64]string { return engine.TreePrefixes(rows) }

// Report a confirmed commit even if subsequent lock cleanup failed. Keep the
// error so callers see a nonzero exit, but never hide the assigned task ID.
func reportResult(w io.Writer, r engine.Request, res engine.Result, err error) error {
	if err != nil && !res.Changed {
		return err
	}
	return errors.Join(printResult(w, r, res), err)
}

func printResult(w io.Writer, r engine.Request, res engine.Result) error {
	var s string
	if !res.Changed {
		s = fmt.Sprintf("[=] Unchanged task #%d", res.ID)
	} else {
		switch r.Op {
		case "renumber":
			s = "[*] Renumbered: " + engine.FormatIDChanges(res.Renumbered)
		case "add":
			for _, t := range res.Document.Tasks {
				if t.ID == res.ID {
					s = fmt.Sprintf("[+] Created task #%d: %s", res.ID, t.Title)
					break
				}
			}
		case "edit":
			s = fmt.Sprintf("[*] Updated task #%d", res.ID)
		case "done":
			s = fmt.Sprintf("[x] Completed task #%d", res.ID)
		case "undone":
			s = fmt.Sprintf("[ ] Reopened task #%d", res.ID)
		case "rm":
			ids := []string{}
			for _, id := range res.Removed {
				ids = append(ids, fmt.Sprintf("#%d", id))
			}
			s = fmt.Sprintf("[-] Removed %d task(s): %s", len(ids), strings.Join(ids, ", "))
		}
	}
	_, e := fmt.Fprintln(w, s)
	return e
}
