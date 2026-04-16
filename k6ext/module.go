// Package k6ext provides the k6 extension registration for xk6-sip-media.
// It registers the module as "k6/x/sip" so k6 scripts can import it.
package k6ext

import (
	"go.k6.io/k6/js/modules"
)

func init() {
	modules.Register("k6/x/sip", new(RootModule))
}

// RootModule is the root k6 module. k6 calls NewModuleInstance per VU.
type RootModule struct{}

// SIPModule is the per-VU module instance exposed to JavaScript.
type SIPModule struct {
	vu modules.VU
}

// Ensure interfaces are satisfied at compile time.
var (
	_ modules.Module   = &RootModule{}
	_ modules.Instance = &SIPModule{}
)

// NewModuleInstance returns a new SIPModule for the current VU.
func (r *RootModule) NewModuleInstance(vu modules.VU) modules.Instance {
	return &SIPModule{vu: vu}
}

// Exports returns the JS exports for this module instance.
func (m *SIPModule) Exports() modules.Exports {
	return modules.Exports{
		Default: m,
	}
}
