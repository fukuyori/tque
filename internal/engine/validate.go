package engine

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	"tque/internal/model"
	"unicode"
	"unicode/utf8"
)

func Title(s string) (string, error) {
	if !utf8.ValidString(s) {
		return "", model.Err("InvalidInput", "invalid UTF-8 title")
	}
	// Reject controls before trimming, so a trailing newline is not silently accepted.
	for _, r := range s {
		if unicode.IsControl(r) {
			return "", model.Err("InvalidInput", "title contains a control character")
		}
	}
	s = strings.TrimSpace(s)
	if n := utf8.RuneCountInString(s); n < 1 || n > 200 {
		return "", model.Err("InvalidInput", "title must contain 1..200 code points")
	}
	return s, nil
}

func ID(s string) (int64, error) {
	if s == "" || strings.IndexFunc(s, func(r rune) bool { return r < '0' || r > '9' }) >= 0 {
		return 0, model.Err("InvalidInput", "invalid ID %q", s)
	}
	n, e := strconv.ParseInt(s, 10, 64)
	if e != nil || n <= 0 {
		return 0, model.Err("InvalidInput", "invalid ID %q", s)
	}
	return n, nil
}

func Date(s string, now time.Time) (string, error) {
	if s == "today" {
		return now.Format(time.DateOnly), nil
	}
	if strings.HasPrefix(s, "+") && strings.HasSuffix(s, "d") {
		ns := s[1 : len(s)-1]
		if ns == "" || strings.IndexFunc(ns, func(r rune) bool { return r < '0' || r > '9' }) >= 0 {
			return "", model.Err("InvalidInput", "invalid date %q", s)
		}
		n, err := strconv.Atoi(ns)
		if err != nil || n > 36500 {
			return "", model.Err("InvalidInput", "relative days must be 0..36500")
		}
		y, m, d := now.Date()
		t := time.Date(y, m, d, 0, 0, 0, 0, now.Location()).AddDate(0, 0, n)
		if t.Year() < 1 || t.Year() > 9999 {
			return "", model.Err("InvalidInput", "date out of range")
		}
		return t.Format(time.DateOnly), nil
	}
	t, err := time.Parse(time.DateOnly, s)
	if err != nil || t.Year() < 1 || t.Year() > 9999 || t.Format(time.DateOnly) != s {
		return "", model.Err("InvalidInput", "invalid date %q", s)
	}
	return s, nil
}

func Validate(d model.Document) error {
	if d.SchemaVersion != model.SchemaVersion {
		return model.Err("UnsupportedSchema", "schema_version=%d", d.SchemaVersion)
	}
	if d.Revision < 0 || d.Tasks == nil {
		return model.Err("InvalidInput", "invalid document revision or tasks")
	}
	byID := map[int64]model.Task{}
	identities := map[string]bool{}
	for _, t := range d.Tasks {
		if t.InternalID == "" || identities[t.InternalID] {
			return model.Err("InvalidInput", "missing/duplicate internal ID for #%d", t.ID)
		}
		identities[t.InternalID] = true
		if _, ok := byID[t.ID]; ok || t.ID <= 0 {
			return model.Err("InvalidInput", "invalid/duplicate task ID #%d", t.ID)
		}
		byID[t.ID] = t
		name, err := Title(t.Title)
		if err != nil {
			return err
		}
		if name != t.Title {
			return model.Err("InvalidInput", "#%d title is not trimmed", t.ID)
		}
		if t.Status != "todo" && t.Status != "done" {
			return model.Err("InvalidInput", "#%d invalid status", t.ID)
		}
		if t.Progress < 0 || t.Progress > 100 || (t.ProgressMode != "auto" && t.ProgressMode != "manual") {
			return model.Err("InvalidInput", "#%d invalid progress", t.ID)
		}
		if t.Status == "done" && (t.Progress != 100 || t.CompletedAt == "") || t.Status == "todo" && t.CompletedAt != "" {
			return model.Err("InvalidInput", "#%d inconsistent completion", t.ID)
		}
		for _, s := range []string{t.CreatedAt, t.CompletedAt} {
			if s == "" && s == t.CompletedAt && t.Status == "todo" {
				continue
			}
			v, e := time.Parse(time.RFC3339, s)
			if e != nil || !strings.HasSuffix(s, "Z") || v.Year() < 1 {
				return model.Err("InvalidInput", "#%d invalid UTC timestamp", t.ID)
			}
		}
		if t.CreatedAt == "" {
			return model.Err("InvalidInput", "#%d missing created_at", t.ID)
		}
		for _, s := range []string{t.StartDate, t.DueDate} {
			if s != "" {
				if len(s) != 10 {
					return model.Err("InvalidInput", "invalid stored date")
				}
				date, e := time.Parse(time.DateOnly, s)
				if e != nil || date.Year() < 1 || date.Year() > 9999 || date.Format(time.DateOnly) != s {
					return model.Err("InvalidInput", "invalid stored date %q", s)
				}
			}
		}
		if t.StartDate != "" && t.DueDate != "" && t.StartDate > t.DueDate {
			return model.Err("InvalidInput", "#%d start is after due", t.ID)
		}
	}
	parents := map[int64][]int64{}
	deps := map[int64][]int64{}
	for _, t := range d.Tasks {
		if t.ParentID != nil {
			parents[t.ID] = []int64{*t.ParentID}
		}
		deps[t.ID] = t.DependsOn
		for i, id := range t.DependsOn {
			if i > 0 && id <= t.DependsOn[i-1] {
				return model.Err("InvalidInput", "#%d dependencies must be sorted and unique", t.ID)
			}
		}
	}
	for _, g := range []struct {
		name  string
		edges map[int64][]int64
	}{{"parent", parents}, {"dep", deps}} {
		for from, ids := range g.edges {
			for _, id := range ids {
				if _, ok := byID[id]; !ok {
					return model.Err("NotFound", "#%d %s references #%d", from, g.name, id)
				}
			}
		}
		if err := acyclic(d.Tasks, g.edges, g.name); err != nil {
			return err
		}
	}
	return nil
}

func acyclic(tasks []model.Task, edges map[int64][]int64, kind string) error {
	type frame struct {
		id   int64
		next int
	}
	color := map[int64]byte{}
	for _, t := range tasks {
		if color[t.ID] != 0 {
			continue
		}
		stack := []frame{{id: t.ID}}
		color[t.ID] = 1
		for len(stack) > 0 {
			f := &stack[len(stack)-1]
			ns := edges[f.id]
			if f.next == len(ns) {
				color[f.id] = 2
				stack = stack[:len(stack)-1]
				continue
			}
			id := ns[f.next]
			f.next++
			if color[id] == 1 {
				parts := []string{}
				started := false
				for _, x := range stack {
					if x.id == id {
						started = true
					}
					if started {
						parts = append(parts, fmt.Sprintf("#%d", x.id))
					}
				}
				parts = append(parts, fmt.Sprintf("#%d", id))
				return model.Err("CycleDetected", "%s", strings.Join(parts, " -("+kind+")-> "))
			}
			if color[id] == 0 {
				color[id] = 1
				stack = append(stack, frame{id: id})
			}
		}
	}
	return nil
}
