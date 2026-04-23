package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

type appMessage struct {
	Event    *eventMessage    `json:"Event,omitempty"`
	Request  *requestMessage  `json:"Request,omitempty"`
	Response *responseMessage `json:"Response,omitempty"`
}

type eventMessage struct {
	ID    string          `json:"id"`
	Event string          `json:"event"`
	Data  json.RawMessage `json:"data,omitempty"`
}

type requestMessage struct {
	ID      string          `json:"id"`
	Command string          `json:"command"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

type responseMessage struct {
	ID   string          `json:"id"`
	Code *int            `json:"code,omitempty"`
	Msg  string          `json:"msg,omitempty"`
	Data json.RawMessage `json:"data,omitempty"`
}

type fileMonitorEvent struct {
	NewLine string `json:"NewLine,omitempty"`
}

type instructionLog struct {
	Header struct {
		Namespace string `json:"namespace"`
		Name      string `json:"name"`
	} `json:"header"`
	Payload struct {
		IsFinal bool `json:"is_final"`
		Results []struct {
			Text string `json:"text"`
		} `json:"results"`
	} `json:"payload"`
}

type shellResult struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exit_code"`
}

type clientSession struct {
	conn          *websocket.Conn
	debug         bool
	abortAfterASR bool

	writeMu   sync.Mutex
	pending   map[string]chan responseMessage
	pendingMu sync.Mutex
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

var requestSeq atomic.Uint64

func main() {
	addr := flag.String("addr", ":4399", "websocket listen address")
	debug := flag.Bool("debug", false, "print raw events for debugging")
	abortAfterASR := flag.Bool("abort-after-asr", true, "restart mico_aivs_lab after final ASR result")
	flag.Parse()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		handleWebSocket(w, r, *debug, *abortAfterASR)
	})

	log.Printf("listening on %s", *addr)
	log.Fatal(http.ListenAndServe(*addr, nil))
}

func handleWebSocket(w http.ResponseWriter, r *http.Request, debug bool, abortAfterASR bool) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	session := &clientSession{
		conn:          conn,
		debug:         debug,
		abortAfterASR: abortAfterASR,
		pending:       map[string]chan responseMessage{},
	}

	log.Printf("client connected: %s", r.RemoteAddr)
	defer log.Printf("client disconnected: %s", r.RemoteAddr)

	conn.SetReadLimit(16 << 20)
	conn.SetPongHandler(func(string) error {
		_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})
	_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))

	done := make(chan struct{})
	go keepAlive(session, done)
	defer close(done)

	for {
		messageType, payload, err := conn.ReadMessage()
		if err != nil {
			if !websocket.IsCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) &&
				!strings.Contains(err.Error(), "use of closed network connection") {
				log.Printf("read failed: %v", err)
			}
			return
		}

		switch messageType {
		case websocket.TextMessage:
			_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))
			if err := session.handleTextMessage(payload); err != nil {
				log.Printf("invalid text message: %v", err)
			}
		case websocket.BinaryMessage:
			_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))
			if debug {
				log.Printf("received binary stream: %d bytes", len(payload))
			}
		}
	}
}

func keepAlive(session *clientSession, done <-chan struct{}) {
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			if err := session.conn.WriteControl(
				websocket.PingMessage,
				[]byte("ping"),
				time.Now().Add(5*time.Second),
			); err != nil {
				return
			}
		}
	}
}

func (s *clientSession) handleTextMessage(payload []byte) error {
	var msg appMessage
	if err := json.Unmarshal(payload, &msg); err != nil {
		return err
	}

	switch {
	case msg.Event != nil:
		return s.handleEvent(*msg.Event)
	case msg.Request != nil:
		if s.debug {
			log.Printf("received unsupported request: command=%s id=%s", msg.Request.Command, msg.Request.ID)
		}
	case msg.Response != nil:
		s.onResponse(*msg.Response)
		if s.debug {
			log.Printf("received response: id=%s code=%v", msg.Response.ID, msg.Response.Code)
		}
	default:
		if s.debug {
			log.Printf("received unknown text message: %s", string(payload))
		}
	}

	return nil
}

func (s *clientSession) handleEvent(event eventMessage) error {
	if s.debug {
		log.Printf("event=%s payload=%s", event.Event, string(event.Data))
	}

	if event.Event != "instruction" {
		return nil
	}

	line, err := parseInstructionLine(event.Data)
	if err != nil {
		return err
	}
	if line == "" {
		return nil
	}

	var logLine instructionLog
	if err := json.Unmarshal([]byte(line), &logLine); err != nil {
		return fmt.Errorf("decode instruction log: %w", err)
	}

	if logLine.Header.Namespace != "SpeechRecognizer" || logLine.Header.Name != "RecognizeResult" {
		return nil
	}
	if !logLine.Payload.IsFinal || len(logLine.Payload.Results) == 0 {
		return nil
	}

	text := strings.TrimSpace(logLine.Payload.Results[0].Text)
	if text == "" {
		return nil
	}

	s.onASR(text, s.abortAfterASR)
	return nil
}

func (s *clientSession) onASR(text string, abort bool) {
	log.Printf("xiaoai command: %s", text)
	if !abort {
		return
	}

	go func() {
		if err := s.abortXiaoAI(5 * time.Second); err != nil {
			log.Printf("abort xiaoai failed: %v", err)
			return
		}
		log.Printf("xiaoai aborted after final ASR")
	}()
}

func (s *clientSession) abortXiaoAI(timeout time.Duration) error {
	resp, err := s.call("run_shell", "/etc/init.d/mico_aivs_lab restart >/dev/null 2>&1", timeout)
	if err != nil {
		return err
	}

	var result shellResult
	if len(resp.Data) > 0 && string(resp.Data) != "null" {
		if err := json.Unmarshal(resp.Data, &result); err != nil {
			return fmt.Errorf("decode run_shell result: %w", err)
		}
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("exit_code=%d stderr=%s", result.ExitCode, strings.TrimSpace(result.Stderr))
	}
	return nil
}

func (s *clientSession) call(command string, payload any, timeout time.Duration) (responseMessage, error) {
	id := fmt.Sprintf("req-%d", requestSeq.Add(1))

	var rawPayload json.RawMessage
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return responseMessage{}, fmt.Errorf("encode payload: %w", err)
		}
		rawPayload = data
	}

	req := appMessage{
		Request: &requestMessage{
			ID:      id,
			Command: command,
			Payload: rawPayload,
		},
	}

	ch := make(chan responseMessage, 1)
	s.pendingMu.Lock()
	s.pending[id] = ch
	s.pendingMu.Unlock()

	if err := s.writeJSON(req); err != nil {
		s.pendingMu.Lock()
		delete(s.pending, id)
		s.pendingMu.Unlock()
		return responseMessage{}, err
	}

	select {
	case resp := <-ch:
		if resp.Code != nil && *resp.Code != 0 {
			return resp, fmt.Errorf("code=%d msg=%s", *resp.Code, resp.Msg)
		}
		return resp, nil
	case <-time.After(timeout):
		s.pendingMu.Lock()
		delete(s.pending, id)
		s.pendingMu.Unlock()
		return responseMessage{}, fmt.Errorf("request timeout: %s", command)
	}
}

func (s *clientSession) onResponse(resp responseMessage) {
	s.pendingMu.Lock()
	ch, ok := s.pending[resp.ID]
	if ok {
		delete(s.pending, resp.ID)
	}
	s.pendingMu.Unlock()

	if ok {
		ch <- resp
	}
}

func (s *clientSession) writeJSON(msg appMessage) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	_ = s.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	return s.conn.WriteJSON(msg)
}

func parseInstructionLine(data json.RawMessage) (string, error) {
	if len(data) == 0 || string(data) == "null" {
		return "", nil
	}

	var newFile string
	if err := json.Unmarshal(data, &newFile); err == nil {
		if newFile == "NewFile" {
			return "", nil
		}
	}

	var fileEvent fileMonitorEvent
	if err := json.Unmarshal(data, &fileEvent); err != nil {
		return "", fmt.Errorf("decode file monitor event: %w", err)
	}

	return fileEvent.NewLine, nil
}
