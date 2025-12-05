package plaf2

import (
	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/unison/internal/mac"
)

type platformGraphicsContext struct {
	pixelFormat mac.OpenGLPixelFormatRef
	ctx         mac.OpenGLContextRef
}

func (w *Window) createOpenGLContext(share *Window, transparent bool) error {
	pixFmt := mac.NewOpenGLPixelFormat()
	if pixFmt == 0 {
		return errs.New("failed to create OpenGL pixel format")
	}
	var shareCtx mac.OpenGLContextRef
	if share != nil {
		shareCtx = share.plGctx.ctx
	}
	ctx := mac.NewOpenGLContext(w.plWnd.view, pixFmt, shareCtx, transparent)
	if ctx == 0 {
		pixFmt.Release()
		return errs.New("failed to create OpenGL context")
	}
	w.plGctx.pixelFormat = pixFmt
	w.plGctx.ctx = ctx
	return nil
}

func (c *platformGraphicsContext) MakeCurrent() {
	c.ctx.MakeCurrent()
}

func (c *platformGraphicsContext) SwapBuffers() {
	c.ctx.FlushBuffer()
}

func (c *platformGraphicsContext) destroy() {
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
}
