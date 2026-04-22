// Package k6ext provides the k6 extension registration for xk6-sip-media.
// It registers the module as "k6/x/sip" so k6 scripts can import it.
package k6ext

import (
	"fmt"
	"sync"

	"go.k6.io/k6/js/modules"
)

func init() {
	modules.Register("k6/x/sip", new(RootModule))
}

// RootModule is the root k6 module. k6 calls NewModuleInstance per VU.
// Metrics are registered once during the first NewModuleInstance call,
// which happens in the init phase when InitEnv() is still valid.
type RootModule struct {
	metricsOnce sync.Once
	metrics     *SIPMetrics
}

// SIPModule is the per-VU module instance exposed to JavaScript.
type SIPModule struct {
	vu      modules.VU
	metrics *SIPMetrics
}

// Ensure interfaces are satisfied at compile time.
var (
	_ modules.Module   = &RootModule{}
	_ modules.Instance = &SIPModule{}
)

// NewModuleInstance returns a new SIPModule for the current VU.
// Metrics registration is deferred to first invocation so the registry
// is available (k6 passes a valid InitEnv during the init phase).
func (r *RootModule) NewModuleInstance(vu modules.VU) modules.Instance {
	r.metricsOnce.Do(func() {
		reg := vu.InitEnv().Registry
		var err error
		r.metrics, err = registerMetrics(reg)
		if err != nil {
			panic(fmt.Sprintf("xk6-sip-media: failed to register metrics: %v", err))
		}
	})
	return &SIPModule{vu: vu, metrics: r.metrics}
}

// Exports returns the JS exports for this module instance.
func (m *SIPModule) Exports() modules.Exports {
	return modules.Exports{
		Default: m,
	}
}
