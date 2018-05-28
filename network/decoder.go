package network

import (
	"io"
)

type Decoder struct {
	phase     int
	remaining int
	rbuf      []byte
}

const (
	decoderPhaseLength = iota
	decoderPhasePayload
)

func (d *Decoder) expectLength() {
	d.phase = decoderPhaseLength
	d.remaining = 4
	d.rbuf = make([]byte, 4)
}

func (d *Decoder) expectPayload() {
	d.phase = decoderPhasePayload
	d.remaining = int(bytesToUint(d.rbuf))
	d.rbuf = make([]byte, d.remaining)
}

func (d *Decoder) readFromImpl(r io.Reader) ([]byte, error) {
	var msg []byte
	n, err := r.Read(d.rbuf[len(d.rbuf)-d.remaining:])
	if n > 0 {
		if d.remaining -= n; d.remaining == 0 {
			switch d.phase {
			case decoderPhaseLength:
				d.expectPayload()
				if d.remaining > maxPayload && err == nil {
					err = maxPayloadError
				}
			case decoderPhasePayload:
				msg = d.rbuf
				d.expectLength()
			}
		}
	}
	return msg, err
}

func (d *Decoder) readFrom(r io.Reader) ([]byte, error) {
	for {
		msg, err := d.readFromImpl(r)
		if msg != nil || err != nil {
			return msg, err
		}
	}
}

func (d *Decoder) Reset() {
	d.expectLength()
}

func (d *Decoder) ReadFrom(r io.Reader) ([]byte, error) {
	return d.readFrom(r)
}
