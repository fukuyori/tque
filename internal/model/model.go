package model

import "fmt"

type Task struct {
	ID           int64   `json:"id"`
	Title        string  `json:"title"`
	Status       string  `json:"status"`
	ParentID     *int64  `json:"parent_id,omitempty"`
	DependsOn    []int64 `json:"depends_on,omitempty"`
	StartDate    string  `json:"start_date,omitempty"`
	DueDate      string  `json:"due_date,omitempty"`
	Progress     int     `json:"progress"`
	ProgressMode string  `json:"progress_mode"`
	CreatedAt    string  `json:"created_at"`
	CompletedAt  string  `json:"completed_at,omitempty"`
}

type Document struct {
	SchemaVersion int    `json:"schema_version"`
	Revision      int64  `json:"revision"`
	Tasks         []Task `json:"tasks"`
}

// SchemaVersion is the only supported storage format. Version 1 stored next_id.
const SchemaVersion = 2

func Empty() Document { return Document{SchemaVersion: SchemaVersion, Tasks: []Task{}} }

func (d Document) Clone() Document {
	d.Tasks = append([]Task{}, d.Tasks...)
	for i := range d.Tasks {
		d.Tasks[i].DependsOn = append([]int64(nil), d.Tasks[i].DependsOn...)
		if p := d.Tasks[i].ParentID; p != nil {
			v := *p
			d.Tasks[i].ParentID = &v
		}
	}
	return d
}

type Error struct{ Kind, Message string }

func (e *Error) Error() string                   { return e.Kind + ": " + e.Message }
func Err(kind, format string, args ...any) error { return &Error{kind, fmt.Sprintf(format, args...)} }
