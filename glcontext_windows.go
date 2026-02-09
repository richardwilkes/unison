package unison

import (
	"unsafe"

	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/unison/internal/w32"
)

type glContext struct {
	dc w32.HDC
	rc w32.HGLRC
}

func (c *glContext) create(wnd, share *Window, transparent bool) error {
	dc := w32.GetDC(wnd.wnd.wnd)
	if dc == 0 {
		return errs.New("failed to get device context for window")
	}
	var shareCtx w32.HGLRC
	if share != nil {
		shareCtx = share.glCtx.rc
	}
	pixelFormat := choosePixelFormat(dc)
	if pixelFormat == 0 {
		return errs.New("failed to choose pixel format for OpenGL context")
	}
	var pfd w32.PIXELFORMATDESCRIPTOR
	if w32.DescribePixelFormat(dc, pixelFormat, uint32(unsafe.Sizeof(pfd)), &pfd) == 0 {
		return errs.New("failed to describe pixel format for OpenGL context")
	}
	if !w32.SetPixelFormat(dc, pixelFormat, &pfd) {
		return errs.New("failed to set pixel format for OpenGL context")
	}
	fakeRC := w32.WglCreateContext(dc)
	if !w32.WglMakeCurrent(dc, fakeRC) {
		return errs.New("failed to make fake OpenGL context current")
	}
	defer func() {
		w32.WglMakeCurrent(0, 0)
		w32.WglDeleteContext(fakeRC)
	}()
	rc := w32.WglCreateContextAttribsARB(dc, shareCtx, []int32{
		w32.WGL_CONTEXT_MAJOR_VERSION_ARB, 3,
		w32.WGL_CONTEXT_MINOR_VERSION_ARB, 2,
		0,
	})
	if rc == 0 {
		return errs.New("failed to create OpenGL context")
	}
	c.dc = dc
	c.rc = rc
	return nil
}

func (c *glContext) makeCurrent() {
	w32.WglMakeCurrent(c.dc, c.rc)
}

func (c *glContext) swapBuffers() {
	w32.SwapBuffers(c.dc)
}

func (c *glContext) destroy() {
	if c.rc != 0 {
		w32.WglDeleteContext(c.rc)
		c.rc = 0
	}
}

func clearOpenGLCurrentContext() {
	w32.WglMakeCurrent(0, 0)
	wndWithCurrentCtx = nil
}

func getPixelFormatAttribValue(attrib int32, attribs, values []int32) int32 {
	for i, a := range attribs {
		if a == attrib {
			return values[i]
		}
	}
	return 0
}

func choosePixelFormat(dc w32.HDC) int32 {
	var pfd w32.PIXELFORMATDESCRIPTOR
	count := w32.DescribePixelFormat(dc, 1, uint32(unsafe.Sizeof(pfd)), nil)
	for i := int32(1); i <= count; i++ {
		w32.DescribePixelFormat(dc, i, uint32(unsafe.Sizeof(pfd)), &pfd)
		if pfd.DwFlags&w32.PFD_DRAW_TO_WINDOW == 0 || pfd.DwFlags&w32.PFD_SUPPORT_OPENGL == 0 {
			continue
		}
		if pfd.DwFlags&w32.PFD_GENERIC_ACCELERATED == 0 && pfd.DwFlags&w32.PFD_GENERIC_FORMAT != 0 {
			continue
		}
		if pfd.IPixelType != w32.PFD_TYPE_RGBA {
			continue
		}
		if pfd.DwFlags&w32.PFD_DOUBLEBUFFER == 0 {
			continue
		}
		if pfd.RedBits != 8 || pfd.GreenBits != 8 || pfd.BlueBits != 8 || pfd.AlphaBits != 8 {
			continue
		}
		if pfd.DepthBits != 24 || pfd.StencilBits != 8 {
			continue
		}
		return i
	}

	// if values := w32.WglGetPixelFormatAttribivARB(dc, 1, 0, []int32{w32.WGL_NUMBER_PIXEL_FORMATS_ARB}); count > values[0] {
	// 	count = values[0]
	// }
	// attribs := []int32{
	// 	w32.WGL_SUPPORT_OPENGL_ARB,
	// 	w32.WGL_DRAW_TO_WINDOW_ARB,
	// 	w32.WGL_PIXEL_TYPE_ARB,
	// 	w32.WGL_ACCELERATION_ARB,
	// 	w32.WGL_RED_BITS_ARB,
	// 	w32.WGL_GREEN_BITS_ARB,
	// 	w32.WGL_BLUE_BITS_ARB,
	// 	w32.WGL_ALPHA_BITS_ARB,
	// 	w32.WGL_DEPTH_BITS_ARB,
	// 	w32.WGL_STENCIL_BITS_ARB,
	// 	0,
	// }
	// for i := int32(1); i <= count; i++ {
	// 	values := w32.WglGetPixelFormatAttribivARB(dc, i, 0, attribs)
	// 	if getPixelFormatAttribValue(w32.WGL_SUPPORT_OPENGL_ARB, attribs, values) != 0 &&
	// 		getPixelFormatAttribValue(w32.WGL_DRAW_TO_WINDOW_ARB, attribs, values) != 0 &&
	// 		getPixelFormatAttribValue(w32.WGL_PIXEL_TYPE_ARB, attribs, values) == w32.WGL_TYPE_RGBA_ARB &&
	// 		getPixelFormatAttribValue(w32.WGL_ACCELERATION_ARB, attribs, values) != w32.WGL_NO_ACCELERATION_ARB &&
	// 		getPixelFormatAttribValue(w32.WGL_RED_BITS_ARB, attribs, values) == 8 &&
	// 		getPixelFormatAttribValue(w32.WGL_GREEN_BITS_ARB, attribs, values) == 8 &&
	// 		getPixelFormatAttribValue(w32.WGL_BLUE_BITS_ARB, attribs, values) == 8 &&
	// 		getPixelFormatAttribValue(w32.WGL_ALPHA_BITS_ARB, attribs, values) == 8 &&
	// 		getPixelFormatAttribValue(w32.WGL_DEPTH_BITS_ARB, attribs, values) == 24 &&
	// 		getPixelFormatAttribValue(w32.WGL_STENCIL_BITS_ARB, attribs, values) == 8 {
	// 		return i
	// 	}
	// }
	return 0
}
