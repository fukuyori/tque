package engine

import (
	"fmt"
	"math"
	"reflect"
	"testing"
	"tque/internal/model"
)

func TestRenumberRelationsAndDirections(t *testing.T) {
	d := model.Empty()
	for i := 1; i <= 10; i++ {
		apply(t, &d, Request{Op: "add", Patch: Patch{Title: ptr(fmt.Sprint(i))}})
	}
	apply(t, &d, Request{Op: "edit", ID: 9, Patch: Patch{ParentSet: true, Parent: ptr(int64(8)), DepsSet: true, Deps: []int64{6, 7}}})
	apply(t, &d, Request{Op: "done", ID: 7})
	before := d.Clone()
	apply(t, &d, Request{Op: "renumber", ID: 8, NewID: 6})
	if d.Tasks[5].ID != 7 || d.Tasks[6].ID != 8 || d.Tasks[7].ID != 6 {
		t.Fatal(d.Tasks)
	}
	if *d.Tasks[8].ParentID != 6 || !reflect.DeepEqual(d.Tasks[8].DependsOn, []int64{7, 8}) {
		t.Fatal(d.Tasks[8])
	}
	if !reflect.DeepEqual(Views(d)[9].BlockedBy, []int64{7}) {
		t.Fatal("blocked IDs did not follow renumbering")
	}
	for i, task := range d.Tasks {
		if task.InternalID != before.Tasks[i].InternalID || task.Title != before.Tasks[i].Title || task.Status != before.Tasks[i].Status {
			t.Fatal("identity or content changed")
		}
	}
	apply(t, &d, Request{Op: "renumber", ID: 6, NewID: 8})
	if !reflect.DeepEqual(d, before) {
		t.Fatal("reverse move did not restore original document")
	}
	apply(t, &d, Request{Op: "renumber", ID: 8, NewID: 20})
	if d.Tasks[5].ID != 6 || d.Tasks[6].ID != 7 || *d.Tasks[8].ParentID != 20 {
		t.Fatal("empty destination shifted unrelated tasks")
	}
	apply(t, &d, Request{Op: "renumber", ID: 20, NewID: 20})
	for _, pair := range [][2]int64{{20, 0}, {20, -1}, {999, 1}} {
		copy := d.Clone()
		if err := Renumber(&copy, pair[0], pair[1]); err == nil || !reflect.DeepEqual(copy, d) {
			t.Fatal("invalid operation changed document", pair)
		}
	}
}

func TestRenumberGapsAndLimits(t *testing.T) {
	d := model.Empty()
	for _, name := range []string{"a", "b", "c"} {
		apply(t, &d, Request{Op: "add", Patch: Patch{Title: ptr(name)}})
	}
	apply(t, &d, Request{Op: "renumber", ID: 3, NewID: math.MaxInt64})
	apply(t, &d, Request{Op: "renumber", ID: math.MaxInt64, NewID: 1})
	if d.Tasks[0].ID != 2 || d.Tasks[1].ID != 3 || d.Tasks[2].ID != 1 {
		t.Fatal(d)
	}
	apply(t, &d, Request{Op: "renumber", ID: 1, NewID: math.MaxInt64})
	if _, err := NewID(d.Tasks); err == nil {
		t.Fatal("ID overflow accepted")
	}
}
