package plaf

/*
#cgo darwin CFLAGS: -DPLATFORM_DARWIN -Wno-deprecated-declarations -x objective-c
#cgo darwin LDFLAGS: -framework Cocoa -framework IOKit -framework CoreVideo -framework OpenGL

#cgo linux CFLAGS: -DPLATFORM_LINUX
#cgo linux LDFLAGS: -lGL -lX11 -lXrandr -lXxf86vm -lXi -lXcursor -lm -lXinerama -ldl -lrt

#cgo windows CFLAGS: -DPLATFORM_WINDOWS
#cgo windows LDFLAGS: -lgdi32 -lopengl32
*/
import "C"
