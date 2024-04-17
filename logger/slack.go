package logger

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"sync"
	"time"

	"k8s.io/klog/v2"
)

type Slack struct {
	webhookUrl string
	msgPrefix  string
	httpClient *http.Client
	pool       []message
	mutex      sync.RWMutex
}

type SlackRequestBody struct {
	Text string `json:"text"`
}

type message struct {
	level string
	msg   string
	time  time.Time
	file  string
	line  int
}

func NewSlack(stopCh chan struct{}, webhookUrl, msgPrefix string) *Slack {
	s := &Slack{
		webhookUrl: webhookUrl,
		msgPrefix:  msgPrefix,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		pool:       make([]message, 0),
		mutex:      sync.RWMutex{},
	}

	s.start(stopCh)

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

func (s *Slack) start(stopCh chan struct{}) {
	go func() {
		for {
			select {
			case <-stopCh: // exit goroutine
				return
			case <-time.After(time.Second):
				s.mutex.RLock()
				l := len(s.pool)
				s.mutex.RUnlock()

				if l == 0 {
					break
				}

				<-time.After(time.Second) // wait for a second before sending the next message

				var text string

				s.mutex.Lock()              // lock the pool
				pool := s.pool              // copy the pool
				s.pool = make([]message, 0) // clear the pool
				s.mutex.Unlock()            // unlock the pool

				for _, msg := range pool {
					text += fmt.Sprintf("```%s[%s][%s][%s:%d] %s```\n", s.msgPrefix, msg.time.Format(time.RFC3339), msg.level, msg.file, msg.line, msg.msg)
				}

				if err := s.send(text); err != nil {
					klog.Errorf("error sending to slack: %s\n%s", err, text)
				}
			}
		}
	}()
}

func (s *Slack) send(msg string) error {

	slackBody, _ := json.Marshal(SlackRequestBody{Text: msg})
	req, err := http.NewRequest(http.MethodPost, s.webhookUrl, bytes.NewBuffer(slackBody))

	if err != nil {
		return err
	}

	req.Header.Add("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}

	buf := new(bytes.Buffer)
	buf.ReadFrom(resp.Body)

	if buf.String() != "ok" {
		return fmt.Errorf("non-ok response returned from Slack")
	}

	return nil
}
