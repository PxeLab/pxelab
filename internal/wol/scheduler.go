package wol

import (
	"context"
	"log/slog"
	"net"
	"time"

	"github.com/pxelab/pxelab/internal/config"
	"github.com/pxelab/pxelab/internal/eventbus"
	"github.com/pxelab/pxelab/internal/models"
	"github.com/pxelab/pxelab/internal/store"
)

type Scheduler struct {
	store    store.Interface
	config   *config.Config
	eventBus *eventbus.Bus
	stopCh   chan struct{}
}

func NewScheduler(st store.Interface, cfg *config.Config, bus *eventbus.Bus) *Scheduler {
	return &Scheduler{
		store:    st,
		config:   cfg,
		eventBus: bus,
		stopCh:   make(chan struct{}),
	}
}

func (s *Scheduler) Start() {
	slog.Info("WOL 定时调度器已启动")
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.checkAndRunSchedules()
		case <-s.stopCh:
			slog.Info("WOL 定时调度器已停止")
			return
		}
	}
}

func (s *Scheduler) Stop() {
	close(s.stopCh)
}

func (s *Scheduler) checkAndRunSchedules() {
	ctx := context.Background()
	schedules, err := s.store.ListWOLSchedules(ctx)
	if err != nil {
		slog.Warn("查询定时唤醒失败", "error", err)
		return
	}

	now := time.Now()
	for _, schedule := range schedules {
		if !schedule.Enabled {
			continue
		}

		if s.shouldRun(schedule, now) {
			slog.Info("执行定时唤醒", "mac", schedule.MAC, "host", schedule.HostName, "repeat", schedule.RepeatType)
			go s.runSchedule(schedule)
		}
	}
}

func (s *Scheduler) shouldRun(schedule models.WOLSchedule, now time.Time) bool {
	switch schedule.RepeatType {
	case "once":
		if schedule.LastRun != nil {
			return false
		}
		return now.After(schedule.ScheduleAt) || now.Equal(schedule.ScheduleAt)
	case "daily":
		if schedule.LastRun != nil {
			lastRunDate := schedule.LastRun.Format("2006-01-02")
			todayDate := now.Format("2006-01-02")
			if lastRunDate == todayDate {
				return false
			}
		}
		return s.matchesTime(schedule.ScheduleTime, now)
	case "weekday":
		weekday := now.Weekday()
		if weekday == time.Saturday || weekday == time.Sunday {
			return false
		}
		return s.matchesTime(schedule.ScheduleTime, now)
	case "weekly":
		if schedule.LastRun != nil {
			lastRunYear, lastRunWeek := schedule.LastRun.ISOWeek()
			currentYear, currentWeek := now.ISOWeek()
			if lastRunYear == currentYear && lastRunWeek == currentWeek {
				return false
			}
		}
		if now.Weekday() != time.Weekday(schedule.Weekday) {
			return false
		}
		return s.matchesTime(schedule.ScheduleTime, now)
	}
	return false
}

func (s *Scheduler) matchesTime(scheduleTime string, now time.Time) bool {
	if scheduleTime == "" {
		return false
	}
	parts := make([]int, 2)
	for i, p := range splitTime(scheduleTime) {
		if i >= 2 {
			break
		}
		parts[i] = p
	}
	return now.Hour() == parts[0] && now.Minute() == parts[1]
}

func splitTime(t string) []int {
	var result []int
	num := 0
	for _, c := range t {
		if c >= '0' && c <= '9' {
			num = num*10 + int(c-'0')
		} else {
			result = append(result, num)
			num = 0
		}
	}
	result = append(result, num)
	return result
}

func (s *Scheduler) runSchedule(schedule models.WOLSchedule) {
	ctx := context.Background()

	bcast := schedule.CustomBroadcast
	if bcast == "" {
		bcast = s.broadcastForMAC(schedule.MAC)
	}

	mac, err := net.ParseMAC(schedule.MAC)
	if err != nil {
		slog.Warn("无效的 MAC 地址", "mac", schedule.MAC, "error", err)
		return
	}

	if bcast == "" {
		bcast = "255.255.255.255"
	}

	if err := sendWOLPacket(bcast, "", mac); err != nil {
		slog.Warn("发送唤醒包失败", "mac", schedule.MAC, "error", err)
		s.store.CreateWOLHistory(ctx, &models.WOLHistory{
			MAC:       schedule.MAC,
			HostName:  schedule.HostName,
			Broadcast: bcast,
			Success:   false,
			ErrorMsg:  err.Error(),
		})
		return
	}

	now := time.Now()
	s.store.UpdateWOLSchedule(ctx, &models.WOLSchedule{
		ID:          schedule.ID,
		MAC:         schedule.MAC,
		HostName:    schedule.HostName,
		ScheduleAt:  schedule.ScheduleAt,
		CronExpr:    schedule.CronExpr,
		RepeatType:  schedule.RepeatType,
		Weekday:     schedule.Weekday,
		ScheduleTime: schedule.ScheduleTime,
		CustomBroadcast: schedule.CustomBroadcast,
		Enabled:     schedule.Enabled,
		LastRun:     &now,
	})

	s.store.CreateWOLHistory(ctx, &models.WOLHistory{
		MAC:       schedule.MAC,
		HostName:  schedule.HostName,
		Broadcast: bcast,
		Success:   true,
	})

	s.eventBus.Publish("event", models.Event{
		Type:  "WOL",
		Level: models.EventInfo,
		Message: "定时唤醒: " + schedule.HostName + " (" + schedule.MAC + ")",
	})

	slog.Info("定时唤醒完成", "mac", schedule.MAC, "host", schedule.HostName, "broadcast", bcast)
}

func (s *Scheduler) broadcastForMAC(mac string) string {
	host, err := s.store.GetHostByMAC(context.Background(), mac)
	if err != nil || host == nil || host.IP == "" {
		return ""
	}

	ip := net.ParseIP(host.IP)
	if ip == nil {
		return ""
	}

	for _, iface := range s.config.Interfaces {
		for _, sn := range iface.Subnets {
			_, cidr, err := net.ParseCIDR(sn.CIDR)
			if err != nil {
				continue
			}
			if cidr.Contains(ip) {
				bcast := make(net.IP, 4)
				for i := range bcast {
					bcast[i] = ip.To4()[i] | ^cidr.Mask[i]
				}
				return bcast.String()
			}
		}
	}
	return ""
}

func sendWOLPacket(bcastIP, localIP string, mac net.HardwareAddr) error {
	packet := make([]byte, 0, 102)
	packet = append(packet, []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}...)
	for i := 0; i < 16; i++ {
		packet = append(packet, mac...)
	}

	ports := []int{9, 7}
	var lastErr error
	for _, port := range ports {
		addr, err := net.ResolveUDPAddr("udp4", net.JoinHostPort(bcastIP, itoa(port)))
		if err != nil {
			lastErr = err
			continue
		}
		var local *net.UDPAddr
		if localIP != "" {
			local, _ = net.ResolveUDPAddr("udp4", net.JoinHostPort(localIP, "0"))
		}
		conn, err := net.DialUDP("udp4", local, addr)
		if err != nil {
			lastErr = err
			continue
		}
		_, writeErr := conn.Write(packet)
		conn.Close()
		if writeErr != nil {
			lastErr = writeErr
			continue
		}
		lastErr = nil
	}
	return lastErr
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
