package plaf

/*
#cgo darwin CFLAGS: -D_GLFW_COCOA -Wno-deprecated-declarations -x objective-c
#cgo darwin LDFLAGS: -framework Cocoa -framework IOKit -framework CoreVideo -framework OpenGL

#cgo linux CFLAGS: -D_GLFW_X11 -D_GNU_SOURCE
#cgo linux LDFLAGS: -lGL -lX11 -lXrandr -lXxf86vm -lXi -lXcursor -lm -lXinerama -ldl -lrt

#cgo windows CFLAGS: -D_GLFW_WIN32
#cgo windows LDFLAGS: -lgdi32 -lopengl32
*/
import "C"
