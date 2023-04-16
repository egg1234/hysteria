package main

import (
	"context"
	"crypto/tls"
	"flag"
	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
	"log"
	"net/http"
	"sync/atomic"
)

var (
	certFile   = flag.String("cert", "localhost.crt", "TLS certificate file")
	keyFile    = flag.String("key", "localhost.key", "TLS key file")
	listenAddr = flag.String("listen", ":4433", "Listen address")
)

func main() {
	flag.Parse()

	server()
}

func server() {
	certs, err := tls.LoadX509KeyPair(*certFile, *keyFile)
	if err != nil {
		log.Fatalln("load cert error:", err)
	}
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{certs},
		NextProtos:   []string{http3.NextProtoH3},
	}
	listener, err := quic.ListenAddr(*listenAddr, tlsConfig, nil)
	if err != nil {
		log.Fatalln("listen error:", err)
	}
	for {
		conn, err := listener.Accept(context.Background())
		if err != nil {
			log.Fatalln("accept error:", err)
		}
		go func() {
			log.Println("connected:", conn.RemoteAddr().String())
			if err := handleConn(conn); err != nil {
				log.Println("disconnected:", conn.RemoteAddr().String(), err)
			}
		}()
	}
}

func handleConn(conn quic.Connection) error {
	h3s := http3.Server{
		Handler: &clientHandler{},
	}
	return h3s.ServeQUICConn(conn)
}

type clientHandler struct {
	knocked atomic.Bool
}

func (h *clientHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet && r.URL.Path == "/hysteria/knock" {
		h.knocked.Store(true)
		_, _ = w.Write([]byte("knock knock"))
		return
	}
	if !h.knocked.Load() {
		http.Error(w, "knock first", http.StatusForbidden)
		return
	}
	_, _ = w.Write([]byte("hello world"))
}
