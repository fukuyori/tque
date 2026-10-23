package engine

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"
	"tque/internal/model"
)

type Repository interface {
	Read(context.Context) (model.Document, error)
	Update(context.Context, *int64, func(*model.Document) error) (model.Document, error)
}
type Service struct {
	Repo Repository
	Now  func() time.Time
}
type Patch struct {
	Title, Progress, Start, Due *string
	ParentSet                   bool
	Parent                      *int64
	DepsSet                     bool
	Deps                        []int64
}
type Request struct {
	Op      string
	ID      int64
	Patch   Patch
	Cascade bool
	NewID   int64
}

type IDChange struct{ From, To int64 }
type Result struct {
	Document   model.Document
	ID         int64
	Removed    []int64
	Changed    bool
	Renumbered []IDChange
}

func (s Service) Execute(ctx context.Context, expected *int64, r Request) (Result, error) {
	now := time.Now()
	if s.Now != nil {
		now = s.Now()
	}
	res := Result{}
	var before int64
	started := false
	d, err := s.Repo.Update(ctx, expected, func(d *model.Document) error {
		started = true
		before = d.Revision
		old := map[string]int64{}
		if r.Op == "renumber" {
			for _, t := range d.Tasks {
				old[t.InternalID] = t.ID
			}
		}
		id, removed, e := Apply(d, r, now)
		if e == nil && r.Op == "renumber" {
			for _, t := range d.Tasks {
				if from := old[t.InternalID]; from != t.ID {
					res.Renumbered = append(res.Renumbered, IDChange{From: from, To: t.ID})
				}
			}
			slices.SortFunc(res.Renumbered, func(a, b IDChange) int {
				if a.From < b.From {
					return -1
				}
				if a.From > b.From {
					return 1
				}
				return 0
			})
		}
		res.ID = id
		res.Removed = removed
		return e
	})
	res.Document = d
	res.Changed = started && d.Revision > before
	return res, err
}

// NewID returns the highest existing ID plus one, or 1 when there are no tasks.
// Deleted IDs are not recorded, so removing the highest ID makes it available again.
func NewID(tasks []model.Task) (int64, error) {
	highest := int64(0)
	for _, t := range tasks {
		highest = max(highest, t.ID)
	}
	if highest == math.MaxInt64 {
		return 0, model.Err("InvalidInput", "task ID limit reached")
	}
	return highest + 1, nil
}

// Apply changes a transaction's private copy. The repository validates before commit.
func Apply(d *model.Document, r Request, now time.Time) (int64, []int64, error) {
	idx := -1
	for i, t := range d.Tasks {
		if t.ID == r.ID {
			idx = i
			break
		}
	}
	if r.Op == "add" {
		id, e := NewID(d.Tasks)
		if e != nil {
			return 0, nil, e
		}
		r.ID = id
		var key [16]byte
		if _, err := rand.Read(key[:]); err != nil {
			return 0, nil, err
		}
		d.Tasks = append(d.Tasks, model.Task{InternalID: hex.EncodeToString(key[:]), ID: r.ID, Status: "todo", ProgressMode: "auto", CreatedAt: now.UTC().Format(time.RFC3339)})
		idx = len(d.Tasks) - 1
	} else if idx < 0 {
		return 0, nil, model.Err("NotFound", "task #%d", r.ID)
	}
	t := &d.Tasks[idx]
	switch r.Op {
	case "renumber":
		if err := Renumber(d, r.ID, r.NewID); err != nil {
			return 0, nil, err
		}
		return r.NewID, nil, nil
	case "add", "edit":
		p := r.Patch
		if r.Op == "edit" && p.Title == nil && p.Progress == nil && p.Start == nil && p.Due == nil && !p.ParentSet && !p.DepsSet {
			return 0, nil, model.Err("InvalidInput", "no attributes specified")
		}
		if p.Title != nil {
			v, e := Title(*p.Title)
			if e != nil {
				return 0, nil, e
			}
			t.Title = v
		}
		if p.ParentSet {
			t.ParentID = p.Parent
		}
		if p.DepsSet {
			t.DependsOn = append([]int64{}, p.Deps...)
			slices.Sort(t.DependsOn)
			t.DependsOn = slices.Compact(t.DependsOn)
		}
		for _, field := range []struct {
			in  *string
			out *string
		}{{p.Start, &t.StartDate}, {p.Due, &t.DueDate}} {
			if field.in == nil {
				continue
			}
			if *field.in == "" {
				*field.out = ""
				continue
			}
			v, e := Date(*field.in, now)
			if e != nil {
				return 0, nil, e
			}
			*field.out = v
		}
		if p.Progress != nil {
			if *p.Progress == "auto" {
				t.ProgressMode = "auto"
			} else {
				v := *p.Progress
				if v == "" || strings.IndexFunc(v, func(r rune) bool { return r < '0' || r > '9' }) >= 0 {
					return 0, nil, model.Err("InvalidInput", "progress must be 0..100 or auto")
				}
				n, e := strconv.Atoi(v)
				if e != nil || n > 100 || t.Status == "done" && n < 100 {
					return 0, nil, model.Err("InvalidInput", "invalid progress; reopen completed task before reducing progress")
				}
				t.Progress = n
				t.ProgressMode = "manual"
			}
		}
	case "done":
		children := map[int64][]model.Task{}
		for _, child := range d.Tasks {
			if child.ParentID != nil {
				children[*child.ParentID] = append(children[*child.ParentID], child)
			}
		}
		pending := []int64{t.ID}
		seen := map[int64]bool{t.ID: true}
		for i := 0; i < len(pending); i++ {
			for _, child := range children[pending[i]] {
				if child.Status != "done" {
					return 0, nil, model.Err("InvalidInput", "cannot complete #%d: child #%d is incomplete", t.ID, child.ID)
				}
				if !seen[child.ID] {
					seen[child.ID] = true
					pending = append(pending, child.ID)
				}
			}
		}
		if t.Status != "done" {
			t.Status = "done"
			t.Progress = 100
			t.ProgressMode = "auto"
			t.CompletedAt = now.UTC().Format(time.RFC3339)
		}
	case "undone":
		if t.Status != "todo" {
			t.Status = "todo"
			t.Progress = 0
			t.CompletedAt = ""
		}
	case "rm":
		ids, err := Removal(*d, r.ID, r.Cascade)
		if err != nil {
			return 0, nil, err
		}
		set := map[int64]bool{}
		for _, id := range ids {
			set[id] = true
		}
		kept := make([]model.Task, 0, len(d.Tasks)-len(ids))
		for _, t := range d.Tasks {
			if !set[t.ID] {
				kept = append(kept, t)
			}
		}
		d.Tasks = kept
		return r.ID, ids, nil
	default:
		return 0, nil, model.Err("InvalidInput", "unknown operation %q", r.Op)
	}
	return r.ID, nil, nil
}

func Removal(d model.Document, id int64, cascade bool) ([]int64, error) {
	children := map[int64][]int64{}
	found := false
	for _, t := range d.Tasks {
		if t.ID == id {
			found = true
		}
		if t.ParentID != nil {
			children[*t.ParentID] = append(children[*t.ParentID], t.ID)
		}
	}
	if !found {
		return nil, model.Err("NotFound", "task #%d", id)
	}
	if len(children[id]) > 0 && !cascade {
		return nil, model.Err("InvalidInput", "#%d has children; use --cascade", id)
	}
	ids := []int64{id}
	set := map[int64]bool{id: true}
	for i := 0; i < len(ids); i++ {
		for _, child := range children[ids[i]] {
			if !set[child] {
				set[child] = true
				ids = append(ids, child)
			}
		}
	}
	refs := []int64{}
	for _, t := range d.Tasks {
		if set[t.ID] {
			continue
		}
		for _, dep := range t.DependsOn {
			if set[dep] {
				refs = append(refs, t.ID)
				break
			}
		}
	}
	if len(refs) > 0 {
		slices.Sort(refs)
		return nil, model.Err("ReferencedTask", "cannot remove #%d: referenced by %v", id, refs)
	}
	slices.Sort(ids)
	return ids, nil
}
