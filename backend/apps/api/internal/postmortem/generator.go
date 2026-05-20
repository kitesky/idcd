package postmortem

import (
	"fmt"
	"time"
)

type DraftInput struct {
	MonitorName string
	MonitorType string
	StartedAt   time.Time
	ResolvedAt  *time.Time
	Duration    time.Duration
	AlertCount  int
}

type TimelineEntry struct {
	Time  string `json:"time"`
	Event string `json:"event"`
}

type ActionItem struct {
	Item    string `json:"item"`
	Owner   string `json:"owner"`
	DueDate string `json:"due_date"`
}

type PostmortemDraft struct {
	Title       string
	Severity    string
	Impact      string
	Timeline    []TimelineEntry
	RootCause   string
	Resolution  string
	ActionItems []ActionItem
}

func severity(d time.Duration) string {
	switch {
	case d < 5*time.Minute:
		return "low"
	case d < 30*time.Minute:
		return "medium"
	case d < 2*time.Hour:
		return "high"
	default:
		return "critical"
	}
}

func durationStr(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%d 秒", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%d 分钟", int(d.Minutes()))
	}
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if m == 0 {
		return fmt.Sprintf("%d 小时", h)
	}
	return fmt.Sprintf("%d 小时 %d 分钟", h, m)
}

func actionItems(monitorType string) []ActionItem {
	due := time.Now().AddDate(0, 0, 7).Format("2006-01-02")
	switch monitorType {
	case "http":
		return []ActionItem{
			{Item: "检查服务器负载", Owner: "待指定", DueDate: due},
			{Item: "增加健康检查超时重试", Owner: "待指定", DueDate: due},
			{Item: "验证回滚计划", Owner: "待指定", DueDate: due},
		}
	case "dns":
		return []ActionItem{
			{Item: "检查 DNS 配置", Owner: "待指定", DueDate: due},
			{Item: "验证 TTL 设置", Owner: "待指定", DueDate: due},
			{Item: "添加备用 DNS", Owner: "待指定", DueDate: due},
		}
	default:
		return []ActionItem{
			{Item: "确认监控配置正确", Owner: "待指定", DueDate: due},
			{Item: "增加告警通知覆盖", Owner: "待指定", DueDate: due},
		}
	}
}

func GenerateDraft(input DraftInput) PostmortemDraft {
	sev := severity(input.Duration)
	dur := durationStr(input.Duration)

	title := fmt.Sprintf("[%s] %s 服务中断（%s）", sev, input.MonitorName, dur)
	impact := fmt.Sprintf("%s（%s）检测到异常，影响持续约 %s", input.MonitorName, input.MonitorType, dur)

	timeline := []TimelineEntry{
		{
			Time:  input.StartedAt.UTC().Format(time.RFC3339),
			Event: "故障开始",
		},
	}
	if input.ResolvedAt != nil {
		timeline = append(timeline, TimelineEntry{
			Time:  input.ResolvedAt.UTC().Format(time.RFC3339),
			Event: "故障结束",
		})
	} else {
		timeline = append(timeline, TimelineEntry{
			Time:  "待恢复",
			Event: "故障结束",
		})
	}

	resolution := ""
	if input.ResolvedAt != nil {
		resolution = fmt.Sprintf("故障于 %s 恢复，共持续 %s",
			input.ResolvedAt.UTC().Format(time.RFC3339), dur)
	}

	return PostmortemDraft{
		Title:       title,
		Severity:    sev,
		Impact:      impact,
		Timeline:    timeline,
		RootCause:   "[待补充] 初步判断为基础设施异常，具体根因需进一步分析",
		Resolution:  resolution,
		ActionItems: actionItems(input.MonitorType),
	}
}
