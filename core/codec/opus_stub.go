//go:build !cgo
// +build !cgo

package codec

import "fmt"

func NewOpus() (Codec, error) {
	return nil, fmt.Errorf("OPUS requires CGO: rebuild with CGO_ENABLED=1 and libopus installed")
}
