package plaf

/*
#cgo darwin CFLAGS: -DPLAF_DARWIN -Wno-deprecated-declarations -x objective-c
#cgo darwin LDFLAGS: -framework Cocoa -framework IOKit -framework CoreVideo -framework OpenGL

#cgo linux CFLAGS: -DPLAF_LINUX
#cgo linux LDFLAGS: -lGL -lX11 -lXrandr -lXxf86vm -lXi -lXcursor -lm -lXinerama -ldl -lrt

#cgo windows CFLAGS: -DPLAF_WINDOWS
#cgo windows LDFLAGS: -lgdi32 -lopengl32
*/
import "C"
