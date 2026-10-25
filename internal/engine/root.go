package engine

import (
	"slices"
	"strconv"
	"strings"
	"tque/internal/model"
	"unicode"
)

// RootValue normalizes a --root argument. It may be an ID or a title.
func RootValue(raw string) (string, error) {
	v := strings.TrimSpace(raw)
	if v == "" {
		return "", model.Err("InvalidInput", "--root must not be empty")
	}
	if strings.IndexFunc(v, unicode.IsControl) >= 0 {
		return "", model.Err("InvalidInput", "--root must not contain control characters")
	}
	return v, nil
}

// RootCandidates returns tasks whose ID equals value or whose title equals it
// ignoring case, in ID order. A numeric value matches both, so a task titled
// "12" is offered alongside task #12. Completed tasks are included.
func RootCandidates(d model.Document, value string) []model.Task {
	id, idErr := ID(value)
	out := []model.Task{}
	for _, t := range d.Tasks {
		if idErr == nil && t.ID == id || strings.EqualFold(t.Title, value) {
			out = append(out, t)
		}
	}
	slices.SortFunc(out, func(a, b model.Task) int {
		if a.ID < b.ID {
			return -1
		}
		if a.ID > b.ID {
			return 1
		}
		return 0
	})
	return out
}

// TaskPath returns the titles from the top-level ancestor down to the task.
func TaskPath(d model.Document, id int64) []string {
	byID := make(map[int64]model.Task, len(d.Tasks))
	for _, t := range d.Tasks {
		byID[t.ID] = t
	}
	path := []string{}
	seen := map[int64]bool{}
	for t, ok := byID[id]; ok && !seen[t.ID]; t, ok = byID[derefOr(t.ParentID)] {
		seen[t.ID] = true
		path = append(path, t.Title)
	}
	slices.Reverse(path)
	return path
}

// FormatRootCandidates lists candidates as "#ID path" for error messages.
func FormatRootCandidates(d model.Document, tasks []model.Task) string {
	parts := make([]string, 0, len(tasks))
	for _, t := range tasks {
		parts = append(parts, "#"+strconv.FormatInt(t.ID, 10)+" "+strings.Join(TaskPath(d, t.ID), " > "))
	}
	return strings.Join(parts, ", ")
}

func subtree(d model.Document, root int64) map[int64]bool {
	children := map[int64][]int64{}
	found := false
	for _, t := range d.Tasks {
		found = found || t.ID == root
		if t.ParentID != nil {
			children[*t.ParentID] = append(children[*t.ParentID], t.ID)
		}
	}
	set := map[int64]bool{}
	if !found {
		return set
	}
	pending := []int64{root}
	set[root] = true
	for i := 0; i < len(pending); i++ {
		for _, c := range children[pending[i]] {
			if !set[c] {
				set[c] = true
				pending = append(pending, c)
			}
		}
	}
	return set
}

func derefOr(p *int64) int64 {
	if p == nil {
		return 0
	}
	return *p
}
