package keypool

import (
	"fmt"
	"strings"
	"time"

	"api-load/internal/failover"
	"api-load/internal/models"

	"github.com/sirupsen/logrus"
)

// DefaultAutoRestoreStatusCodes 是分组开启自动恢复但未显式收窄时的拉黑原因过滤,
// 绝大多数渠道的日配额用尽表现为 429。
const DefaultAutoRestoreStatusCodes = "429"

// groupAutoRestoreSchedule 返回分组的 auto_restore_schedule 原始值,空串表示未开启。
func groupAutoRestoreSchedule(group *models.Group) string {
	if group == nil || group.Config == nil {
		return ""
	}
	raw, _ := group.Config["auto_restore_schedule"].(string)
	return strings.TrimSpace(raw)
}

// groupAutoRestoreStatusCodes 返回自动恢复生效的状态码 spec,未配置时默认只匹配 429。
func groupAutoRestoreStatusCodes(group *models.Group) string {
	if group == nil || group.Config == nil {
		return DefaultAutoRestoreStatusCodes
	}
	if raw, ok := group.Config["auto_restore_status_codes"].(string); ok && strings.TrimSpace(raw) != "" {
		return strings.TrimSpace(raw)
	}
	return DefaultAutoRestoreStatusCodes
}

// autoRestoreCooldownUntil 计算此刻被拉黑的 key 的自动恢复时间点;
// 分组未开启、状态码不命中或调度解析失败时返回 nil(保持既有的验证恢复路径)。
func autoRestoreCooldownUntil(group *models.Group, statusCode int, now time.Time) *time.Time {
	schedule := groupAutoRestoreSchedule(group)
	if schedule == "" {
		return nil
	}
	matcher, err := failover.ParseStatusCodeMatcher(groupAutoRestoreStatusCodes(group))
	if err != nil {
		logrus.WithFields(logrus.Fields{"groupID": group.ID, "error": err}).Warn("Invalid auto_restore_status_codes, skipping auto-restore cooldown")
		return nil
	}
	if !matcher.Match(statusCode) {
		return nil
	}
	until, err := NextAutoRestoreTime(schedule, now)
	if err != nil {
		logrus.WithFields(logrus.Fields{"groupID": group.ID, "error": err}).Warn("Invalid auto_restore_schedule, skipping auto-restore cooldown")
		return nil
	}
	return &until
}

// NextAutoRestoreTime 把调度配置解析成 from 之后的下一个恢复时间点。
// 支持两种语法:
//   - Go duration("24h"、"90m"):从拉黑时刻起的滚动窗口
//   - 每日固定时刻("00:05 +08:00"、"23:30 -07:00"、"04:00"):
//     该钟点的下一次出现;不带时区偏移时按服务器本地时区
func NextAutoRestoreTime(schedule string, from time.Time) (time.Time, error) {
	schedule = strings.TrimSpace(schedule)
	if schedule == "" {
		return time.Time{}, fmt.Errorf("empty auto-restore schedule")
	}
	if d, err := time.ParseDuration(schedule); err == nil {
		if d <= 0 {
			return time.Time{}, fmt.Errorf("auto-restore duration must be positive: %q", schedule)
		}
		return from.Add(d), nil
	}
	if t, err := time.Parse("15:04 -07:00", schedule); err == nil {
		return nextDailyOccurrence(t.Hour(), t.Minute(), t.Location(), from), nil
	}
	if t, err := time.Parse("15:04", schedule); err == nil {
		return nextDailyOccurrence(t.Hour(), t.Minute(), time.Local, from), nil
	}
	return time.Time{}, fmt.Errorf("invalid auto-restore schedule %q: use a duration like \"24h\" or a daily time like \"00:05 +08:00\"", schedule)
}

// ValidateAutoRestoreSchedule 供分组配置保存时校验调度语法。
func ValidateAutoRestoreSchedule(schedule string) error {
	_, err := NextAutoRestoreTime(schedule, time.Now())
	return err
}

func nextDailyOccurrence(hour, minute int, loc *time.Location, from time.Time) time.Time {
	local := from.In(loc)
	next := time.Date(local.Year(), local.Month(), local.Day(), hour, minute, 0, 0, loc)
	if !next.After(from) {
		next = next.AddDate(0, 0, 1)
	}
	return next
}
