package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"syscall"
)

const (
	html = `
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <title>Hello Universe</title>
</head>
<body>
  <h3>Hello World!</h3>
</body>
</html>
`
)

var (
	httpAddr    string
	certFile    string
	keyFile     string
	enableHttps bool
)

func main() {
	flag.StringVar(&httpAddr, "http", "0.0.0.0:443", "http service address")
	flag.StringVar(&certFile, "tls-cert-file", certFile, "File containing the default x509 Certificate for https.")
	flag.StringVar(&keyFile, "tls-private-key-file", keyFile, "File containing the default x509 Certificate for https.")
	flag.BoolVar(&enableHttps, "https", false, "enable https")
	flag.Parse()

	fmt.Printf("starting helloworld...\n")

	mux := http.NewServeMux()
	mux.HandleFunc("/", httpHandler)
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	httpServer := http.Server{
		Handler: mux,
	}

	go func() {
		if enableHttps {
			httpServer.Addr = ":443"
			log.Fatalf("https error: %+v", httpServer.ListenAndServeTLS(certFile, keyFile))
		} else {
			httpServer.Addr = ":8080"
			log.Fatal(httpServer.ListenAndServe())
		}
	}()

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	<-signalChan

	log.Printf("Shutdown signal received shutting down gracefully...\n")

	_ = httpServer.Shutdown(context.Background())
}

func httpHandler(resp http.ResponseWriter, _ *http.Request) {
	_, _ = fmt.Fprintf(resp, html)
}
