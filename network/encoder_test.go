package network

import (
	"bytes"
	"testing"
)

func testEncoderImpl(t *testing.T, payload []byte, expected []byte) {
	e := encoder{}
	w := &bytes.Buffer{}

	e.Reset()
	e.Next(payload)

	done, err := e.WriteTo(w)
	actual := w.Bytes()
	if !done {
		t.Fatalf("incomplete write: %v", actual)
	}
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bytes.Compare(actual, expected) != 0 {
		t.Fatalf("unmatched Message: actual %v, expected %v", actual, expected)
	}
}

func TestEncoder1(t *testing.T) {
	testEncoderImpl(t, []byte{'x'}, []byte{0, 0, 0, 1, 'x'})
}

func TestEncoder2(t *testing.T) {
	testEncoderImpl(t, []byte{'x', 'y', 'z'}, []byte{0, 0, 0, 3, 'x', 'y', 'z'})
}
