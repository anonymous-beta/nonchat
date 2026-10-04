// nonchat server entrypoint.
//
// Credit: Anonymous-beta (chinedu)
package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/anonymous-beta/nonchat/internal/server"
)

func main() {
	addr := flag.String("addr", ":7777", "listen address (host:port)")
	flag.Parse()

	srv := server.New(*addr)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Println("shutting down nonchat server")
		srv.Shutdown()
	}()

	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
