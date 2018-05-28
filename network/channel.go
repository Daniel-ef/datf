package network

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"io"
	"net"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

type Channel interface {
	Named
	// Returns source (inbound) port.
	GetSrcPort() int
	// Returns destination (outbound) port.
	GetDstPort() int
	// Closes the channel, aborting all in-flight messages.
	Close()
}

// Represents unidirectional flow within a full-duplex channel.
type semichannel struct {
	mu        sync.RWMutex // protects read queue
	dec       Decoder
	enc       encoder
	readQueue []MessageI
	writeMsg  MessageI
	writeCh   chan MessageI
}

// Represents bidirectional channel.
type channel struct {
	name    string
	srcPort int
	dstPort int
	counter *uint64
	logger  zap.Logger

	closeCh chan struct{}
	closeWg sync.WaitGroup

	inbound  semichannel
	outbound semichannel
}

const (
	backoffTimeout = 10 * time.Second
	readTimeout    = 1 * time.Second
	writeTimeout   = 1 * time.Second
)

func isTimeout(err error) bool {
	if opErr, ok := err.(*net.OpError); ok && opErr.Timeout() {
		return true
	}
	return false
}

func isEOF(err error) bool {
	if err == io.EOF {
		return true
	}
	return false
}

func fieldsFor(msg MessageI) []zapcore.Field {
	return []zapcore.Field{
		zap.Uint64("Seqnum", msg.GetSeqNum()),
		zap.Uint64("Crc64", msg.GetCrc64()),
		zap.Int("size", msg.GetSize())}
}

func (sc *semichannel) addMessage(msg MessageI) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	sc.readQueue = append(sc.readQueue, msg)
}

func (sc *semichannel) getMessageIndexBySeqNum(seqnum uint64) int {
	i := sort.Search(
		len(sc.readQueue),
		func(i int) bool { return sc.readQueue[i].GetSeqNum() >= seqnum })
	if i < len(sc.readQueue) && sc.readQueue[i].GetSeqNum() == seqnum {
		return i
	} else {
		return -1
	}
}

func (sc *semichannel) decideOnMessage(msg MessageI, outcome bool, logger zap.Logger) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	if i := sc.getMessageIndexBySeqNum(msg.GetSeqNum()); i >= 0 {
		sc.readQueue = append(sc.readQueue[:i], sc.readQueue[i+1:]...)
		if outcome {
			logger.Debug("Message accepted", fieldsFor(msg)...)
			sc.writeCh <- msg
		} else {
			logger.Debug("Message rejected", fieldsFor(msg)...)
		}
	} else {
		logger.Debug("ignoring duplicate request for Message processing", fieldsFor(msg)...)
	}
}

func runConnectionRead(
	doneCh chan struct{},
	closeCh <-chan struct{},
	counter *uint64,
	sc *semichannel,
	conn *net.TCPConn,
	connLogger zap.Logger,
	msgChan chan Message,
	src_port int,
	dst_port int,
	) {

	defer func() {
		close(doneCh)
		conn.CloseRead()
	}()

	sc.dec.Reset()
	for {
		select {
		case <-closeCh:
			return
		default:
		}
		conn.SetReadDeadline(time.Now().Add(readTimeout))
		buf, err := sc.dec.ReadFrom(conn)
		if buf != nil {
			num := atomic.AddUint64(counter, 1)
			crc := computeCrc64(buf)
			msg := &Message{Seqnum: num, Crc64: crc, Payload: buf,
				Src:strconv.Itoa(src_port), Dst:strconv.Itoa(dst_port)}
			msg.DecideFn = func(outcome bool) { sc.decideOnMessage(msg, outcome, connLogger) }
			connLogger.Debug("received Message", fieldsFor(msg)...)
			sc.addMessage(msg)
			msgChan <- *msg
		}
		if err != nil {
			if isTimeout(err) {
				continue
			}
			if isEOF(err) {
				return
			}
			connLogger.Error("read() failed", zap.Error(err))
			return
		}
	}
}

func runConnectionWrite(
	doneCh chan struct{},
	closeCh <-chan struct{},
	sc *semichannel,
	conn *net.TCPConn,
	connLogger zap.Logger,
	) {

	defer func() {
		close(doneCh)
		conn.CloseWrite()
	}()
	sc.enc.Reset()
	for {
		select {
		case <-closeCh:
			return
		default:
		}
		if sc.writeMsg == nil {
			select {
			case <-closeCh:
				return
			case sc.writeMsg = <-sc.writeCh:
				connLogger.Debug("sending Message", fieldsFor(sc.writeMsg)...)
				sc.enc.Next(sc.writeMsg.GetPayload())
			}
		}
		conn.SetDeadline(time.Now().Add(writeTimeout))
		done, err := sc.enc.WriteTo(conn)
		if done {
			connLogger.Debug("Message sent ", fieldsFor(sc.writeMsg)...)
			sc.writeMsg = nil
		}
		if err != nil {
			if isTimeout(err) {
				continue
			}
			connLogger.Error("write() failed", zap.Error(err))
			return
		}
	}
}

func NewChannel(name string, srcPort int, dstPort int, counter *uint64,
				logger zap.Logger, msgChan chan Message) Channel {
	chBuferCapacity := 100
	c := &channel{
		name:    name,
		srcPort: srcPort,
		dstPort: dstPort,
		counter: counter,
		logger:  *logger.With(zap.String("channel", name)),
		closeCh: make(chan struct{}),
		inbound: semichannel{
			readQueue: make([]MessageI, 0),
			writeCh:   make(chan MessageI, chBuferCapacity),
		},
		outbound: semichannel{
			readQueue: make([]MessageI, 0),
			writeCh:   make(chan MessageI, chBuferCapacity),
		},
	}
	c.closeWg.Add(2)
	go c.runInboundFlow(msgChan)
	go c.runOutboundFlow(msgChan)
	return c
}

func (c *channel) runInboundFlow(msgChan chan Message) {
	defer c.closeWg.Done()

	addr := ":" + strconv.Itoa(c.srcPort)

	for {
		select {
		case <-c.closeCh:
			c.logger.Debug("inbound flow terminated")
			return
		default:
		}

		listener, err := net.Listen("tcp", addr)
		if err != nil {
			c.logger.Error("listen() failed", zap.String("addr", addr), zap.Error(err))
			select {
			case <-c.closeCh:
			case <-time.After(backoffTimeout):
			}
			continue
		}

		listenerLogger := c.logger.With(
			zap.String("addr", listener.Addr().String()),
			zap.String("network", listener.Addr().Network()))
		listenerLogger.Debug("listening for incoming connection")

		c.closeWg.Add(1)
		// Blocking call here; every channel must have exactly once inbound connection.
		c.runInboundListener(listener.(*net.TCPListener), *listenerLogger, msgChan)
	}
}

func (c *channel) runInboundListener(listener *net.TCPListener, listenerLogger zap.Logger, msgChan chan Message) {
	defer c.closeWg.Done()

	type acceptResult struct {
		conn *net.TCPConn
		err  error
	}
	acceptCh := make(chan acceptResult)

	for {
		go func() {
			listener.SetDeadline(time.Time{})
			conn, err := listener.AcceptTCP()
			select {
			case <-c.closeCh:
			case acceptCh <- acceptResult{conn, err}:
			}
		}()

		select {
		case <-c.closeCh:
			listener.Close()
			listenerLogger.Debug("no longer listening for incoming connection")
			return
		case result := <-acceptCh:
			conn, err := result.conn, result.err
			if err != nil {
				if isTimeout(err) {
					continue
				}
				listenerLogger.Error("accept() failed", zap.Error(err))
				return
			}

			connLogger := c.logger.With(
				zap.String("localaddr", conn.LocalAddr().String()),
				zap.String("remoteaddr", conn.RemoteAddr().String()),
				zap.String("direction", "inbound"))
			connLogger.Debug("accepted inbound connection")

			c.closeWg.Add(1)
			// Blocking call here; every channel must have exactly once inbound connection.
			c.runConnection(&c.inbound, conn, *connLogger, msgChan, true)
		}
	}
}

func (c *channel) runOutboundFlow(msgChan chan Message) {
	defer c.closeWg.Done()

	addr := ":" + strconv.Itoa(c.dstPort)

	for {
		select {
		case <-c.closeCh:
			c.logger.Debug("outbound flow terminated")
			return
		default:
		}

		conn, err := net.DialTimeout("tcp", addr, backoffTimeout)
		if err != nil {
			c.logger.Error("connect() failed", zap.String("addr", addr), zap.Error(err))
			select {
			case <-c.closeCh:
			case <-time.After(backoffTimeout):
			}
			continue
		}

		connLogger := c.logger.With(
			zap.String("localaddr", conn.LocalAddr().String()),
			zap.String("remoteaddr", conn.RemoteAddr().String()),
			zap.String("direction", "outbound"))
		connLogger.Debug("established outbound connection")

		c.closeWg.Add(1)
		// Blocking call here; every channel must have exactly once outbound connection.
		c.runConnection(&c.inbound, conn.(*net.TCPConn), *connLogger, msgChan, false)
	}
}

func (c *channel) runConnection(sc *semichannel, conn *net.TCPConn, connLogger zap.Logger, msgChan chan Message, inbound bool) {
	defer func() {
		connLogger.Debug("closing connection")
		conn.Close()
		c.closeWg.Done()
	}()

	readCh := make(chan struct{})
	writeCh := make(chan struct{})

	if inbound {
		go runConnectionRead(readCh, c.closeCh, c.counter, sc, conn, connLogger, msgChan, c.srcPort, c.dstPort)
	} else {
		go runConnectionWrite(writeCh, c.closeCh, sc, conn, connLogger)
	}

	for n := 2; n > 0; {
		select {
		case _, ok := <-readCh:
			if !ok {
				readCh = nil
				n--
			}
			connLogger.Debug("Read ends", zap.Any("ok", ok))
			return
		case _, ok := <-writeCh:
			if !ok {
				writeCh = nil
				n--
			}
			connLogger.Debug("Write ends", zap.Any("ok", ok))
		}
	}
}

func (c *channel) GetName() string {
	return c.name
}

func (c *channel) GetSrcPort() int {
	return c.srcPort
}

func (c *channel) GetDstPort() int {
	return c.dstPort
}

func (c *channel) Close() {
	c.logger.Debug("closing channel")
	close(c.closeCh)
	c.closeWg.Wait()
	c.logger.Debug("channel closed")
}
