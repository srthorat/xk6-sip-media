//go:build !g729
// +build !g729

package codec

import "fmt"

func newG729() (Codec, error) {
	return nil, fmt.Errorf("G729 is disabled to comply with MIT/BSD open-core (requires GPLv3 bcg729): rebuild with 'go build -tags g729'")
}
