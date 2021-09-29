package logger

import (
	"fmt"

	"k8s.io/klog"
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

func (l *Logger) WithSlack(stopCh chan struct{}, webhookUrl, msgPrefix string) *Logger {
	l.slack = NewSlack(stopCh, webhookUrl, msgPrefix)
	return l
}

func (l *Logger) Infof(format string, args ...interface{}) {
	if !klog.V(2) {
		return
	}
	msg := fmt.Sprintf(format, args...)
	klog.InfoDepth(l.depth, msg)
	if l.slack != nil {
		l.slack.InfoDepth(l.depth+1, msg)
	}
}

func (l *Logger) Errorf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	klog.ErrorDepth(l.depth, msg)
	if l.slack != nil {
		l.slack.ErrorDepth(l.depth+1, msg)
	}
}

func (l *Logger) Warningf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	klog.WarningDepth(l.depth, msg)
	if l.slack != nil {
		l.slack.WarningDepth(l.depth+1, msg)
	}
}
