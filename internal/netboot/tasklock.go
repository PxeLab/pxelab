package netboot

// TaskBootDecision 决定 PXE 引导层对一台主机应下发什么内容（R7 失败锁定）。
// latestStatus 是该主机最新装机任务的状态，空串表示没有任何装机任务。
//
//	serve=true  → 引导进入安装流程（任务活跃：pending/installing）
//	locked=true → 失败锁定：最新任务 failed，引导必须回落本地磁盘/菜单，
//	              等待人工重试解锁（防安装循环刷掉装一半的机器）
//	两者皆 false → 无任务或任务已完成，走正常菜单流程
func TaskBootDecision(latestStatus string) (serve bool, locked bool) {
	switch latestStatus {
	case "pending", "installing":
		return true, false
	case "failed":
		return false, true
	default: // "" / done / 其他未知状态
		return false, false
	}
}
