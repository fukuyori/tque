package storage

import (
	"encoding/json"
	"fmt"
	"sort"
	"tque/internal/model"
	"tque/internal/strictjson"
)

// Display IDs remain the public interface. Only the persistence boundary uses
// stable relationship keys, so CLI JSON never exposes internal identities.
type storedTask struct {
	model.Task
	Key string `json:"internal_id"`
	// Shadow the embedded display-reference fields so they are never written.
	// Decode separately rejects these legacy fields in schema 3.
	ParentID       *int64   `json:"parent_id,omitempty"`
	DependsOn      []int64  `json:"depends_on,omitempty"`
	ParentKey      string   `json:"parent_internal_id,omitempty"`
	DependencyKeys []string `json:"depends_on_internal_ids,omitempty"`
}
type storedDocument struct {
	SchemaVersion int          `json:"schema_version"`
	Revision      int64        `json:"revision"`
	Tasks         []storedTask `json:"tasks"`
}

func encode(d model.Document) ([]byte, error) {
	keys := map[int64]string{}
	seen := map[string]bool{}
	for _, t := range d.Tasks {
		if t.InternalID == "" || seen[t.InternalID] {
			return nil, fmt.Errorf("missing/duplicate internal ID for #%d", t.ID)
		}
		keys[t.ID] = t.InternalID
		seen[t.InternalID] = true
	}
	out := storedDocument{SchemaVersion: model.SchemaVersion, Revision: d.Revision, Tasks: []storedTask{}}
	for _, t := range d.Tasks {
		s := storedTask{Task: t, Key: t.InternalID}
		if t.ParentID != nil {
			var ok bool
			s.ParentKey, ok = keys[*t.ParentID]
			if !ok {
				return nil, fmt.Errorf("missing parent of #%d", t.ID)
			}
		}
		for _, id := range t.DependsOn {
			key, ok := keys[id]
			if !ok {
				return nil, fmt.Errorf("missing dependency of #%d", t.ID)
			}
			s.DependencyKeys = append(s.DependencyKeys, key)
		}
		sort.Strings(s.DependencyKeys)
		out.Tasks = append(out.Tasks, s)
	}
	return json.MarshalIndent(out, "", "  ")
}

func decodeCurrent(data []byte) (model.Document, error) {
	var stored storedDocument
	if err := strictjson.Decode(data, &stored); err != nil {
		return model.Document{}, err
	}
	d := model.Document{SchemaVersion: stored.SchemaVersion, Revision: stored.Revision, Tasks: []model.Task{}}
	ids := map[string]int64{}
	for _, t := range stored.Tasks {
		if t.Key == "" || ids[t.Key] != 0 {
			return d, fmt.Errorf("missing/duplicate internal ID")
		}
		ids[t.Key] = t.ID
	}
	for _, s := range stored.Tasks {
		t := s.Task
		t.InternalID = s.Key
		if s.ParentKey != "" {
			id, ok := ids[s.ParentKey]
			if !ok {
				return d, fmt.Errorf("unknown parent internal ID for #%d", t.ID)
			}
			t.ParentID = &id
		}
		seen := map[string]bool{}
		for _, key := range s.DependencyKeys {
			id, ok := ids[key]
			if !ok || seen[key] {
				return d, fmt.Errorf("unknown/duplicate dependency internal ID for #%d", t.ID)
			}
			seen[key] = true
			t.DependsOn = append(t.DependsOn, id)
		}
		sort.Slice(t.DependsOn, func(i, j int) bool { return t.DependsOn[i] < t.DependsOn[j] })
		d.Tasks = append(d.Tasks, t)
	}
	return d, nil
}
