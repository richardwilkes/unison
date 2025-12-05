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

func NewCursor(img *image.NRGBA, xhot, yhot int) *Cursor { // formerly plafCreateCursor
	nsCursor := mac.NewCursor(img, xhot, yhot)
	if nsCursor == 0 {
		return nil
	}
	c := &Cursor{plCursor: nsCursor}
	cursorList = append(cursorList, c)
	return c
}

func (c *Cursor) destroy() { // formerly _plafDestroyCursor
	if c.plCursor != 0 {
		c.plCursor.Release()
		c.plCursor = 0
	}
}

func ArrowCursor() *Cursor { // formerly plafCreateStandardCursor
	if arrowCursor == nil {
		arrowCursor = &Cursor{plCursor: mac.ArrowCursor()}
		cursorList = append(cursorList, arrowCursor)
	}
	return arrowCursor
}

func IBeamCursor() *Cursor { // formerly plafCreateStandardCursor
	if ibeamCursor == nil {
		ibeamCursor = &Cursor{plCursor: mac.IBeamCursor()}
		cursorList = append(cursorList, ibeamCursor)
	}
	return ibeamCursor
}

func CrosshairCursor() *Cursor { // formerly plafCreateStandardCursor
	if crosshairCursor == nil {
		crosshairCursor = &Cursor{plCursor: mac.CrosshairCursor()}
		cursorList = append(cursorList, crosshairCursor)
	}
	return crosshairCursor
}

func PointingHandCursor() *Cursor { // formerly plafCreateStandardCursor
	if pointingHandCursor == nil {
		pointingHandCursor = &Cursor{plCursor: mac.PointingHandCursor()}
		cursorList = append(cursorList, pointingHandCursor)
	}
	return pointingHandCursor
}

func ResizeLeftRightCursor() *Cursor { // formerly plafCreateStandardCursor
	if resizeLeftRightCursor == nil {
		resizeLeftRightCursor = &Cursor{plCursor: mac.ResizeLeftRightCursor()}
		cursorList = append(cursorList, resizeLeftRightCursor)
	}
	return resizeLeftRightCursor
}

func ResizeUpDownCursor() *Cursor { // formerly plafCreateStandardCursor
	if resizeUpDownCursor == nil {
		resizeUpDownCursor = &Cursor{plCursor: mac.ResizeUpDownCursor()}
		cursorList = append(cursorList, resizeUpDownCursor)
	}
	return resizeUpDownCursor
}
