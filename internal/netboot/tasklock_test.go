package netboot

import "testing"

func TestTaskBootDecision(t *testing.T) {
	cases := []struct {
		name       string
		status     string
		wantServe  bool
		wantLocked bool
	}{
		{"无任务（空状态）走正常菜单", "", false, false},
		{"pending 下发安装", "pending", true, false},
		{"installing 下发安装", "installing", true, false},
		{"failed 锁定不再重装", "failed", false, true},
		{"done 走正常菜单", "done", false, false},
		{"未知状态走正常菜单", "cancelled", false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			serve, locked := TaskBootDecision(c.status)
			if serve != c.wantServe || locked != c.wantLocked {
				t.Fatalf("TaskBootDecision(%q) = (%v, %v), want (%v, %v)",
					c.status, serve, locked, c.wantServe, c.wantLocked)
			}
		})
	}
}
