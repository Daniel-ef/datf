package websocket

import (
	"context"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
	"time"
	"hse-dss-efimov/network"
	"strconv"
)

const (
	maxMessageSize = 512
	pingPeriod     = 5 * time.Second
	readTimeout    = 15 * time.Second // must be greater than ping period
	writeTimeout   = 5 * time.Second
)

type session struct {
	id     string
	logger zap.Logger

	conn  *websocket.Conn
	queue chan Message
	db MsgDb
}

type MsgDb map[uint64]network.Message

/*
 * Send message to WebSocket
 */
func sendToWs(msg network.Message, update bool, s *session, messagesDb *MsgDb) {
	msgNum := msg.Seqnum
	wsMsg := wsMessage{Src: msg.Src, Dst: msg.Dst,
		MsgNumber: strconv.FormatUint(msgNum, 10), Payload: string(msg.Payload)}
	s.logger.Debug("sending json message to WS", zap.Any("msg", wsMsg))
	if update {
		(*messagesDb)[msgNum] = msg
	}

	if err := s.conn.WriteJSON(wsMsg); err != nil {
		s.logger.Error("failed to send json message", zap.Error(err))
		return
	}
}

func (s *session) runLoop(ctx context.Context, callHandler CallHandler, msgDbChan *Chans_ports) {
	defer func() {
		s.conn.Close()
		s.logger.Debug("websocket was closed")
	}()

	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()

	s.conn.SetReadLimit(maxMessageSize)
	s.conn.SetReadDeadline(time.Now().Add(readTimeout))
	s.conn.SetPongHandler(func(string) error {
		// s.logger.Debug("received pong message")
		s.conn.SetReadDeadline(time.Now().Add(readTimeout))
		return nil
	})

	type readResult struct {
		msg Message
		err error
	}
	readCh := make(chan readResult)

	msgsDb := s.db

	for {
		go func() {
			var msg = &Message{}
			err := s.conn.ReadJSON(msg)
			select {
			case <-ctx.Done():
			case readCh <- readResult{*msg, err}:
			}
		}()

		select {
		case <-ctx.Done():
			return

		case readResult := <-readCh:
			if msg, err := readResult.msg, readResult.err; err != nil {
				if closeErr, ok := err.(*websocket.CloseError); ok {
					s.logger.Debug("received close message", zap.Error(closeErr))
				} else {
					s.logger.Error("failed to receive json message", zap.Error(err))
				}
				return
			} else {
				s.logger.Debug("received json message from websocket", zap.Any("msg", msg))

				switch msg.Kind {
				case MK_Request:
					req := msg.Request
					switch req {
					case "db":
						for key := range msgsDb {
							sendToWs(msgsDb[key], false, s, &msgsDb)
						}
					default:
					}
				case MK_Response:
					msgNumber, err := strconv.ParseUint(msg.MsgNumber, 10, 64)
					if err != nil {
						s.logger.Debug("Fail to cast message number", zap.Any("err", err))
						continue
					}
					msgNet, ok := msgsDb[msgNumber]
					if !ok {
						s.logger.Debug("There is no message with such message number or decision happened")
						continue
					}
					decision := msg.Data
					msgNet.DecideFn(decision == "1")
					delete(msgsDb, msgNumber)
				default:
					s.logger.Debug("ignoring message", zap.Any("msg", msg))
				}
			}

		case msg, ok := <-msgDbChan.MsgChan:
			s.conn.SetWriteDeadline(time.Now().Add(writeTimeout))
			if !ok {
				s.logger.Debug("sending close message")
				s.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			} else {
				sendToWs(msg,true, s, &msgsDb)
			}

		case <-ticker.C:
			s.conn.SetWriteDeadline(time.Now().Add(writeTimeout))
			// s.logger.Debug("sending ping message")
			if err := s.conn.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
				s.logger.Error("failed to send ping message", zap.Error(err))
				return
			}
		}
	}
}
