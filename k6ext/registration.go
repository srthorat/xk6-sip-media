package k6ext

import (
	sipcall "github.com/srthorat/xk6-sip-media/sip"
)

// K6Registration wraps a *sip.Registration for the k6 JavaScript runtime.
type K6Registration struct {
	reg *sipcall.Registration
}

// Refresh re-sends REGISTER to renew the registration.
func (r *K6Registration) Refresh() error {
	return r.reg.Refresh()
}

// Unregister sends REGISTER with Expires: 0 to remove the registration.
func (r *K6Registration) Unregister() error {
	return r.reg.Unregister()
}
