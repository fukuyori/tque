package engine

import (
	"slices"
	"strings"
	"tque/internal/model"
)

type View struct {
	model.Task
	DependsOn         []int64 `json:"depends_on"`
	EffectiveProgress int     `json:"effective_progress"`
	Blocked           bool    `json:"blocked"`
	BlockedBy         []int64 `json:"blocked_by"`
	Context           bool    `json:"-"`
	Depth             int     `json:"-"`
}
type Query struct {
	All, Blocked, DueToday, Overdue, Ready, Tree bool
	Search, Sort, Today                          string
	// Root limits the result to that task and its descendants; 0 means all tasks.
	Root int64
}

func Views(d model.Document) map[int64]View {
	views := make(map[int64]View, len(d.Tasks))
	children := map[int64][]int64{}
	pending := map[int64]int{}
	for _, t := range d.Tasks {
		views[t.ID] = View{Task: t, DependsOn: append([]int64{}, t.DependsOn...), BlockedBy: []int64{}}
		if t.ParentID != nil {
			children[*t.ParentID] = append(children[*t.ParentID], t.ID)
		}
	}
	queue := []int64{}
	for _, t := range d.Tasks {
		pending[t.ID] = len(children[t.ID])
		if pending[t.ID] == 0 {
			queue = append(queue, t.ID)
		}
	}
	for i := 0; i < len(queue); i++ {
		id := queue[i]
		v := views[id]
		v.EffectiveProgress = v.Progress
		if v.Status == "done" {
			v.EffectiveProgress = 100
		} else if v.ProgressMode == "auto" && len(children[id]) > 0 {
			sum := 0
			for _, c := range children[id] {
				sum += views[c].EffectiveProgress
			}
			v.EffectiveProgress = sum / len(children[id])
		}
		for _, dep := range v.DependsOn {
			if views[dep].Status != "done" {
				v.BlockedBy = append(v.BlockedBy, dep)
			}
		}
		v.Blocked = len(v.BlockedBy) > 0
		views[id] = v
		if v.ParentID != nil {
			p := *v.ParentID
			pending[p]--
			if pending[p] == 0 {
				queue = append(queue, p)
			}
		}
	}
	return views
}

func Select(d model.Document, q Query) ([]View, int) {
	views := Views(d)
	matched := map[int64]bool{}
	included := map[int64]bool{}
	var scope map[int64]bool
	if q.Root != 0 {
		scope = subtree(d, q.Root)
	}
	for id, v := range views {
		if scope != nil && !scope[id] {
			continue
		}
		if !q.All && v.Status != "todo" || q.Blocked && !v.Blocked {
			continue
		}
		if q.DueToday && (v.Status != "todo" || v.DueDate == "" || v.DueDate > q.Today) {
			continue
		}
		if q.Overdue && (v.Status != "todo" || v.DueDate == "" || v.DueDate >= q.Today) {
			continue
		}
		if q.Ready && (v.Status != "todo" || v.Blocked || v.StartDate != "" && v.StartDate > q.Today) {
			continue
		}
		if !strings.Contains(strings.ToLower(v.Title), strings.ToLower(q.Search)) {
			continue
		}
		matched[id] = true
		included[id] = true
	}
	if q.Tree {
		// Context ancestors stop at the root so tasks above it stay hidden.
		for id := range matched {
			for cur := id; cur != q.Root; {
				p := views[cur].ParentID
				if p == nil || included[*p] {
					break
				}
				included[*p] = true
				cur = *p
			}
		}
	}
	less := func(a, b View) int {
		if q.Sort == "name" && a.Title != b.Title {
			return strings.Compare(a.Title, b.Title)
		}
		if q.Sort == "due" && a.DueDate != b.DueDate {
			if a.DueDate == "" {
				return 1
			}
			if b.DueDate == "" {
				return -1
			}
			return strings.Compare(a.DueDate, b.DueDate)
		}
		if a.ID < b.ID {
			return -1
		}
		if a.ID > b.ID {
			return 1
		}
		return 0
	}
	out := []View{}
	if !q.Tree {
		for id := range included {
			out = append(out, views[id])
		}
		slices.SortFunc(out, less)
		return out, len(matched)
	}
	// A row whose parent is not included starts a tree at depth 0. Without a
	// root these are top-level tasks; with a root it is the root itself.
	children := map[int64][]View{}
	roots := []View{}
	for id := range included {
		v := views[id]
		v.Context = !matched[id]
		if v.ParentID != nil && included[*v.ParentID] {
			children[*v.ParentID] = append(children[*v.ParentID], v)
		} else {
			roots = append(roots, v)
		}
	}
	for p := range children {
		slices.SortFunc(children[p], less)
	}
	slices.SortFunc(roots, less)
	stack := []View{}
	for i := len(roots) - 1; i >= 0; i-- {
		stack = append(stack, roots[i])
	}
	for len(stack) > 0 {
		v := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		out = append(out, v)
		cs := children[v.ID]
		for i := len(cs) - 1; i >= 0; i-- {
			c := cs[i]
			c.Depth = v.Depth + 1
			stack = append(stack, c)
		}
	}
	return out, len(matched)
}
