package service

// Enterprise Org warnings and alerts (meta-repo docs/enterprise-org-prd.md §4,
// §7.4): the background task that looks at what organization keys have used,
// writes what it finds to the organization's alert list, and tells the people
// who should hear about it.
//
// 🔴 Nothing here is on the path of a request, and nothing here stops one
// (PRD D13, red line 4). The task reads what requests left behind — a key's
// row, the usage log, the monthly request counter — once a minute. It does not
// hang off the settlement of a request, on purpose: a key's spending reaches
// its row a few seconds after the request (BATCH_UPDATE_ENABLED), so a check
// made right at settlement reads the numbers from before it; and settlements
// happen in several places — chat, audio, realtime, video tasks, refunds —
// that a scan of the keys covers without a line in any of them.
//
// What it decides is in internal/org/service (alert_scan.go, alert_digest.go).
// This file holds what needs the platform: the loop, the counter of requests
// per month, the wording, and the sending.

import (
	"context"
	"fmt"
	"html"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/i18n"
	orgmodel "github.com/QuantumNous/new-api/internal/org/model"
	orgservice "github.com/QuantumNous/new-api/internal/org/service"
	tenantquota "github.com/QuantumNous/new-api/internal/quota"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"

	"github.com/bytedance/gopkg/util/gopool"
)

const (
	// notifyTypeOrgAlert is the notification type of an organization's alerts.
	// It is a type of its own because the platform limits how many
	// notifications of one type a user gets: sharing "quota_exceed" would let
	// these and the low-balance reminder crowd each other out.
	notifyTypeOrgAlert = "org_alert"
	// orgAlertTick is how often the task looks at the keys.
	orgAlertTick = time.Minute
	// orgAnomalyEvery is how often it also reads the usage log for the anomaly
	// rules, which is the heavier half.
	orgAnomalyEvery = 10 * time.Minute
	// orgAllowanceOverlap is how far back, in seconds, a pass looks behind the
	// start of the one before it, so that a key written to while that one was
	// reading is not missed. Looking at a key twice costs nothing: a warning
	// is raised once per level and cycle whatever the number of looks.
	orgAllowanceOverlap = 30
	// orgAlertLinesShown caps the alerts one notification spells out.
	orgAlertLinesShown = 20
)

// orgAlertOnce makes StartOrgAlertTask start the task a single time.
var orgAlertOnce sync.Once

// sendOrgAlertNotice delivers a notification to one member. It is NotifyUser;
// a variable so that tests can see who was told what without sending anything.
var sendOrgAlertNotice = NotifyUser

// orgAlertWatch is what the task remembers from one pass to the next.
type orgAlertWatch struct {
	lastPass    int64             // when the last complete look at the keys started; 0 before the first
	lastAnomaly time.Time         // when the usage log was last read for the anomaly rules
	lastDigest  map[int]time.Time // when each organization was last sent a notification
}

// StartOrgAlertTask starts the background task that raises and sends the
// warnings and alerts of every organization. It runs on the master node only,
// like the platform's other periodic tasks, and looks once right away: the
// first look takes in every organization key, which is what catches up on
// anything that happened while the gateway was down.
func StartOrgAlertTask() {
	orgAlertOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}
		gopool.Go(func() {
			watch := &orgAlertWatch{lastDigest: map[int]time.Time{}}
			ticker := time.NewTicker(orgAlertTick)
			defer ticker.Stop()
			watch.pass(time.Now())
			for range ticker.C {
				watch.pass(time.Now())
			}
		})
	})
}

// pass is one round of the task: warnings about what keys have left, every so
// often the anomaly rules, then whatever is waiting to be sent. A step that
// fails is reported and tried again on the next round; it never takes the
// task down.
func (w *orgAlertWatch) pass(now time.Time) {
	defer func() {
		if r := recover(); r != nil {
			common.SysError(fmt.Sprintf("organization alert task: panic: %v", r))
		}
	}()
	since := int64(0)
	if w.lastPass != 0 {
		since = w.lastPass - orgAllowanceOverlap
	}
	if _, err := orgservice.ScanAllowances(model.DB, since, now, orgMonthlyUsage); err != nil {
		common.SysError("organization alert task: failed to check key allowances: " + err.Error())
	} else {
		w.lastPass = now.Unix()
	}
	if now.Sub(w.lastAnomaly) >= orgAnomalyEvery {
		if _, err := orgservice.ScanAnomalies(model.DB, model.LOG_DB, now); err != nil {
			common.SysError("organization alert task: failed to check for anomalies: " + err.Error())
		}
		w.lastAnomaly = now
	}
	w.send(now)
}

// send notifies the members of every organization that has alerts waiting,
// unless that organization was sent a notification too recently — then they
// wait for a later round and leave together.
func (w *orgAlertWatch) send(now time.Time) {
	unsent, err := orgservice.UnsentAlerts(model.DB, now)
	if err != nil {
		common.SysError("organization alert task: failed to read the alerts to send: " + err.Error())
		return
	}
	for orgID, alerts := range unsent {
		if now.Sub(w.lastDigest[orgID]) < orgAlertDigestGap() {
			continue
		}
		digests, err := orgservice.DigestsFor(model.DB, orgID, alerts)
		if err != nil {
			common.SysError(fmt.Sprintf("organization alert task: failed to decide who hears about the alerts of organization %d: %s", orgID, err.Error()))
			continue
		}
		for _, digest := range digests {
			setting := digest.Recipient.GetSetting()
			if err := sendOrgAlertNotice(digest.Recipient.Id, digest.Recipient.Email, setting, orgAlertNotice(setting, digest.Alerts)); err != nil {
				common.SysError(fmt.Sprintf("failed to send organization alerts to user %d: %s", digest.Recipient.Id, err.Error()))
			}
		}
		// Marked sent whether or not every member could be reached: the
		// alerts stay in the organization's list, and trying again every
		// minute would turn one unreachable mailbox into a flood for the rest.
		if err := orgservice.MarkAlertsSent(model.DB, alerts, now); err != nil {
			common.SysError(fmt.Sprintf("organization alert task: failed to mark the alerts of organization %d as sent: %s", orgID, err.Error()))
		}
		w.lastDigest[orgID] = now
	}
}

// orgAlertDigestGap is the least time between two notifications to the same
// organization. It follows the platform's own limit — a user gets
// NotifyLimitCount notifications of one type per NotificationLimitDurationMinute
// — and is set just wide of it, so that the limit never has to drop one of
// these: alerts raised in between wait and leave together.
func orgAlertDigestGap() time.Duration {
	window := time.Duration(constant.NotificationLimitDurationMinute) * time.Minute
	return window/time.Duration(max(constant.NotifyLimitCount, 1)) + time.Minute
}

// orgMonthlyUsage answers how many requests a key has made this calendar
// month, from the counter the gateway's own monthly limit keeps.
func orgMonthlyUsage(keyID int) (int, error) {
	rdb := common.RDB
	if !common.RedisEnabled {
		rdb = nil
	}
	return tenantquota.MonthlyUsed(context.Background(), rdb, keyID)
}

// orgAlertNotice words one notification for one member: a title that says how
// many alerts there are and a line for each, in the language they saved — as
// plain text for the channels that cannot show HTML. A notification that
// reports an anomaly closes by saying that nothing was blocked.
func orgAlertNotice(setting dto.UserSetting, alerts []orgservice.AlertNotice) dto.Notify {
	plain := setting.NotifyType == dto.NotifyTypeBark || setting.NotifyType == dto.NotifyTypeGotify
	separator := "<br/>"
	if plain {
		separator = "\n"
	}
	lang := setting.Language
	lines := make([]string, 0, len(alerts)+2)
	reportsAnomaly := false
	for i, alert := range alerts {
		reportsAnomaly = reportsAnomaly || slices.Contains(orgmodel.AnomalyRules, alert.Rule)
		switch {
		case i < orgAlertLinesShown:
			lines = append(lines, orgAlertLine(lang, alert, plain))
		case i == orgAlertLinesShown:
			lines = append(lines, i18n.Translate(lang, i18n.MsgOrgAlertMore, map[string]any{"Count": len(alerts) - i}))
		}
	}
	if reportsAnomaly {
		lines = append(lines, i18n.Translate(lang, i18n.MsgOrgAlertFooter))
	}
	title := i18n.Translate(lang, i18n.MsgOrgAlertTitle, map[string]any{"Count": len(alerts)})
	return dto.NewNotify(notifyTypeOrgAlert, title, strings.Join(lines, separator), nil)
}

// orgAlertLine words one alert. The key's name, its holder's name and the
// addresses are what people typed or what a client sent, so they are escaped
// wherever the line is going to be read as HTML.
func orgAlertLine(lang string, alert orgservice.AlertNotice, plain bool) string {
	text := func(raw string) string {
		if plain {
			return raw
		}
		return html.EscapeString(raw)
	}
	detail := alert.Detail
	args := map[string]any{"Key": text(detail.Key), "Holder": text(alert.Holder)}
	spent := detail.Limit > 0 && detail.Used >= detail.Limit
	if detail.Limit > 0 {
		args["Percent"] = int(int64(detail.Used) * 100 / int64(detail.Limit))
	}
	message := ""
	switch alert.Rule {
	case orgmodel.AlertRuleQuota:
		args["Used"], args["Limit"] = logger.FormatQuota(detail.Used), logger.FormatQuota(detail.Limit)
		message = i18n.MsgOrgAlertQuota
		if spent {
			message = i18n.MsgOrgAlertQuotaSpent
		}
	case orgmodel.AlertRuleMonthly:
		args["Used"], args["Limit"] = detail.Used, detail.Limit
		message = i18n.MsgOrgAlertMonthly
		if spent {
			message = i18n.MsgOrgAlertMonthlySpent
		}
	case orgmodel.AlertRuleSpike, orgmodel.AlertRuleOffHours:
		args["Spent"], args["Average"] = logger.FormatQuota(detail.Spent), logger.FormatQuota(detail.DailyAverage)
		message = i18n.MsgOrgAlertSpike
		if alert.Rule == orgmodel.AlertRuleOffHours {
			message = i18n.MsgOrgAlertOffHours
		}
	case orgmodel.AlertRuleNewIP:
		args["Ips"] = text(strings.Join(detail.Ips, ", "))
		message = i18n.MsgOrgAlertNewIP
	default:
		// A rule this build does not know how to word: name the key at least.
		return fmt.Sprintf("%s: %s", alert.Rule, text(detail.Key))
	}
	return i18n.Translate(lang, message, args)
}
