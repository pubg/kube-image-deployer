package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"sync"
	"time"

	"k8s.io/klog/v2"
)

// maxBlockTextLength Slack 블록 텍스트 길이 제한 (실제 제한 3000자)
const maxBlockTextLength = 3000

// maxBlocksPerMessage Slack 메시지당 블록 수 제한 (실제 제한 50개)
const maxBlocksPerMessage = 50

type Slack struct {
	webhookUrl string
	msgPrefix  string
	httpClient *http.Client
	pool       []message
	mutex      sync.RWMutex
}

// SlackRequestBody Block Kit 페이로드.
// 로그를 rich_text_preformatted 블록 하나로 뭉쳐 보내야
// Slack에서 메시지가 여러 개로 쪼개지지 않는다.
type SlackRequestBody struct {
	Blocks []SlackBlock `json:"blocks"`
}

type SlackBlock struct {
	Type     string              `json:"type"`
	Elements []SlackBlockElement `json:"elements"`
}

type SlackBlockElement struct {
	Type     string             `json:"type"`
	Elements []SlackTextElement `json:"elements"`
}

type SlackTextElement struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type message struct {
	level string
	msg   string
	time  time.Time
	file  string
	line  int
}

func NewSlack(ctx context.Context, webhookUrl, msgPrefix string) *Slack {
	s := &Slack{
		webhookUrl: webhookUrl,
		msgPrefix:  msgPrefix,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		pool:       make([]message, 0),
		mutex:      sync.RWMutex{},
	}

	s.start(ctx)

	return s
}

func (s *Slack) InfoDepth(depth int, msg string) {
	s.pushMessage(depth+1, "info", msg)

}

func (s *Slack) WarningDepth(depth int, msg string) {
	s.pushMessage(depth+1, "warn", msg)
}

func (s *Slack) ErrorDepth(depth int, msg string) {
	s.pushMessage(depth+1, "error", msg)
}

func (s *Slack) pushMessage(depth int, level, msg string) {
	_, file, line, ok := runtime.Caller(depth)

	if !ok {
		file = "???"
		line = 0
	}

	s.mutex.Lock()
	s.pool = append(s.pool, message{level: level, msg: msg, time: time.Now(), file: file, line: line})
	s.mutex.Unlock()
}

func (s *Slack) start(ctx context.Context) {
	go func() {
		for {
			select {
			case <-ctx.Done(): // exit goroutine
				s.flush() // 종료 전 남은 메시지 전송
				return
			case <-time.After(time.Second):
				s.mutex.RLock()
				l := len(s.pool)
				s.mutex.RUnlock()

				if l == 0 {
					break
				}

				<-time.After(time.Second) // 후속 메시지를 모으기 위해 잠시 대기

				s.flush()
			}
		}
	}()
}

// flush pool에 쌓인 메시지를 모두 전송한다.
func (s *Slack) flush() {

	s.mutex.Lock()              // lock the pool
	pool := s.pool              // copy the pool
	s.pool = make([]message, 0) // clear the pool
	s.mutex.Unlock()            // unlock the pool

	if len(pool) == 0 {
		return
	}

	lines := make([]string, 0, len(pool))
	for _, msg := range pool {
		lines = append(lines, fmt.Sprintf("%s[%s][%s][%s:%d] %s", s.msgPrefix, msg.time.Format(time.RFC3339), msg.level, msg.file, msg.line, msg.msg))
	}

	blocks := buildBlocks(lines)

	// Slack 메시지당 블록 수 제한을 넘으면 나눠 보낸다
	for len(blocks) > 0 {
		batch := blocks
		if len(batch) > maxBlocksPerMessage {
			batch = blocks[:maxBlocksPerMessage]
		}
		blocks = blocks[len(batch):]

		if err := s.send(batch); err != nil {
			klog.Errorf("error sending to slack: %s", err)
		}
	}

}

// buildBlocks 로그 라인들을 rich_text_preformatted 블록 목록으로 변환한다.
// 여러 라인을 하나의 블록으로 뭉치되, 블록당 텍스트 길이 제한을 지키도록 줄 단위로 청킹한다.
func buildBlocks(lines []string) []SlackBlock {

	blocks := make([]SlackBlock, 0)
	current := ""

	appendBlock := func(text string) {
		blocks = append(blocks, SlackBlock{
			Type: "rich_text",
			Elements: []SlackBlockElement{{
				Type:     "rich_text_preformatted",
				Elements: []SlackTextElement{{Type: "text", Text: text}},
			}},
		})
	}

	for _, line := range lines {

		// 단일 라인이 제한을 넘으면 강제 분할
		for len(line) > maxBlockTextLength {
			if current != "" {
				appendBlock(current)
				current = ""
			}
			appendBlock(line[:maxBlockTextLength])
			line = line[maxBlockTextLength:]
		}

		switch {
		case current == "":
			current = line
		case len(current)+1+len(line) <= maxBlockTextLength:
			current += "\n" + line
		default:
			appendBlock(current)
			current = line
		}
	}

	if current != "" {
		appendBlock(current)
	}

	return blocks

}

func (s *Slack) send(blocks []SlackBlock) error {

	slackBody, err := json.Marshal(SlackRequestBody{Blocks: blocks})

	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, s.webhookUrl, bytes.NewBuffer(slackBody))

	if err != nil {
		return err
	}

	req.Header.Add("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(resp.Body); err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("slack returned status %d: %s", resp.StatusCode, buf.String())
	}

	if buf.String() != "ok" {
		return fmt.Errorf("non-ok response returned from Slack: %s", buf.String())
	}

	return nil
}
