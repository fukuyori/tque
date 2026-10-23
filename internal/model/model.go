package model

import "fmt"

type Task struct {
	// InternalID is stable identity. ID and the in-memory relationship fields
	// are display numbers; storage translates relationships to stable identities.
	InternalID   string  `json:"-"`
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

// SchemaVersion stores relationships using stable internal identities.
// Storage also reads version 2 and migrates it on the next successful write.
const SchemaVersion = 3

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
