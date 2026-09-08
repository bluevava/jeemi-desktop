//go:build linux && cgo && !bindings

package desktop

/*
#cgo pkg-config: gdk-3.0
#include <gdk/gdk.h>
#include <stdlib.h>
*/
import "C"

import (
	"sync"
	"unsafe"
)

var identityOnce sync.Once

func prepareNativeIdentity() {
	identityOnce.Do(func() {
		id, name := C.CString(AppID), C.CString(Name)
		defer C.free(unsafe.Pointer(id))
		defer C.free(unsafe.Pointer(name))
		// Wails sets ProgramName after constructing its GtkWindow. Set it before
		// GTK opens a display, so both Wayland app_id and X11 WM_CLASS match.
		C.g_set_prgname(id)
		C.gdk_set_program_class(id)
		C.g_set_application_name(name)
	})
}
