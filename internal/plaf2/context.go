package plaf2

var wndWithCurrentCtx *Window

func ClearOpenGLCurrentContext() {
	clearOpenGLCurrentContext()
	wndWithCurrentCtx = nil
}
