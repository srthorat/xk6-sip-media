//go:build g729
// +build g729

package codec

/*
#cgo CFLAGS: -I${SRCDIR}/bcg729/include
#cgo LDFLAGS: -L${SRCDIR}/bcg729/lib -lbcg729
#include <bcg729/decoder.h>
#include <bcg729/encoder.h>
#include <stdlib.h>
*/
import "C"
import (
	"sync"
	"unsafe"
)

// G729 is the ITU-T G.729 Audio Codec.
// Operates at 8000Hz. Converts 80 PCM16 samples (160 bytes) into 10 bytes of G.729 payload.
type G729 struct {
	mu      sync.Mutex
	encoder *C.bcg729EncoderChannelContextStruct
	decoder *C.bcg729DecoderChannelContextStruct
}

func newG729() (Codec, error) {
	return &G729{
		encoder: C.initBcg729EncoderChannel(C.uint8_t(0)), // 0 = no VAD
		decoder: C.initBcg729DecoderChannel(),
	}, nil
}

func (c *G729) Name() string {
	return "G729"
}

func (c *G729) SampleRate() int {
	return 8000
}

func (c *G729) PayloadType() uint8 {
	return 18
}

func (c *G729) Encode(frame []int16) []byte {
	if len(frame) == 0 {
		return nil
	}
	if len(frame)%80 != 0 {
		return nil
	}

	frames := len(frame) / 80
	out := make([]byte, frames*10)

	c.mu.Lock()
	defer c.mu.Unlock()

	for i := 0; i < frames; i++ {
		pcmBlock := frame[i*80 : (i+1)*80]
		g729Block := out[i*10 : (i+1)*10]

		var bslen C.uint8_t
		C.bcg729Encoder(
			c.encoder,
			(*C.int16_t)(unsafe.Pointer(&pcmBlock[0])),
			(*C.uint8_t)(unsafe.Pointer(&g729Block[0])),
			&bslen,
		)
	}
	return out
}

func (c *G729) Decode(payload []byte) []int16 {
	if len(payload) == 0 {
		return nil
	}
	if len(payload)%10 != 0 {
		return nil
	}

	frames := len(payload) / 10
	out := make([]int16, frames*80)

	c.mu.Lock()
	defer c.mu.Unlock()

	for i := 0; i < frames; i++ {
		g729Block := payload[i*10 : (i+1)*10]
		pcmBlock := out[i*80 : (i+1)*80]

		C.bcg729Decoder(
			c.decoder,
			(*C.uint8_t)(unsafe.Pointer(&g729Block[0])),
			C.uint8_t(10), // bitStreamLength
			0,             // frameErasureFlag
			0,             // SIDFrameFlag
			0,             // rfc3389PayloadFlag
			(*C.int16_t)(unsafe.Pointer(&pcmBlock[0])),
		)
	}
	return out
}

func (c *G729) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.encoder != nil {
		C.closeBcg729EncoderChannel(c.encoder)
		c.encoder = nil
	}
	if c.decoder != nil {
		C.closeBcg729DecoderChannel(c.decoder)
		c.decoder = nil
	}
	return nil
}
