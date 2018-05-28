package network

import "hash/crc64"

// In-flight Message, intercepted by the channel.
type MessageI interface {
	GetSeqNum() uint64
	GetCrc64() uint64
	GetPayload() []byte
	GetSize() int

	// Enqueues Message for a subsequent write operation for eventual delivery.
	Accept()
	// Discards Message, preventing its delivery.
	Reject()
}

type Message struct {
	Seqnum  uint64
	Crc64   uint64
	Payload []byte

	DecideFn func(bool)
	Src string
	Dst string
}

func computeCrc64(data []byte) uint64 {
	return crc64.Checksum(data, crc64.MakeTable(crc64.ISO))
}

func (m *Message) GetSeqNum() uint64 {
	return m.Seqnum
}

func (m *Message) GetCrc64() uint64 {
	return m.Crc64
}

func (m *Message) GetPayload() []byte {
	return m.Payload
}

func (m *Message) GetSize() int {
	return len(m.Payload)
}

func (m *Message) Accept() {
	m.DecideFn(true)
}

func (m *Message) Reject() {
	m.DecideFn(false)
}
