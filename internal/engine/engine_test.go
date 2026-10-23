package engine

import (
	"strconv"
	"testing"
	"time"
	"tque/internal/model"
)

func ptr[T any](v T) *T { return &v }
func apply(t *testing.T, d *model.Document, r Request) {
	t.Helper()
	if _, _, e := Apply(d, r, time.Date(2026, 10, 21, 0, 0, 0, 0, time.UTC)); e != nil {
		t.Fatal(e)
	}
	if e := Validate(*d); e != nil {
		t.Fatal(e)
	}
}

func TestProgressAndStatus(t *testing.T) {
	d := model.Empty()
	apply(t, &d, Request{Op: "add", Patch: Patch{Title: ptr("親")}})
	apply(t, &d, Request{Op: "add", Patch: Patch{Title: ptr("子"), ParentSet: true, Parent: ptr(int64(1)), Progress: ptr("61")}})
	if Views(d)[1].EffectiveProgress != 61 {
		t.Fatal(Views(d))
	}
	apply(t, &d, Request{Op: "done", ID: 1})
	apply(t, &d, Request{Op: "undone", ID: 1})
	if d.Tasks[0].Progress != 100 || Views(d)[1].EffectiveProgress != 61 || d.Tasks[0].Status != "todo" {
		t.Fatal(d)
	}
	apply(t, &d, Request{Op: "rm", ID: 2})
	if Views(d)[1].EffectiveProgress != 100 {
		t.Fatal(d)
	}
	apply(t, &d, Request{Op: "add", Patch: Patch{Title: ptr("新しい子"), ParentSet: true, Parent: ptr(int64(1))}})
	if d.Tasks[1].ID != 2 {
		t.Fatal("new ID is not highest remaining ID plus one", d.Tasks[1].ID)
	}
	apply(t, &d, Request{Op: "edit", ID: 1, Patch: Patch{Progress: ptr("50")}})
	apply(t, &d, Request{Op: "done", ID: 2})
	if Views(d)[1].EffectiveProgress != 50 {
		t.Fatal("manual overwritten")
	}
	apply(t, &d, Request{Op: "edit", ID: 1, Patch: Patch{Progress: ptr("auto")}})
	if Views(d)[1].EffectiveProgress != 100 || d.Tasks[0].Progress != 50 {
		t.Fatal("auto lost stored progress")
	}
}

func TestNewID(t *testing.T) {
	tasks := func(ids ...int64) []model.Task {
		out := []model.Task{}
		for _, id := range ids {
			out = append(out, model.Task{ID: id})
		}
		return out
	}
	for _, c := range []struct {
		remaining []int64
		want      int64
	}{{nil, 1}, {[]int64{2, 3}, 4}, {[]int64{1, 2}, 3}, {[]int64{1}, 2}, {[]int64{5, 2}, 6}} {
		if got, e := NewID(tasks(c.remaining...)); e != nil || got != c.want {
			t.Fatal(c.remaining, got, e)
		}
	}
	if _, e := NewID(tasks(9223372036854775807)); e == nil {
		t.Fatal("ID limit accepted")
	}
}

func TestAddAfterRemoval(t *testing.T) {
	for _, c := range []struct {
		removed []int64
		want    []int64
	}{{[]int64{1}, []int64{2, 3, 4}}, {[]int64{3}, []int64{1, 2, 3}}, {[]int64{3, 2}, []int64{1, 2}}, {[]int64{3, 2, 1}, []int64{1}}} {
		d := model.Empty()
		for _, title := range []string{"a", "b", "c"} {
			apply(t, &d, Request{Op: "add", Patch: Patch{Title: ptr(title)}})
		}
		for _, id := range c.removed {
			apply(t, &d, Request{Op: "rm", ID: id})
		}
		apply(t, &d, Request{Op: "add", Patch: Patch{Title: ptr("new")}})
		got := []int64{}
		for _, task := range d.Tasks {
			got = append(got, task.ID)
		}
		if len(got) != len(c.want) {
			t.Fatal(c.removed, got)
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Fatal(c.removed, got)
			}
		}
		if d.Tasks[len(d.Tasks)-1].Title != "new" {
			t.Fatal("new task is not the last appended task", d.Tasks)
		}
	}
}

func TestGraphsRemovalAndFilters(t *testing.T) {
	d := model.Empty()
	apply(t, &d, Request{Op: "add", Patch: Patch{Title: ptr("parent")}})
	apply(t, &d, Request{Op: "add", Patch: Patch{Title: ptr("日本語 child"), ParentSet: true, Parent: ptr(int64(1)), Due: ptr("today")}})
	// Parent depending on its child is allowed; graphs are distinct.
	apply(t, &d, Request{Op: "edit", ID: 1, Patch: Patch{DepsSet: true, Deps: []int64{2}}})
	if _, e := Removal(d, 2, false); e == nil {
		t.Fatal("external dependency not protected")
	}
	if ids, e := Removal(d, 1, true); e != nil || len(ids) != 2 {
		t.Fatal(ids, e)
	}
	copy := d.Clone()
	_, _, _ = Apply(&copy, Request{Op: "edit", ID: 2, Patch: Patch{DepsSet: true, Deps: []int64{1}}}, time.Now())
	if e := Validate(copy); e == nil {
		t.Fatal("dependency cycle accepted")
	}
	rows, n := Select(d, Query{All: true, Tree: true, Search: "CHILD", Today: "2026-10-21"})
	if n != 1 || len(rows) != 2 || !rows[0].Context || rows[1].Context {
		t.Fatal(rows, n)
	}
	rows, _ = Select(d, Query{All: true, DueToday: true, Today: "2026-10-21"})
	if len(rows) != 1 || rows[0].ID != 2 {
		t.Fatal(rows)
	}
	apply(t, &d, Request{Op: "done", ID: 1})
	if !Views(d)[1].Blocked {
		t.Fatal("done must retain dependency warning")
	}
}

func TestDatesAndTitles(t *testing.T) {
	now := time.Date(2024, 2, 28, 23, 0, 0, 0, time.FixedZone("local", 9*3600))
	for in, want := range map[string]string{"+1d": "2024-02-29", "+2d": "2024-03-01", "today": "2024-02-28", "0001-01-01": "0001-01-01"} {
		v, e := Date(in, now)
		if e != nil || v != want {
			t.Fatal(in, v, e)
		}
	}
	for _, in := range []string{"2023-02-29", "2024-2-28", "0000-01-01", "+36501d", "+-1d"} {
		if _, e := Date(in, now); e == nil {
			t.Fatal(in)
		}
	}
	for _, in := range []string{"", "   ", "title\n", "\x1b[31m", "\xff"} {
		if _, e := Title(in); e == nil {
			t.Fatal(in)
		}
	}
	if s, e := Title("  日本語 👨‍👩‍👧  "); e != nil || s != "日本語 👨‍👩‍👧" {
		t.Fatal(s, e)
	}
}

func TestDeepHierarchy(t *testing.T) {
	d := model.Empty()
	for i := int64(1); i <= 1001; i++ {
		task := model.Task{InternalID: strconv.FormatInt(i, 10), ID: i, Title: "task", Status: "todo", ProgressMode: "auto", CreatedAt: "2026-10-21T00:00:00Z"}
		if i > 1 {
			task.ParentID = ptr(i - 1)
		}
		if i == 1001 {
			task.Progress = 77
		}
		d.Tasks = append(d.Tasks, task)
	}
	if e := Validate(d); e != nil {
		t.Fatal(e)
	}
	if Views(d)[1].EffectiveProgress != 77 {
		t.Fatal("deep aggregation")
	}
}
