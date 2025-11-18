package plaf

/*
#cgo darwin CFLAGS: -Wno-deprecated-declarations -x objective-c
#cgo darwin LDFLAGS: -framework Cocoa -framework CoreVideo -framework OpenGL

#cgo linux LDFLAGS: -lGL -lX11 -lXrandr -lXxf86vm -lXi -lXcursor -lm -lXinerama -ldl -lrt

#cgo windows LDFLAGS: -lgdi32 -lopengl32
*/
import "C"
