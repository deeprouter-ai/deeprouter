package service

import (
	"slices"
	"time"

	"github.com/QuantumNous/new-api/common"
	orgmodel "github.com/QuantumNous/new-api/internal/org/model"
	platformmodel "github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

// Who is told about an alert (PRD §4). The owner and the admins hear about
// every one. A warning — the key is running out of what it was given — also
// goes to whoever holds that key, because they are the one it will stop for.
// An anomaly does not: if the key leaked, its holder is not who to ask.
//
// Alerts wait in org_alerts with notified_time 0 until they have gone out, so
// one that was raised just before a restart is still sent after it, and
// several raised close together can leave as one notification.

// alertSendWindow is how long an alert that has not gone out is still worth
// sending. Past it the alert stays in the list and nobody is notified: a
// reminder about a day that is over helps no one.
const alertSendWindow = 24 * time.Hour

// AlertNotice is one alert as a notification words it.
type AlertNotice struct {
	Rule   string
	Level  int
	Holder string // who held the key when the alert was raised, as they are called today
	Detail orgmodel.AlertDetail
}

// AlertDigest is everything one member is to be told in one notification.
type AlertDigest struct {
	// Recipient carries what a notification needs: id, email and settings.
	Recipient platformmodel.User
	Alerts    []AlertNotice
	// SeesList says the recipient can open the organization's alert list: the
	// owner and the admins. A holder told about their own key may well not.
	SeesList bool
}

// UnsentAlerts returns the alerts that have not gone out as a notification
// yet, by organization and oldest first.
func UnsentAlerts(db *gorm.DB, now time.Time) (map[int][]orgmodel.OrgAlert, error) {
	var alerts []orgmodel.OrgAlert
	if err := db.Where("notified_time = ? AND created_time >= ?", 0, now.Add(-alertSendWindow).Unix()).
		Order("id").Find(&alerts).Error; err != nil {
		return nil, err
	}
	unsent := map[int][]orgmodel.OrgAlert{}
	for _, alert := range alerts {
		unsent[alert.OrgId] = append(unsent[alert.OrgId], alert)
	}
	return unsent, nil
}

// MarkAlertsSent records that alerts have gone out, so they are not sent again.
func MarkAlertsSent(db *gorm.DB, alerts []orgmodel.OrgAlert, now time.Time) error {
	ids := make([]int, 0, len(alerts))
	for _, alert := range alerts {
		ids = append(ids, alert.Id)
	}
	for chunk := range slices.Chunk(ids, scanChunk) {
		if err := db.Model(&orgmodel.OrgAlert{}).Where("id IN ?", chunk).
			Update("notified_time", now.Unix()).Error; err != nil {
			return err
		}
	}
	return nil
}

// DigestsFor decides who is told about an organization's alerts, and returns
// one digest per member with something to hear: the owner and the admins get
// all of them, and a key's holder the warnings about that key. A service
// account is told nothing — it has nowhere to be told — and neither is a
// holder whose account is disabled or gone.
func DigestsFor(db *gorm.DB, orgID int, alerts []orgmodel.OrgAlert) ([]AlertDigest, error) {
	if len(alerts) == 0 {
		return nil, nil
	}
	ownerID, err := ownerUserID(db, orgID)
	if err != nil {
		return nil, err
	}
	watchers, err := WalletWatchers(db, orgID, ownerID)
	if err != nil {
		return nil, err
	}
	everyHolder := make([]int, 0, len(alerts))
	for _, alert := range alerts {
		everyHolder = append(everyHolder, alert.UserId)
	}
	names, err := userNames(db, everyHolder)
	if err != nil {
		return nil, err
	}
	notice := func(alert *orgmodel.OrgAlert) AlertNotice {
		return AlertNotice{Rule: alert.Rule, Level: alert.Level, Holder: names[alert.UserId], Detail: alertDetail(alert)}
	}
	watching := make(map[int]bool, len(watchers))
	for _, watcher := range watchers {
		watching[watcher.Id] = true
	}

	// What each holder is to hear besides: the warnings on their own keys.
	// A holder who is the owner or an admin hears them with everything else.
	everything := make([]AlertNotice, 0, len(alerts))
	warnings := map[int][]AlertNotice{}
	var warned []int
	for i := range alerts {
		alert := &alerts[i]
		everything = append(everything, notice(alert))
		if slices.Contains(orgmodel.WarningRules, alert.Rule) && !watching[alert.UserId] {
			if warnings[alert.UserId] == nil {
				warned = append(warned, alert.UserId)
			}
			warnings[alert.UserId] = append(warnings[alert.UserId], notice(alert))
		}
	}
	var holders []platformmodel.User
	if len(warned) > 0 {
		if err := db.Select("id", "email", "setting").
			Where("org_id = ? AND id IN ? AND is_service = ? AND status = ?", orgID, warned, false, common.UserStatusEnabled).
			Order("id").Find(&holders).Error; err != nil {
			return nil, err
		}
	}

	digests := make([]AlertDigest, 0, len(watchers)+len(holders))
	for _, watcher := range watchers {
		digests = append(digests, AlertDigest{Recipient: watcher, Alerts: everything, SeesList: true})
	}
	for _, holder := range holders {
		digests = append(digests, AlertDigest{Recipient: holder, Alerts: warnings[holder.Id]})
	}
	return digests, nil
}
