package engine

import (
	"slices"
	"strings"
	"testing"
	"tque/internal/model"
)

// 1 Blog
// ├─ 2 Design
// │  ├─ 3 Draft
// │  └─ 4 Review (done)
// └─ 5 Publish
// 6 Work
// └─ 7 design
// 8 "12"
// 12 Other
func rootDoc() model.Document {
	task := func(id int64, title string, parent int64) model.Task {
		t := model.Task{ID: id, Title: title, Status: "todo", ProgressMode: "auto"}
		if parent != 0 {
			t.ParentID = ptr(parent)
		}
		return t
	}
	d := model.Document{Tasks: []model.Task{
		task(1, "Blog", 0), task(2, "Design", 1), task(3, "Draft", 2), task(4, "Review", 2),
		task(5, "Publish", 1), task(6, "Work", 0), task(7, "design", 6), task(8, "12", 0), task(12, "Other", 0),
	}}
	d.Tasks[3].Status, d.Tasks[3].Progress, d.Tasks[3].CompletedAt = "done", 100, "2026-10-21T00:00:00Z"
	return d
}

func ids(rows []View) []int64 {
	out := []int64{}
	for _, r := range rows {
		out = append(out, r.ID)
	}
	return out
}

func TestSelectRootTree(t *testing.T) {
	d := rootDoc()
	rows, n := Select(d, Query{All: true, Tree: true, Root: 2})
	if !slices.Equal(ids(rows), []int64{2, 3, 4}) || n != 3 {
		t.Fatal(ids(rows), n)
	}
	if rows[0].Depth != 0 || rows[1].Depth != 1 || rows[0].Context {
		t.Fatal("root must start at depth 0", rows)
	}
	prefixes := TreePrefixes(rows)
	if prefixes[2] != "" || prefixes[3] != "├─ " || prefixes[4] != "└─ " {
		t.Fatal(prefixes)
	}
	for i := range rows {
		if TreeGap(rows, i) {
			t.Fatal("a single root has no separators")
		}
	}

	// Context ancestors stop at the root: Blog (#1) must not appear.
	rows, n = Select(d, Query{All: true, Tree: true, Root: 2, Search: "draft"})
	if !slices.Equal(ids(rows), []int64{2, 3}) || n != 1 || !rows[0].Context {
		t.Fatal(ids(rows), n)
	}
	// The default todo-only filter keeps the root as context for matches.
	rows, _ = Select(d, Query{Tree: true, Root: 2})
	if !slices.Equal(ids(rows), []int64{2, 3}) {
		t.Fatal(ids(rows))
	}
	// Flat lists are limited to the subtree too.
	rows, n = Select(d, Query{All: true, Root: 1})
	if !slices.Equal(ids(rows), []int64{1, 2, 3, 4, 5}) || n != 5 {
		t.Fatal(ids(rows), n)
	}
	if rows, _ := Select(d, Query{All: true, Tree: true, Root: 99}); len(rows) != 0 {
		t.Fatal("unknown root should select nothing", ids(rows))
	}
}

func TestSelectWithoutRootIsUnchanged(t *testing.T) {
	d := rootDoc()
	rows, _ := Select(d, Query{All: true, Tree: true})
	if !slices.Equal(ids(rows), []int64{1, 2, 3, 4, 5, 6, 7, 8, 12}) {
		t.Fatal(ids(rows))
	}
	gaps := []int64{}
	for i, r := range rows {
		if TreeGap(rows, i) {
			gaps = append(gaps, r.ID)
		}
	}
	if !slices.Equal(gaps, []int64{6, 8}) {
		t.Fatal("separators should precede roots after children", gaps)
	}
	if p := TreePrefixes(rows); p[1] != "" || p[3] != "│  ├─ " || p[5] != "└─ " {
		t.Fatal(p)
	}
}

func TestRootCandidates(t *testing.T) {
	d := rootDoc()
	got := func(v string) []int64 {
		out := []int64{}
		for _, c := range RootCandidates(d, v) {
			out = append(out, c.ID)
		}
		return out
	}
	if !slices.Equal(got("design"), []int64{2, 7}) {
		t.Fatal("title match ignores case", got("design"))
	}
	if !slices.Equal(got("2"), []int64{2}) {
		t.Fatal(got("2"))
	}
	if !slices.Equal(got("12"), []int64{8, 12}) {
		t.Fatal("numeric value matches both ID and title", got("12"))
	}
	if !slices.Equal(got("Review"), []int64{4}) {
		t.Fatal("completed tasks are candidates", got("Review"))
	}
	if len(got("Desig")) != 0 || len(got("99")) != 0 {
		t.Fatal("no partial or unknown matches")
	}
	d.Tasks = append(d.Tasks, model.Task{ID: 20, Title: "20", Status: "todo"})
	if !slices.Equal(got("20"), []int64{20}) {
		t.Fatal("a task matching by ID and title is listed once", got("20"))
	}
	if s := FormatRootCandidates(d, RootCandidates(d, "design")); s != "#2 Blog > Design, #7 Work > design" {
		t.Fatal(s)
	}
}

func TestRootValueAndPath(t *testing.T) {
	if v, e := RootValue("  Blog  "); e != nil || v != "Blog" {
		t.Fatal(v, e)
	}
	for _, bad := range []string{"", "   ", "a\tb", "\x1b"} {
		if _, e := RootValue(bad); e == nil || !strings.Contains(e.Error(), "InvalidInput") {
			t.Fatalf("%q: %v", bad, e)
		}
	}
	if p := TaskPath(rootDoc(), 3); !slices.Equal(p, []string{"Blog", "Design", "Draft"}) {
		t.Fatal(p)
	}
	if p := TaskPath(rootDoc(), 99); len(p) != 0 {
		t.Fatal(p)
	}
}
