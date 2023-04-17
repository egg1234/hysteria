package main

import (
	"context"
	"crypto/tls"
	"flag"
	"io"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"sync/atomic"
	"time"

	"github.com/quic-go/quic-go/quicvarint"

	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
)

const (
	frameTypeProxyRequest  = 0x401
	frameTypeProxyResponse = 0x402

	httpStatusKnocked = 233
)

func server() {
	flags := flag.NewFlagSet("server", flag.ExitOnError)

	listenAddr := flags.String("listen", ":4433", "Listen address")
	certFile := flags.String("cert", "cert.crt", "TLS certificate file")
	keyFile := flags.String("key", "cert.key", "TLS key file")
	password := flags.String("password", "pass@word", "Password for the server")

	_ = flags.Parse(os.Args[2:])

	log.Printf("Server listening on %s, password: %s\n", *listenAddr, *password)

	cert, err := tls.LoadX509KeyPair(*certFile, *keyFile)
	if err != nil {
		log.Fatalln("load cert error:", err)
	}
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
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
			if err := serverHandleConn(conn, *password); err != nil {
				log.Println("disconnected:", conn.RemoteAddr().String(), err)
			}
		}()
	}
}

func serverHandleConn(conn quic.Connection, password string) error {
	handler := &h3sHandler{
		Password: password,
		Dialer: &net.Dialer{
			Timeout: 10 * time.Second,
		},
	}
	h3s := http3.Server{
		Handler:        handler,
		StreamHijacker: handler.ProxyStreamHijacker,
	}
	return h3s.ServeQUICConn(conn)
}

type h3sHandler struct {
	Password string
	Dialer   *net.Dialer

	knocked atomic.Bool
}

func (h *h3sHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost && r.URL.Path == "/knock" {
		if h.knocked.Load() {
			w.WriteHeader(httpStatusKnocked)
			return
		}
		// Check password in header
		pw := r.Header.Get("Authorization")
		if pw != h.Password {
			_ = h.writeRandomQuote(w)
			log.Printf("failed knock attempt from %s\n", r.RemoteAddr)
			return
		}
		// Password is correct
		h.knocked.Store(true)
		log.Printf("successfully knocked from %s\n", r.RemoteAddr)
		w.WriteHeader(httpStatusKnocked)
		return
	}
	_ = h.writeRandomQuote(w)
}

func (h *h3sHandler) ProxyStreamHijacker(ft http3.FrameType, conn quic.Connection, stream quic.Stream, err error) (bool, error) {
	if !h.knocked.Load() || ft != frameTypeProxyRequest {
		return false, nil
	}
	log.Printf("proxy request from %s, stream id: %d\n", conn.RemoteAddr().String(), stream.StreamID())
	go func() {
		h.handleProxyStream(stream)
		_ = stream.Close()
		log.Printf("proxy request from %s, stream id: %d closed\n", conn.RemoteAddr().String(), stream.StreamID())
	}()
	return true, nil
}

func (h *h3sHandler) handleProxyStream(stream quic.Stream) {
	// Proxy request format:
	// 1. address length (varint)
	// 2. address (string)

	qr := quicvarint.NewReader(stream)
	l, err := quicvarint.Read(qr)
	if err != nil {
		log.Println("read address length error:", err)
		return
	}
	addr := make([]byte, l)
	_, err = io.ReadFull(stream, addr)
	if err != nil {
		log.Println("read address error:", err)
		return
	}

	// Connect to the target address
	tconn, err := h.Dialer.Dial("tcp", string(addr))
	// Proxy response format:
	// 1. error code (byte) 0=success, 1=error
	if err != nil {
		log.Println("dial error:", err)
		_, _ = stream.Write([]byte{1})
		return
	}
	defer tconn.Close()
	_, _ = stream.Write([]byte{0})

	// Proxy data
	go func() {
		_, _ = io.Copy(tconn, stream)
		_ = tconn.Close()
	}()
	_, _ = io.Copy(stream, tconn)
}

func (h *h3sHandler) writeRandomQuote(w http.ResponseWriter) error {
	quotes := []string{
		"The unexamined life is not worth living. - Socrates",
		"The only way to deal with an unfree world is to become so absolutely free that your very existence is an act of rebellion. - Albert Camus",
		"Man is condemned to be free; because once thrown into the world, he is responsible for everything he does. - Jean-Paul Sartre",
		"Freedom is what you do with what's been done to you. - Jean-Paul Sartre",
		"You cannot step into the same river twice, for fresh waters are ever flowing in upon you. - Heraclitus",
		"In the midst of winter, I found there was, within me, an invincible summer. - Albert Camus",
	}
	_, err := w.Write([]byte(quotes[rand.Intn(len(quotes))]))
	return err
}
