package engine

import (
	"fmt"
	"slices"
	"strings"
	"tque/internal/model"
)

// Renumber changes display numbers globally, including hidden/completed tasks.
// Stable identity is never changed. Relationship numbers are remapped once
// from the original snapshot; persisted relationships remain internal IDs.
func Renumber(d *model.Document, from, to int64) error {
	if from <= 0 || to <= 0 {
		return model.Err("InvalidInput", "IDs must be positive")
	}
	found, occupied := false, false
	for _, t := range d.Tasks {
		found = found || t.ID == from
		occupied = occupied || t.ID == to
	}
	if !found {
		return model.Err("NotFound", "task #%d", from)
	}
	if from == to {
		return nil
	}
	numbers := map[int64]int64{from: to}
	if occupied {
		for _, t := range d.Tasks {
			if to < from && t.ID >= to && t.ID < from {
				numbers[t.ID] = t.ID + 1
			}
			if from < to && t.ID > from && t.ID <= to {
				numbers[t.ID] = t.ID - 1
			}
		}
	}
	translate := func(id int64) int64 {
		if n, ok := numbers[id]; ok {
			return n
		}
		return id
	}
	for i := range d.Tasks {
		t := &d.Tasks[i]
		t.ID = translate(t.ID)
		if t.ParentID != nil {
			p := translate(*t.ParentID)
			t.ParentID = &p
		}
		for j, dep := range t.DependsOn {
			t.DependsOn[j] = translate(dep)
		}
		slices.Sort(t.DependsOn)
	}
	return nil
}

func FormatIDChanges(changes []IDChange) string {
	parts := make([]string, 0, len(changes))
	for _, c := range changes {
		parts = append(parts, fmt.Sprintf("%d→%d", c.From, c.To))
	}
	return strings.Join(parts, ", ")
}
