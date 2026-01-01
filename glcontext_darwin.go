package unison

import (
	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/unison/internal/mac"
)

type glContext struct {
	pixelFormat mac.OpenGLPixelFormatRef
	ctx         mac.OpenGLContextRef
}

func (c *glContext) create(wnd, share *Window, transparent bool) error {
	pixFmt := mac.NewOpenGLPixelFormat()
	if pixFmt == 0 {
		return errs.New("failed to create OpenGL pixel format")
	}
	var shareCtx mac.OpenGLContextRef
	if share != nil {
		shareCtx = share.glCtx.ctx
	}
	ctx := mac.NewOpenGLContext(wnd.wnd.view, pixFmt, shareCtx, transparent)
	if ctx == 0 {
		pixFmt.Release()
		return errs.New("failed to create OpenGL context")
	}
	c.pixelFormat = pixFmt
	c.ctx = ctx
	return nil
}

func (c *glContext) makeCurrent() {
	c.ctx.MakeCurrent()
}

func (c *glContext) swapBuffers() {
	c.ctx.FlushBuffer()
}

func (c *glContext) destroy() {
	if c.ctx != 0 {
		c.ctx.Release()
		c.ctx = 0
	}
	if c.pixelFormat != 0 {
		c.pixelFormat.Release()
		c.pixelFormat = 0
	}
}

func clearOpenGLCurrentContext() {
	mac.ClearOpenGLCurrentContext()
	wndWithCurrentCtx = nil
}
