package engine

import (
	"slices"
	"testing"
	"tque/internal/model"
)

// 1 Plan
// └─ 2 Requirements
// 3 Research
// └─ 4 Survey
func orderDoc(deps map[int64][]int64) model.Document {
	d := model.Document{Tasks: []model.Task{
		{ID: 1, Title: "Plan", Status: "todo"}, {ID: 2, Title: "Requirements", Status: "todo", ParentID: ptr(int64(1))},
		{ID: 3, Title: "Research", Status: "todo"}, {ID: 4, Title: "Survey", Status: "todo", ParentID: ptr(int64(3))},
	}}
	for i := range d.Tasks {
		d.Tasks[i].DependsOn = deps[d.Tasks[i].ID]
	}
	return d
}

func TestTreeOrdersSiblingsByDependency(t *testing.T) {
	for _, c := range []struct {
		name string
		deps map[int64][]int64
		want []int64
	}{
		{"no dependencies", nil, []int64{1, 2, 3, 4}},
		{"child depends on a child of another parent", map[int64][]int64{2: {4}}, []int64{1, 2, 3, 4}},
		{"parent depends on another parent", map[int64][]int64{1: {3}}, []int64{3, 4, 1, 2}},
		{"parent depends on a child of another parent", map[int64][]int64{1: {4}}, []int64{3, 4, 1, 2}},
		{"parent depends on its own child", map[int64][]int64{1: {2}}, []int64{1, 2, 3, 4}},
		{"branches depend on each other", map[int64][]int64{1: {4}, 3: {2}}, []int64{1, 2, 3, 4}},
	} {
		rows, _ := Select(orderDoc(c.deps), Query{All: true, Tree: true, Sort: "id"})
		if got := ids(rows); !slices.Equal(got, c.want) {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

func TestFlatListOrdersByDependency(t *testing.T) {
	rows, _ := Select(orderDoc(map[int64][]int64{2: {4}}), Query{All: true, Sort: "id"})
	if got := ids(rows); !slices.Equal(got, []int64{1, 3, 4, 2}) {
		t.Fatal(got)
	}
}

func TestDependencyOrderWithSortAndHiddenTasks(t *testing.T) {
	// 1 depends on 2 (done), which depends on 3. 4 is unrelated.
	d := model.Document{Tasks: []model.Task{
		{ID: 1, Title: "a", Status: "todo", DueDate: "2026-10-01", DependsOn: []int64{2}},
		{ID: 2, Title: "b", Status: "done", DependsOn: []int64{3}},
		{ID: 3, Title: "c", Status: "todo", DueDate: "2026-10-03"},
		{ID: 4, Title: "d", Status: "todo", DueDate: "2026-10-02"},
	}}
	for _, c := range []struct {
		q    Query
		want []int64
	}{
		// The hidden done task still keeps 3 above 1.
		{Query{Tree: true, Sort: "id"}, []int64{3, 1, 4}},
		{Query{Sort: "id"}, []int64{3, 1, 4}},
		// Due order applies to unrelated rows; 1 waits for 3 despite its earlier date.
		{Query{Tree: true, Sort: "due"}, []int64{4, 3, 1}},
		{Query{All: true, Tree: true, Sort: "name"}, []int64{3, 2, 1, 4}},
	} {
		rows, _ := Select(d, c.q)
		if got := ids(rows); !slices.Equal(got, c.want) {
			t.Errorf("%+v: got %v, want %v", c.q, got, c.want)
		}
	}
}
