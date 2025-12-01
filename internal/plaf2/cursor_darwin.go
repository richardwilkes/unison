package plaf2

import (
	"image"

	"github.com/richardwilkes/unison/internal/mac"
)

type plafCursor = mac.Cursor

var (
	arrowCursor           *Cursor
	ibeamCursor           *Cursor
	crosshairCursor       *Cursor
	pointingHandCursor    *Cursor
	resizeLeftRightCursor *Cursor
	resizeUpDownCursor    *Cursor
)

func NewCursor(img *image.NRGBA, xhot, yhot int) *Cursor {
	nsCursor := mac.NewCursor(img, xhot, yhot)
	if nsCursor == 0 {
		return nil
	}
	c := &Cursor{nativeCursor: nsCursor}
	cursorList = append(cursorList, c)
	return c
}

func (c *Cursor) destroy() {
	if c.nativeCursor != 0 {
		c.nativeCursor.Release()
		c.nativeCursor = 0
	}
}

func ArrowCursor() *Cursor {
	if arrowCursor == nil {
		arrowCursor = &Cursor{nativeCursor: mac.ArrowCursor()}
		cursorList = append(cursorList, arrowCursor)
	}
	return arrowCursor
}

func IBeamCursor() *Cursor {
	if ibeamCursor == nil {
		ibeamCursor = &Cursor{nativeCursor: mac.IBeamCursor()}
		cursorList = append(cursorList, ibeamCursor)
	}
	return ibeamCursor
}

func CrosshairCursor() *Cursor {
	if crosshairCursor == nil {
		crosshairCursor = &Cursor{nativeCursor: mac.CrosshairCursor()}
		cursorList = append(cursorList, crosshairCursor)
	}
	return crosshairCursor
}

func PointingHandCursor() *Cursor {
	if pointingHandCursor == nil {
		pointingHandCursor = &Cursor{nativeCursor: mac.PointingHandCursor()}
		cursorList = append(cursorList, pointingHandCursor)
	}
	return pointingHandCursor
}

func ResizeLeftRightCursor() *Cursor {
	if resizeLeftRightCursor == nil {
		resizeLeftRightCursor = &Cursor{nativeCursor: mac.ResizeLeftRightCursor()}
		cursorList = append(cursorList, resizeLeftRightCursor)
	}
	return resizeLeftRightCursor
}

func ResizeUpDownCursor() *Cursor {
	if resizeUpDownCursor == nil {
		resizeUpDownCursor = &Cursor{nativeCursor: mac.ResizeUpDownCursor()}
		cursorList = append(cursorList, resizeUpDownCursor)
	}
	return resizeUpDownCursor
}
