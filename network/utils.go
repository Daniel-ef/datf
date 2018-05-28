package network

import "errors"

const maxPayload = 32 * 1024

var maxPayloadError = errors.New("payload exceeds limit")

func uintToBytes(v uint32, out []byte) {
	out[0] = byte((v >> 24) & 0xff)
	out[1] = byte((v >> 16) & 0xff)
	out[2] = byte((v >> 8) & 0xff)
	out[3] = byte((v >> 0) & 0xff)
}

func bytesToUint(in []byte) uint32 {
	v := uint32(0)
	v += uint32(in[3]) << 0
	v += uint32(in[2]) << 8
	v += uint32(in[1]) << 16
	v += uint32(in[0]) << 24
	return v
}
