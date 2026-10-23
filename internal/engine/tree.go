package engine

// TreePrefixes returns branch guides for the visible tree rows.
func TreePrefixes(rows []View) map[int64]string {
	last := map[int64]int64{}
	for _, v := range rows {
		p := int64(0)
		if v.ParentID != nil {
			p = *v.ParentID
		}
		last[p] = v.ID
	}
	prefixes := map[int64]string{}
	ancestorTail := map[int64]string{}
	for _, v := range rows {
		if v.ParentID == nil {
			prefixes[v.ID] = ""
			ancestorTail[v.ID] = ""
			continue
		}
		p := *v.ParentID
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

// TreeGap reports a transition from a visible child to a new root.
func TreeGap(rows []View, i int) bool {
	return i > 0 && i < len(rows) && rows[i].ParentID == nil && rows[i-1].ParentID != nil
}
