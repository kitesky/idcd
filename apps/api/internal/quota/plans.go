// Package quota implements plan-based quota enforcement for idcd.
// Unlimited resources are represented as 0.
package quota

import (
	"fmt"

	"github.com/kite365/idcd/lib/shared/apperr"
)

// CodeQuotaExceeded is the machine-readable error code returned when a quota
// limit is reached.
const CodeQuotaExceeded apperr.Code = "QUOTA_EXCEEDED"

// QuotaExceeded creates a quota-exceeded application error.
func QuotaExceeded(msg string) *apperr.Error {
	return &apperr.Error{
		Code:    CodeQuotaExceeded,
		Message: msg,
	}
}

// PlanLimits holds the quota upper bounds for a single subscription plan.
// A value of 0 means unlimited (no enforcement).
type PlanLimits struct {
	MaxMonitors     int // 0 = unlimited
	MinIntervalS    int // smallest allowed check interval in seconds
	MaxNodes        int // 0 = unlimited
	MaxChannels     int // 0 = unlimited
	MaxStatusPages  int // 0 = unlimited
	MaxAPIDailyReqs int // 0 = unlimited
}

// planTable stores the canonical limits for each plan name.
var planTable = map[string]PlanLimits{
	"free": {
		MaxMonitors:     3,
		MinIntervalS:    300,
		MaxNodes:        1,
		MaxChannels:     1,
		MaxStatusPages:  0, // free has 0 status pages
		MaxAPIDailyReqs: 100,
	},
	"pro": {
		MaxMonitors:     50,
		MinIntervalS:    60,
		MaxNodes:        5,
		MaxChannels:     5,
		MaxStatusPages:  3,
		MaxAPIDailyReqs: 5000,
	},
	"team": {
		MaxMonitors:     200,
		MinIntervalS:    60,
		MaxNodes:        10,
		MaxChannels:     20,
		MaxStatusPages:  10,
		MaxAPIDailyReqs: 50000,
	},
	"business": {
		MaxMonitors:     0,
		MinIntervalS:    30,
		MaxNodes:        0,
		MaxChannels:     0,
		MaxStatusPages:  0,
		MaxAPIDailyReqs: 0,
	},
}

// Limits returns the PlanLimits for the given plan name.
// Unknown plans fall back to "free" limits for safety.
func Limits(plan string) PlanLimits {
	if l, ok := planTable[plan]; ok {
		return l
	}
	return planTable["free"]
}

// planDisplayName maps plan identifiers to user-facing names.
func planDisplayName(plan string) string {
	switch plan {
	case "free":
		return "Free"
	case "pro":
		return "Pro"
	case "team":
		return "Team"
	case "business":
		return "Business"
	default:
		return "Free"
	}
}

// CheckMonitorCount verifies that adding one more monitor does not exceed the
// plan's monitor limit. current is the number of monitors the user already has.
// Returns nil if allowed; returns a QUOTA_EXCEEDED error otherwise.
func CheckMonitorCount(plan string, current int) error {
	l := Limits(plan)
	if l.MaxMonitors == 0 {
		return nil // unlimited
	}
	if current >= l.MaxMonitors {
		return QuotaExceeded(fmt.Sprintf(
			"您的 %s 档已达到 %d 个监控项上限。升级套餐可管理更多监控项。",
			planDisplayName(plan), l.MaxMonitors,
		))
	}
	return nil
}

// CheckMonitorInterval verifies that the requested check interval meets the
// plan's minimum interval requirement.
func CheckMonitorInterval(plan string, intervalS int) error {
	l := Limits(plan)
	if intervalS < l.MinIntervalS {
		return QuotaExceeded(fmt.Sprintf(
			"您的 %s 档最小检测间隔为 %d 秒。",
			planDisplayName(plan), l.MinIntervalS,
		))
	}
	return nil
}

// CheckNodeCount verifies that the requested concurrent node count does not
// exceed the plan's node limit.
func CheckNodeCount(plan string, nodeCount int) error {
	l := Limits(plan)
	if l.MaxNodes == 0 {
		return nil // unlimited
	}
	if nodeCount > l.MaxNodes {
		return QuotaExceeded(fmt.Sprintf(
			"您的 %s 档最多支持 %d 个并发节点。",
			planDisplayName(plan), l.MaxNodes,
		))
	}
	return nil
}

// CheckChannelCount verifies that adding one more alert channel does not exceed
// the plan's channel limit. current is the user's existing channel count.
func CheckChannelCount(plan string, current int) error {
	l := Limits(plan)
	if l.MaxChannels == 0 {
		return nil // unlimited
	}
	if current >= l.MaxChannels {
		return QuotaExceeded(fmt.Sprintf(
			"您的 %s 档已达到 %d 个告警通道上限。",
			planDisplayName(plan), l.MaxChannels,
		))
	}
	return nil
}

// CheckStatusPageCount verifies that adding one more status page does not exceed
// the plan's status page limit. current is the user's existing count.
func CheckStatusPageCount(plan string, current int) error {
	l := Limits(plan)
	if l.MaxStatusPages == 0 && plan != "free" {
		// business unlimited
		return nil
	}
	if l.MaxStatusPages == 0 && plan == "free" {
		// free has 0 allowed status pages
		return QuotaExceeded(fmt.Sprintf(
			"您的 %s 档不支持状态页功能。升级 Pro 档可创建最多 3 个状态页。",
			planDisplayName(plan),
		))
	}
	if current >= l.MaxStatusPages {
		return QuotaExceeded(fmt.Sprintf(
			"您的 %s 档已达到 %d 个状态页上限。",
			planDisplayName(plan), l.MaxStatusPages,
		))
	}
	return nil
}
