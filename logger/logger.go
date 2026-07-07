package logger

import (
	"context"
	"fmt"

	"k8s.io/klog/v2"
)

type Logger struct {
	slack *Slack
	depth int
}

func NewLogger() *Logger {
	return &Logger{
		depth: 1,
		slack: nil, // default=disabled
	}
}

func (l *Logger) SetDepth(depth int) *Logger {
	l.depth = depth
	return l
}

func (l *Logger) WithSlack(ctx context.Context, webhookUrl, msgPrefix string) *Logger {
	l.slack = NewSlack(ctx, webhookUrl, msgPrefix)
	return l
}

// Infof 일반 정보 로그. notify-on-change 전환으로 Info 로그는 실제 이벤트
// (등록/해제, digest 변경, 패치 적용)에만 발생하므로 verbosity 게이트 없이 항상 기록한다.
func (l *Logger) Infof(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	klog.InfoDepth(l.depth, msg)
	if l.slack != nil {
		l.slack.InfoDepth(l.depth+1, msg)
	}
}

func (l *Logger) Errorf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	klog.ErrorDepth(l.depth, msg)
	if l.slack != nil {
		l.slack.ErrorDepth(l.depth+1, msg)
	}
}

func (l *Logger) Warningf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	klog.WarningDepth(l.depth, msg)
	if l.slack != nil {
		l.slack.WarningDepth(l.depth+1, msg)
	}
}

// Debugf 상세 흐름 로그. klog에만 기록하고 Slack으로는 미러하지 않는다.
// 배포 웨이브에서 대량 발생하는 이미지 변경 로그가 채널을 덮지 않도록 분리한다.
func (l *Logger) Debugf(format string, args ...any) {
	klog.InfoDepth(l.depth, fmt.Sprintf(format, args...))
}
