package k6ext

import (
	sipcall "github.com/srthorat/xk6-sip-media/sip"
)

// K6RegisteredUAS wraps a *sip.RegisteredUAS for the k6 JavaScript runtime.
//
// Returned by sip.registerAndListen(). The VU keeps the handle alive for the
// duration of the scenario, then calls stop() to unregister and shut down.
type K6RegisteredUAS struct {
	uas *sipcall.RegisteredUAS
}

// Stop unregisters from the SIP proxy and shuts down the UA.
//
//	uas.stop();
func (u *K6RegisteredUAS) Stop() {
	u.uas.Stop()
}

// Unregister sends REGISTER Expires:0 without stopping the listener.
// Useful if you want to re-register later with a different AOR.
//
//	uas.unregister();
func (u *K6RegisteredUAS) Unregister() error {
	return u.uas.Unregister()
}
