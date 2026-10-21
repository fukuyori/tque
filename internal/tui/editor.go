package tui

import "github.com/rivo/uniseg"

// editor keeps the entire draft; validation, not the widget, enforces title limits.
type editor struct {
	text string
	pos  int
}

func (e *editor) set(s string)    { e.text = s; e.pos = len(s) }
func (e *editor) insert(s string) { e.text = e.text[:e.pos] + s + e.text[e.pos:]; e.pos += len(s) }
func (e *editor) left() int {
	g := uniseg.NewGraphemes(e.text)
	prev := 0
	for g.Next() {
		start, end := g.Positions()
		if end >= e.pos {
			return start
		}
		prev = end
	}
	return prev
}
func (e *editor) right() int {
	g := uniseg.NewGraphemes(e.text)
	for g.Next() {
		_, end := g.Positions()
		if end > e.pos {
			return end
		}
	}
	return len(e.text)
}
func (e *editor) key(k string) {
	switch k {
	case "left":
		e.pos = e.left()
	case "right":
		e.pos = e.right()
	case "home", "ctrl+a":
		e.pos = 0
	case "end", "ctrl+e":
		e.pos = len(e.text)
	case "backspace":
		p := e.left()
		e.text = e.text[:p] + e.text[e.pos:]
		e.pos = p
	case "delete":
		e.text = e.text[:e.pos] + e.text[e.right():]
	}
}
