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
	NextID        int64  `json:"next_id"`
	Tasks         []Task `json:"tasks"`
}

func Empty() Document { return Document{SchemaVersion: 1, NextID: 1, Tasks: []Task{}} }

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
