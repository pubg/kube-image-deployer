package events

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"k8s.io/klog/v2"
)

// digestDisplayLen digest 표시 축약 길이
const digestDisplayLen = 12

// maxSectionTextLength 섹션 하나의 mrkdwn 텍스트 길이 상한 (실제 제한 3,000자 — 여유 확보)
const maxSectionTextLength = 2900

// maxBlocksPerMessage Slack 메시지당 블록 수 제한 (실제 제한 50개)
const maxBlocksPerMessage = 50

// SlackNotifier Summary를 Block Kit 메시지로 만들어 웹훅으로 전송한다.
// 로그 미러(logger.Slack)와 달리 구조화된 이미지 변경 알림 전용이다.
type SlackNotifier struct {
	webhookUrl  string
	clusterName string
	windowSec   uint
	httpClient  *http.Client
}

func NewSlackNotifier(webhookUrl, clusterName string, windowSec uint) *SlackNotifier {
	return &SlackNotifier{
		webhookUrl:  webhookUrl,
		clusterName: clusterName,
		windowSec:   windowSec,
		httpClient:  &http.Client{Timeout: 10 * time.Second},
	}
}

// Notify Summary를 전송한다. 블록 수가 Slack 한도를 넘으면 여러 메시지로 나눠 보낸다.
// 전송 실패는 로그만 남긴다 (best-effort — 다음 윈도우가 곧 온다).
func (n *SlackNotifier) Notify(summary Summary) {
	blocks := BuildBlocks(summary, n.clusterName, n.windowSec, time.Now())
	pages := paginateBlocks(blocks)
	sent := 0
	for _, page := range pages {
		if err := n.send(page); err != nil {
			klog.Errorf("slack image update notify error: %v", err)
			continue
		}
		sent++
	}
	// 발송 여부를 파드 로그로 확인할 수 있도록 성공도 klog에 남긴다 (Slack 미러 아님)
	klog.Infof("slack image update notify sent: images=%d workloads=%d containers=%d pages=%d/%d",
		summary.TotalImages, summary.TotalWorkloads, summary.TotalContainers, sent, len(pages))
}

func (n *SlackNotifier) send(blocks []map[string]any) error {

	body, err := json.Marshal(map[string]any{"blocks": blocks})

	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, n.webhookUrl, bytes.NewBuffer(body))

	if err != nil {
		return err
	}

	req.Header.Add("Content-Type", "application/json")

	resp, err := n.httpClient.Do(req)
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

// BuildBlocks Summary를 이미지 중심 mrkdwn 섹션형 Block Kit 블록 목록으로 변환한다.
//
//	[header]  🚀 Image Updates — <cluster>
//	[context] 수집 20s · 이미지 N종 · workload M개 · 컨테이너 K개 · <time>
//	[divider]
//	[section × 이미지 묶음]
//	  `/api:release-*` → `release-1102 (@sha256:...)`
//	   └ default · deployment/api (app)
//	   └ default · deployment/worker (app, sidecar)
//
// 같은 이미지(감시 대상 + 적용 digest)가 여러 workload에 적용되면 하나의 묶음 아래에 나열된다.
func BuildBlocks(summary Summary, clusterName string, windowSec uint, at time.Time) []map[string]any {

	blocks := []map[string]any{
		{
			"type": "header",
			"text": map[string]any{"type": "plain_text", "text": "🚀 Image Updates — " + clusterName, "emoji": true},
		},
		{
			"type": "context",
			"elements": []any{map[string]any{
				"type": "mrkdwn",
				"text": fmt.Sprintf("수집 %ds · 이미지 *%d*종 · workload *%d*개 · 컨테이너 *%d*개 · %s",
					windowSec, summary.TotalImages, summary.TotalWorkloads, summary.TotalContainers, at.UTC().Format("2006-01-02 15:04 UTC")),
			}},
		},
		{"type": "divider"},
	}

	return append(blocks, imageSections(summary.Images)...)

}

// imageEntryLines 이미지 하나의 표시 라인들 — 첫 줄은 이미지 헤더, 이후 적용 대상들
func imageEntryLines(image ImageSummary) []string {

	watch := image.WatchURL
	if image.WatchTag != "" {
		watch += ":" + image.WatchTag
	}

	lines := []string{fmt.Sprintf("`%s` → `%s`",
		escapeMrkdwn(displayImage(watch)), escapeMrkdwn(displayTo(image.To, image.ToTag)))}

	for _, target := range image.Targets {
		lines = append(lines, fmt.Sprintf(" └ %s · %s/%s (%s)",
			escapeMrkdwn(target.Namespace), target.Kind, escapeMrkdwn(target.Name),
			escapeMrkdwn(strings.Join(target.Containers, ", "))))
	}

	return lines

}

// imageSections 이미지 묶음들을 섹션 텍스트 한도에 맞춰 mrkdwn 섹션들로 패킹한다.
// 이미지 사이는 빈 줄로 구분하고, 한도를 넘으면 라인 단위로 다음 섹션으로 넘긴다.
func imageSections(images []ImageSummary) []map[string]any {

	sections := make([]map[string]any, 0, 1)
	current := ""

	appendLine := func(line string, newEntry bool) {
		separator := "\n"
		if newEntry && current != "" {
			separator = "\n\n" // 이미지 묶음 사이 빈 줄
		}
		switch {
		case current == "":
			current = line
		case len(current)+len(separator)+len(line) > maxSectionTextLength:
			sections = append(sections, mrkdwnSection(current))
			current = line
		default:
			current += separator + line
		}
	}

	for _, image := range images {
		for i, line := range imageEntryLines(image) {
			appendLine(line, i == 0)
		}
	}

	if current != "" {
		sections = append(sections, mrkdwnSection(current))
	}

	return sections

}

func mrkdwnSection(text string) map[string]any {
	return map[string]any{
		"type": "section",
		"text": map[string]any{"type": "mrkdwn", "text": text},
	}
}

func paginateBlocks(blocks []map[string]any) [][]map[string]any {
	pages := make([][]map[string]any, 0, 1)
	for len(blocks) > 0 {
		n := min(len(blocks), maxBlocksPerMessage)
		pages = append(pages, blocks[:n])
		blocks = blocks[n:]
	}
	return pages
}

// truncateDigest "@sha256:..." digest를 표시용 길이로 축약한다.
func truncateDigest(ref string) string {
	const marker = "@sha256:"
	i := strings.Index(ref, marker)
	if i < 0 {
		return ref
	}
	digest := ref[i+len(marker):]
	if len(digest) > digestDisplayLen {
		digest = digest[:digestDisplayLen]
	}
	return ref[:i] + marker + digest
}

// displayImage 레지스트리 호스트를 제거하고 digest를 축약한 표시용 문자열.
// "123.dkr.ecr.../payment:release-*" → "/payment:release-*", "busybox:1.34.*" → 그대로.
func displayImage(ref string) string {
	ref = truncateDigest(ref)
	if i := strings.Index(ref, "/"); i > 0 {
		host := ref[:i]
		if strings.ContainsAny(host, ".:") || host == "localhost" {
			return ref[i:]
		}
	}
	return ref
}

// displayTo 적용된 이미지의 표시 문자열.
// digest가 취득된 원본 태그가 있으면 "dev-111 (@sha256:...)" 형태로,
// 없으면 digest만 표기한다.
func displayTo(to, toTag string) string {

	display := displayImage(to)
	if i := strings.Index(to, "@"); i >= 0 {
		display = truncateDigest(to[i:]) // From과 같은 url 반복을 피하고 digest만
	}

	if toTag != "" {
		return toTag + " (" + display + ")"
	}
	return display

}

// escapeMrkdwn Slack mrkdwn 제어 문자를 이스케이프한다.
func escapeMrkdwn(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}
