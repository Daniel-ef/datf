package websocket

import (
	"fmt"
	"github.com/gorilla/websocket"
	"hse-dss-efimov/ctx"
	"go.uber.org/zap"
	"math/rand"
	"net/http"
)

const (
	readBufferSize  = 1024
	writeBufferSize = 1024
	queueCapacity   = 128
)

func HttpHandler(dispatcher *Dispatcher, w http.ResponseWriter, r *http.Request, msg_db_chan *Chans_ports) {
	conn, err := websocket.Upgrade(w, r, nil, readBufferSize, writeBufferSize)
	if _, ok := err.(websocket.HandshakeError); ok {
		http.Error(w, "handshake error", 400)
		return
	} else if err != nil {
		http.Error(w, "client error", 400)
		return
	}

	sessionId, ok := r.Context().Value(ctx.RequestIdKey).(string)
	if !ok {
		sessionId = fmt.Sprintf("%x", rand.Int63())
	}

	sessionLogger := dispatcher.logger.With(zap.String("session_id", sessionId))
	sessionLogger.Debug("opened websocket session", zap.String("remote_addr", r.RemoteAddr))

	session := &session{
		id:     sessionId,
		logger: *sessionLogger,
		conn:   conn,
		queue:  make(chan Message, queueCapacity),
		db:     msg_db_chan.MsgsDb,
	}

	dispatcher.registerCh <- session
	session.runLoop(r.Context(), dispatcher.callHandler, msg_db_chan)
	dispatcher.unregisterCh <- session
}
