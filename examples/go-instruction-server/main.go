package main

import (
	"flag"
	"log"
	"time"

	"github.com/idootop/open-xiaoai/examples/go-instruction-server/internal/server"
	"github.com/idootop/open-xiaoai/examples/go-instruction-server/internal/speaker"
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
	spk := speaker.New()

	srv := server.New(cfg, func(session *server.Session, text string) {
		log.Printf("xiaoai command: %s", text)

		if text == "测试播放文字" {
			go func() {
				if err := session.AbortXiaoAI(5 * time.Second); err != nil {
					log.Printf("abort xiaoai failed: %v", err)
					return
				}
				time.Sleep(2 * time.Second)
				if err := spk.PlayText(session, "你好，很高兴认识你！", 30*time.Second); err != nil {
					log.Printf("play text failed: %v", err)
					return
				}
				log.Printf("played demo reply text")
			}()
			return
		}

		if text == "测试长段播放文字" {
			go func() {
				if err := session.AbortXiaoAI(5 * time.Second); err != nil {
					log.Printf("abort xiaoai failed: %v", err)
					return
				}
				time.Sleep(2 * time.Second)

				chunks := []string{
					"你好，我现在开始演示流式文字播放。",
					"这段回复不会一次性整段播完，",
					"而是像 migpt 一样，",
					"按多段文字顺序调用音箱本地 TTS。",
					"每一段播完之后，",
					"再继续播放下一段。",
				}

				if err := spk.PlayTextStream(session, chunks, 30*time.Second, 100*time.Millisecond); err != nil {
					log.Printf("play text stream failed: %v", err)
					return
				}
				log.Printf("played demo reply text stream")
			}()
			return
		}

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
