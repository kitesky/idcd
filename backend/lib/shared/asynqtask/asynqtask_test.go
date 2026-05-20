package asynqtask_test

import (
	"testing"
	"time"

	"github.com/kite365/idcd/lib/shared/asynqtask"
)

// D5 — RefundRetryFirstDelay + RefundRetrySecondDelay 必须等于"30 分钟内发
// 道歉邮箱"的窗口。任何调参后跑这个测试确认文案不需要同步改。
func TestRefundRetryTotalWindow(t *testing.T) {
	total := asynqtask.RefundRetryFirstDelay + asynqtask.RefundRetrySecondDelay
	if total != 30*time.Minute {
		t.Fatalf("D5 contract: first+second delay must total 30m (got %v); "+
			"如果故意改窗口，需要同步更新道歉邮箱模板里的 \"30 分钟\" 措辞 + admin 告警",
			total)
	}
}

func TestRefundRetryMaxAttempts(t *testing.T) {
	if asynqtask.RefundRetryMaxAttempts < 1 {
		t.Fatalf("RefundRetryMaxAttempts must be >= 1, got %d", asynqtask.RefundRetryMaxAttempts)
	}
}

// 防止后续把 task / queue 改成空串或重名（asynq mux dispatch 会安静地不工作）。
func TestNamesNonEmptyAndUnique(t *testing.T) {
	tasks := []string{
		asynqtask.TaskRefundRetry,
		asynqtask.TaskRefundApology,
		asynqtask.TaskAlertNotification,
	}
	queues := []string{
		asynqtask.QueueBilling,
		asynqtask.QueueNotifierDefault,
		asynqtask.QueueNotifierCritical,
	}
	checkUniqueNonEmpty(t, "task", tasks)
	checkUniqueNonEmpty(t, "queue", queues)
}

func checkUniqueNonEmpty(t *testing.T, kind string, names []string) {
	t.Helper()
	seen := make(map[string]bool, len(names))
	for _, n := range names {
		if n == "" {
			t.Fatalf("%s name is empty", kind)
		}
		if seen[n] {
			t.Fatalf("%s name %q duplicated", kind, n)
		}
		seen[n] = true
	}
}
