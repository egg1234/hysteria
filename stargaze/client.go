package main

import (
	"context"
	"crypto/tls"
	"flag"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"

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
	if resp.StatusCode != httpStatusKnocked {
		log.Fatalln("knock failed, status code:", resp.StatusCode)
	}
	_ = resp.Body.Close()

	log.Println("knock success, opening proxy stream")

	stream, err := conn.OpenStream()
	if err != nil {
		log.Fatalln("open proxy stream error:", err)
	}

	target := "ipinfo.io:80"

	var reqBs []byte
	reqBs = quicvarint.Append(reqBs, frameTypeProxyRequest)
	reqBs = quicvarint.Append(reqBs, uint64(len(target)))
	reqBs = append(reqBs, []byte(target)...)

	_, err = stream.Write(reqBs)
	if err != nil {
		log.Fatalln("write proxy request error:", err)
	}

	log.Println("proxy request sent, waiting for response")

	respCode := make([]byte, 1)
	_, err = io.ReadFull(stream, respCode)
	if err != nil {
		log.Fatalln("read proxy response error:", err)
	}
	if respCode[0] != 0 {
		log.Fatalln("proxy request failed, error code:", respCode[0])
	}

	log.Println("proxy connection established")

	_, err = stream.Write([]byte("GET / HTTP/1.1\r\n" +
		"Host: ipinfo.io\r\n" +
		"Connection: close\r\n\r\n"))
	if err != nil {
		log.Fatalln("write HTTP request error:", err)
	}

	log.Println("HTTP request sent, waiting for response")

	hResp, err := io.ReadAll(stream)
	if err != nil {
		log.Fatalln("read HTTP response error:", err)
	}

	log.Printf("HTTP response:\n%s", hResp)
}
