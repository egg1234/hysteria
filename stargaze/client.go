package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
	"github.com/quic-go/quic-go/quicvarint"
)

func client() {
	flags := flag.NewFlagSet("client", flag.ExitOnError)

	serverAddr := flags.String("server", "localhost:4433", "Server address")
	password := flags.String("password", "pass@word", "Password for the server")
	insecure := flags.Bool("insecure", false, "Skip TLS verification")

	_ = flags.Parse(os.Args[2:])

	log.Printf("Client connecting to %s, password: %s\n", *serverAddr, *password)

	tlsConfig := &tls.Config{
		NextProtos:         []string{http3.NextProtoH3},
		InsecureSkipVerify: *insecure,
	}

	var conn quic.EarlyConnection
	rt := http3.RoundTripper{
		TLSClientConfig: tlsConfig,
		Dial: func(ctx context.Context, addr string, tlsCfg *tls.Config, cfg *quic.Config) (quic.EarlyConnection, error) {
			c, err := quic.DialAddrEarlyContext(ctx, addr, tlsCfg, cfg)
			if err == nil {
				conn = c
			}
			return c, err
		},
	}

	resp, err := rt.RoundTrip(&http.Request{
		Method: http.MethodPost,
		URL: &url.URL{
			Scheme: "https",
			Host:   *serverAddr,
			Path:   "/knock",
		},
		Header: http.Header{"Authorization": []string{*password}},
	})
	if err != nil {
		log.Fatalln("knock request error:", err)
	}
	if resp.StatusCode != http.StatusOK {
		log.Fatalln("knock request returned status code:", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalln("read knock response error:", err)
	}
	if string(body) != "OK" {
		log.Fatalln("knock response error:", string(body))
	}

	log.Println("knock success, opening proxy stream")

	stream, err := conn.OpenStream()
	if err != nil {
		log.Fatalln("open proxy stream error:", err)
	}
	reqBs := []byte(fmt.Sprintf("早上好中国现在我有冰淇淋 %s", time.Now().String()))
	var payload []byte
	payload = quicvarint.Append(payload, frameTypeProxyRequest)
	payload = quicvarint.Append(payload, uint64(len(reqBs)))
	payload = append(payload, reqBs...)

	if _, err := stream.Write(payload); err != nil {
		log.Fatalln("write proxy request error:", err)
	}

	time.Sleep(5 * time.Second)
}
