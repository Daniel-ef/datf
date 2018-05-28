package websocket

import (
	"go.uber.org/zap"
	"sync"
)

type Dispatcher struct {
	logger  zap.Logger
	closeCh chan struct{}
	closeWg sync.WaitGroup

	callHandler CallHandler

	sessions     map[*session]bool
	registerCh   chan *session
	unregisterCh chan *session
}

func NewDispatcher(logger zap.Logger, callHandler CallHandler) *Dispatcher {
	d := &Dispatcher{
		logger:       logger,
		callHandler:  callHandler,
		closeCh:      make(chan struct{}),
		sessions:     make(map[*session]bool),
		registerCh:   make(chan *session),
		unregisterCh: make(chan *session),
	}
	d.closeWg.Add(1)
	go d.run()
	return d
}

func (d *Dispatcher) run() {
	defer d.closeWg.Done()
	for {
		select {
		case <-d.closeCh:
			for s := range d.sessions {
				d.unregisterSession(s)
			}
			return
		case s := <-d.registerCh:
			d.registerSession(s)
		case s := <-d.unregisterCh:
			if _, ok := d.sessions[s]; ok {
				d.unregisterSession(s)
			}
		}
	}
}



func (d *Dispatcher) registerSession(s *session) {
	if _, ok := d.sessions[s]; !ok {
		d.sessions[s] = true
		d.logger.Debug("session registered", zap.String("session_id", s.id))

		s.queue <- Message{Kind: MK_Hello}

	} else {
		d.logger.Warn("duplicate session registration", zap.String("session_id", s.id))
	}
}

func (d *Dispatcher) unregisterSession(s *session) {
	if _, ok := d.sessions[s]; ok {
		delete(d.sessions, s)
		close(s.queue)
		d.logger.Debug("session unregistered", zap.String("session_id", s.id))
	} else {
		d.logger.Warn("unknown session unregistration", zap.String("session_id", s.id))
	}
}

func (d *Dispatcher) Close() {
	d.logger.Debug("closing dispatcher")
	close(d.closeCh)
	d.closeWg.Wait()
	d.logger.Debug("dispatcher closed")
}
