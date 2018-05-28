package network

import (
	"io"
)

type encoder struct {
	phase     int
	remaining int
	wbuf      []byte
	data      []byte
}

const (
	encoderPhaseLength = iota
	encoderPhasePayload
	encoderPhaseDone
)

func (e *encoder) prepareLength() {
	e.phase = encoderPhaseLength
	e.remaining = 4
	e.wbuf = make([]byte, 4)
	uintToBytes(uint32(len(e.data)), e.wbuf)
}

func (e *encoder) preparePayload() {
	e.phase = encoderPhasePayload
	e.remaining = len(e.data)
	e.wbuf = e.data
}

func (e *encoder) done() {
	e.phase = encoderPhaseDone
	e.remaining = 0
	e.wbuf = nil
	e.data = nil
}

func (e *encoder) writeToOnce(w io.Writer) error {
	n, err := w.Write(e.wbuf[len(e.wbuf)-e.remaining:])
	if n > 0 {
		if e.remaining -= n; e.remaining == 0 {
			switch e.phase {
			case encoderPhaseLength:
				e.preparePayload()
				if len(e.data) > maxPayload && err == nil {
					err = maxPayloadError
				}
			case encoderPhasePayload:
				e.done()
			}
		}
	}
	return err
}

func (e *encoder) writeTo(w io.Writer) (bool, error) {
	for e.phase != encoderPhaseDone {
		if err := e.writeToOnce(w); err != nil {
			return e.phase == encoderPhaseDone, err
		}
	}
	return true, nil
}

func (e *encoder) Reset() {
	e.done()
}

func (e *encoder) Next(msg []byte) {
	e.data = msg
	e.prepareLength()
}

func (e *encoder) WriteTo(w io.Writer) (bool, error) {
	return e.writeTo(w)
}
