package plaf2

import "slices"

var cursorList []*Cursor

type Cursor struct {
}

func (c *Cursor) Destroy() {
	if c == nil {
		return
	}
	for _, w := range windowList {
		if w.cursor == c {
			w.cursor = nil
			w.adjustToCursorChange()
		}
	}
	/* TODO
	   _plafDestroyCursor(cursor);
	*/
	cursorList = slices.DeleteFunc(cursorList, func(cur *Cursor) bool { return cur == c })
}
