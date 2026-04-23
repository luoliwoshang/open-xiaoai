package main

import (
	"flag"
	"log"
	"time"

	"github.com/idootop/open-xiaoai/examples/go-instruction-server/internal/server"
)

func main() {
	addr := flag.String("addr", ":4399", "websocket listen address")
	debug := flag.Bool("debug", false, "print raw events for debugging")
	abortAfterASR := flag.Bool("abort-after-asr", true, "restart mico_aivs_lab after final ASR result")
	flag.Parse()

	cfg := server.Config{
		Addr:  *addr,
		Debug: *debug,
	}

	srv := server.New(cfg, func(session *server.Session, text string) {
		log.Printf("xiaoai command: %s", text)
		if !*abortAfterASR {
			return
		}

		go func() {
			if err := session.AbortXiaoAI(5 * time.Second); err != nil {
				log.Printf("abort xiaoai failed: %v", err)
				return
			}
			log.Printf("xiaoai aborted after final ASR")
		}()
	})

	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
