package constants

import "time"

// 数据保留期常量.
//
// 改这些值前需要检查:
//   - Timescale 连续聚合的窗口长度 (probe_result hot/cold)
//   - 数据库 partition / drop chunk 的脚本
//   - 证书申请失败的退款时效 (D5)
const (
	// ProbeResultHotRetention 是 probe_result 热表的保留期.
	// 热表保留高基数原始数据, 用于近 7 天的明细查询.
	ProbeResultHotRetention = 7 * 24 * time.Hour

	// ProbeResultColdRetention 是 probe_result 冷归档的保留期 (聚合后).
	// 冷归档每天聚合一次, 保留 90 天用于趋势 / 可用率统计.
	ProbeResultColdRetention = 90 * 24 * time.Hour

	// CertOrderRetention 是证书订单 (含失败 / 退款) 的最短保留期.
	// 对应 D5 退款重试窗口 + 用户申诉窗口.
	CertOrderRetention = 30 * 24 * time.Hour
)
