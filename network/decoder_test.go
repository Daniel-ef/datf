package network

import (
	"bytes"
	"io"
	"testing"
)

func testDecoderImpl(t *testing.T, in []byte, msgs ...[]byte) {
	d := Decoder{}
	r := bytes.NewReader(in)

	d.Reset()
	for i := 0; i < len(msgs); i++ {
		msg, err := d.ReadFrom(r)
		if err != nil {
			t.Fatalf("unexpected error at Message %v: %v", i, err)
		}
		if msg == nil || bytes.Compare(msg, msgs[i]) != 0 {
			t.Fatalf("unmatched Message %v: actual %v, expected %v", i, msg, msgs[i])
		}
	}

	msg, err := d.ReadFrom(r)
	if err != io.EOF {
		t.Errorf("unexpected error at eof: %v", err)
	}
	if msg != nil {
		t.Errorf("unexpected Message at eof: %v", msg)
	}
}

func TestDecoder1(t *testing.T) {
	testDecoderImpl(t, []byte{0, 0, 0, 1, 'x'}, []byte{'x'})
}

func TestDecoder2(t *testing.T) {
	m := make([]byte, 256+1)
	for i := 0; i < 256+1; i++ {
		m[i] = byte((int('a') + i) % ('z' - 'a' + 1))
	}
	testDecoderImpl(t, append([]byte{0, 0, 1, 1}, m...), m)
}
