//go:build darwin && cgo

package sessionend

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa
void jeemi_session_watch_start(void);
void jeemi_session_watch_stop(void);
void jeemi_session_reply(void);
*/
import "C"

import "sync"

var darwinSession struct {
	sync.Mutex
	request *request
}

func start(r *request, _ func()) (func(), error) {
	darwinSession.Lock()
	darwinSession.request = r
	darwinSession.Unlock()
	C.jeemi_session_watch_start()
	return func() { C.jeemi_session_watch_stop() }, nil
}

//export jeemiSessionEnd
func jeemiSessionEnd() {
	darwinSession.Lock()
	r := darwinSession.request
	darwinSession.Unlock()
	go func() {
		if r != nil {
			r.finish()
		}
		C.jeemi_session_reply()
	}()
}
