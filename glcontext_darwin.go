package unison

import (
	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/unison/internal/mac"
)

type apiGLContext struct {
	pixelFormat mac.OpenGLPixelFormatRef
	ctx         mac.OpenGLContextRef
}

func (c *apiGLContext) apiCreate(wnd, share *Window, transparent bool) error {
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

func (c *apiGLContext) apiMakeCurrent() {
	c.ctx.MakeCurrent()
}

func (c *apiGLContext) apiSwapBuffers() {
	c.ctx.FlushBuffer()
}

func (c *apiGLContext) apiDestroy() {
	if c.ctx != 0 {
		c.ctx.Release()
		c.ctx = 0
	}
	if c.pixelFormat != 0 {
		c.pixelFormat.Release()
		c.pixelFormat = 0
	}
}

func apiClearOpenGLCurrentContext() {
	mac.ClearOpenGLCurrentContext()
	wndWithCurrentCtx = nil
}
