package engine

// TreePrefixes returns branch guides for the visible tree rows.
// Depth-0 rows start a tree, so a root task below the top level has no guide.
func TreePrefixes(rows []View) map[int64]string {
	parent := func(v View) int64 {
		if v.Depth == 0 || v.ParentID == nil {
			return 0
		}
		return *v.ParentID
	}
	last := map[int64]int64{}
	for _, v := range rows {
		last[parent(v)] = v.ID
	}
	prefixes := map[int64]string{}
	ancestorTail := map[int64]string{}
	for _, v := range rows {
		p := parent(v)
		if p == 0 {
			prefixes[v.ID] = ""
			ancestorTail[v.ID] = ""
			continue
		}
		stem := ancestorTail[p]
		branch := "├─ "
		tail := "│  "
		if last[p] == v.ID {
			branch = "└─ "
			tail = "   "
		}
		prefixes[v.ID] = stem + branch
		ancestorTail[v.ID] = stem + tail
	}
	return prefixes
}

// TreeGap reports a transition from a visible child to a new tree start.
func TreeGap(rows []View, i int) bool {
	return i > 0 && i < len(rows) && rows[i].Depth == 0 && rows[i-1].Depth > 0
}
