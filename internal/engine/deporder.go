package engine

import (
	"container/heap"
	"slices"
)

type orderHeap struct {
	ids  []int64
	less func(a, b int64) bool
}

func (h *orderHeap) Len() int           { return len(h.ids) }
func (h *orderHeap) Less(i, j int) bool { return h.less(h.ids[i], h.ids[j]) }
func (h *orderHeap) Swap(i, j int)      { h.ids[i], h.ids[j] = h.ids[j], h.ids[i] }
func (h *orderHeap) Push(x any)         { h.ids = append(h.ids, x.(int64)) }
func (h *orderHeap) Pop() any {
	n := len(h.ids) - 1
	id := h.ids[n]
	h.ids = h.ids[:n]
	return id
}

// dependencyOrder returns ids so that every id follows the ids in before[id].
// Ties follow cmp, so unrelated ids keep the selected sort order. A cycle,
// which only sibling groups can form, is broken at its first id by cmp.
func dependencyOrder(ids []int64, before map[int64][]int64, cmp func(a, b int64) int) []int64 {
	sorted := slices.Clone(ids)
	slices.SortFunc(sorted, cmp)
	waiting := map[int64]int{}
	after := map[int64][]int64{}
	for _, id := range sorted {
		for _, b := range before[id] {
			waiting[id]++
			after[b] = append(after[b], id)
		}
	}
	ready := &orderHeap{less: func(a, b int64) bool { return cmp(a, b) < 0 }}
	for _, id := range sorted {
		if waiting[id] == 0 {
			ready.ids = append(ready.ids, id)
		}
	}
	heap.Init(ready)
	out := make([]int64, 0, len(sorted))
	emitted := map[int64]bool{}
	emit := func(id int64) {
		emitted[id] = true
		out = append(out, id)
		for _, next := range after[id] {
			waiting[next]--
			if waiting[next] == 0 && !emitted[next] {
				heap.Push(ready, next)
			}
		}
	}
	for next := 0; len(out) < len(sorted); {
		if ready.Len() > 0 {
			emit(heap.Pop(ready).(int64))
			continue
		}
		for emitted[sorted[next]] {
			next++
		}
		emit(sorted[next])
	}
	return out
}

// displayRanks orders rows so that a task appears below what it depends on.
// In a tree, only siblings can be reordered: a task moves below the sibling
// that is, or contains, one of its own dependencies. Dependencies held by its
// descendants do not move it. A flat list has no branches, so every dependency
// applies. Ranks are computed over all tasks, so hidden tasks still order the
// visible ones.
func displayRanks(views map[int64]View, tree bool, cmp func(a, b View) int) map[int64]int {
	byID := func(a, b int64) int { return cmp(views[a], views[b]) }
	rank := make(map[int64]int, len(views))
	number := func(ids []int64, before map[int64][]int64) {
		for i, id := range dependencyOrder(ids, before, byID) {
			rank[id] = i
		}
	}
	before := map[int64][]int64{}
	if !tree {
		ids := make([]int64, 0, len(views))
		for id, v := range views {
			ids = append(ids, id)
			before[id] = v.DependsOn
		}
		number(ids, before)
		return rank
	}
	parent := func(id int64) int64 {
		if p := views[id].ParentID; p != nil {
			return *p
		}
		return 0
	}
	groups := map[int64][]int64{}
	for id, v := range views {
		group := parent(id)
		groups[group] = append(groups[group], id)
		for _, dep := range v.DependsOn {
			// Climb from the dependency to the row that shares this task's parent.
			sibling := dep
			for sibling != 0 && parent(sibling) != group {
				sibling = parent(sibling)
			}
			if sibling != 0 && sibling != id && !slices.Contains(before[id], sibling) {
				before[id] = append(before[id], sibling)
			}
		}
	}
	for _, ids := range groups {
		number(ids, before)
	}
	return rank
}
