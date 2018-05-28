package cmd

import (
	"context"
	"fmt"
	"hse-dss-efimov/ctx"
	"hse-dss-efimov/network"
	"hse-dss-efimov/websocket"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
)

type ports struct {
	src int
	dst int
}

var channelCmd = &cobra.Command{
	Use:   "channel SRCPORT DSTPORT [WEBPORT]",
	Short: "Establishes a channel between source port and destination port",
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) < 2 {
			fmt.Println("Command requires at least SRCPORT and DSTPORT arguments")
			os.Exit(-1)
		}

		var ports_pairs []ports

		for i := 0; i < len(args) - 1; i += 2 {
			srcport, err := strconv.Atoi(args[i])
			if err != nil {
				fmt.Printf("Cannot parse SRCPORT: %v", err)
				os.Exit(-1)
			}

			dstport, err := strconv.Atoi(args[i + 1])
			if err != nil {
				fmt.Printf("Cannot parse DSTPORT: %v", err)
				os.Exit(-1)
			}
			ports_pairs = append(ports_pairs, ports{srcport, dstport})
		}

		webport, err := -1, error(nil)
		if len(args) % 2 == 1 {
			webport, err = strconv.Atoi(args[len(args) - 1])
			if err != nil {
				fmt.Printf("Cannot parse WEBPORT: %v", err)
				os.Exit(-1)
			}
		}

		mainChannel(ports_pairs, webport)
	},
}

func init() {
	RootCmd.AddCommand(channelCmd)
}

func httpRootHandler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte{'H', 'i', '!'})
}

type httpWrapperHandler struct {
	logger  zap.Logger
	handler http.Handler
}

func (h httpWrapperHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	requestId := fmt.Sprintf("%x", rand.Int63())
	defer func() {
		if err := recover(); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			h.logger.Error("panic while handling http request",
				zap.String("request_id", requestId),
				zap.Any("err", err))
		}
	}()
	h.logger.Debug("got http request",
		zap.String("request_id", requestId),
		zap.String("request_uri", r.RequestURI),
		zap.String("remote_addr", r.RemoteAddr))
	h.handler.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctx.RequestIdKey, requestId)))
}

func mainChannel(port_pairs []ports, webport int) {
	logger, _ := zap.NewDevelopment()

	dispatcher := websocket.NewDispatcher(*logger, func(ctx websocket.CallCtx) {
		logger.Debug("handling call", zap.Any("data", ctx.Data()))
	})

	msg_db_chan := &websocket.Chans_ports{MsgsDb:make(websocket.MsgDb), MsgChan:make(chan network.Message, 100)}

	counter := uint64(0)
	for _, pair := range port_pairs {
		channel := network.NewChannel("c", pair.src, pair.dst, &counter, *logger, msg_db_chan.MsgChan)
		defer channel.Close()
	}


	if webport > 0 {
		m := http.NewServeMux()
		// m.HandleFunc("/", httpRootHandler)
		m.Handle("/web/", http.StripPrefix("/web/", http.FileServer(http.Dir("./web"))))
		m.Handle("/images/", http.StripPrefix("/images/", http.FileServer(http.Dir("./images"))))
		m.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
			websocket.HttpHandler(dispatcher, w, r, msg_db_chan)
		})
		go func() {
			logger.Debug("http server is serving on port " + strconv.Itoa(webport))
			a := ":" + strconv.Itoa(webport)
			h := httpWrapperHandler{*logger, m}
			if err := http.ListenAndServe(a, h); err != nil {
				logger.Panic("failed to listen and serve http", zap.Error(err))
			}
		}()
	} else {
		logger.Debug("http server is not serving")
	}

	q := make(chan os.Signal)
	signal.Notify(q, syscall.SIGINT, syscall.SIGTERM)
	log.Println(<-q)

	dispatcher.Close()
}
