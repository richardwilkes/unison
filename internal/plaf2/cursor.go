package plaf2

import "slices"

var cursorList []*Cursor

type Cursor struct {
	plCursor plafCursor
}

func (c *Cursor) Destroy() { // formerly plafDestroyCursor
	if c == nil {
		return
	}
	for _, w := range windowList {
		if w.cursor == c {
			w.cursor = nil
			w.adjustToCursorChange()
		}
	}
	cursorList = slices.DeleteFunc(cursorList, func(cur *Cursor) bool { return cur == c })
	c.destroy()
}
