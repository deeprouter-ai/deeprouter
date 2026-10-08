package service

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	orgmodel "github.com/QuantumNous/new-api/internal/org/model"
	platformmodel "github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Enterprise Org P8 (meta-repo docs/enterprise-org-prd.md §4): who is told
// about an alert, who may read the alert list and how far, who may mark an
// alert as dealt with, and the settings warnings and alerts run on.

// seedAlert writes an alert about a key straight into the table, as a scan
// would have raised it, and returns it.
func seedAlert(t *testing.T, db *gorm.DB, key platformmodel.Token, rule string, level int) orgmodel.OrgAlert {
	t.Helper()
	holder := reloadUser(t, db, key.UserId)
	alert := orgmodel.OrgAlert{
		OrgId: key.OrgId, Rule: rule, TokenId: key.Id, UserId: key.UserId, DepartmentId: holder.DepartmentId,
		Level: level, Detail: alertDetailJSON(orgmodel.AlertDetail{Key: key.Name, Used: 800, Limit: 1000}),
		CreatedTime: scanNow.Unix(),
	}
	require.NoError(t, db.Create(&alert).Error)
	return alert
}

// reachable gives a member an address a notification can find them at.
func reachable(t *testing.T, db *gorm.DB, userID int, email string) {
	t.Helper()
	require.NoError(t, db.Model(&platformmodel.User{}).Where("id = ?", userID).Update("email", email).Error)
}

// recipientsOf maps who a set of digests is for to what each is told about.
func recipientsOf(digests []AlertDigest) map[int][]string {
	told := map[int][]string{}
	for _, digest := range digests {
		for _, alert := range digest.Alerts {
			told[digest.Recipient.Id] = append(told[digest.Recipient.Id], alert.Rule+" on "+alert.Detail.Key)
		}
	}
	return told
}

// --- who is told ---------------------------------------------------------------

// Acceptance: warnings go to the admins and the key's holder; anomaly alerts go
// to the admins. "The admins" are the owner and every admin; nobody else hears,
// and a service account has nowhere to be told.
func TestDigestsFor_TheAdminsHearEverythingAndAHolderTheirOwnWarnings(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		c := newCast(t, db, "Acme")
		org := c.org
		staff, manager, bot := c.actors[kindStaff].UserId, c.actors[kindManager].UserId, c.actors[kindBot].UserId
		colleague := seedMemberIn(t, db, org, "colleague", presetRoleID(t, db, orgmodel.RoleStaff), c.sales.Id)
		for id, email := range map[int]string{
			org.owner.Id: "owner@acme.test", c.actors[kindAdmin].UserId: "admin@acme.test",
			staff: "staff@acme.test", manager: "manager@acme.test", colleague.Id: "colleague@acme.test",
		} {
			reachable(t, db, id, email)
		}
		staffKey := seedKey(t, db, org, staff, "Staff key")
		colleagueKey := seedKey(t, db, org, colleague.Id, "Colleague key")
		botKey := seedKey(t, db, org, bot, "CI key")
		parkedKey := seedKey(t, db, org, org.owner.Id, "Parked key")
		alerts := []orgmodel.OrgAlert{
			seedAlert(t, db, staffKey, orgmodel.AlertRuleQuota, 80),
			seedAlert(t, db, staffKey, orgmodel.AlertRuleSpike, 0),
			seedAlert(t, db, colleagueKey, orgmodel.AlertRuleMonthly, 100),
			seedAlert(t, db, colleagueKey, orgmodel.AlertRuleNewIP, 0),
			seedAlert(t, db, botKey, orgmodel.AlertRuleQuota, 100),
			seedAlert(t, db, parkedKey, orgmodel.AlertRuleQuota, 80),
		}

		digests, err := DigestsFor(db, org.id, alerts)
		require.NoError(t, err)
		everything := []string{
			"quota on Staff key", "spike on Staff key", "monthly on Colleague key",
			"new_ip on Colleague key", "quota on CI key", "quota on Parked key",
		}
		assert.Equal(t, map[int][]string{
			org.owner.Id:               everything,
			c.actors[kindAdmin].UserId: everything,
			staff:                      {"quota on Staff key"},
			colleague.Id:               {"monthly on Colleague key"},
		}, recipientsOf(digests), "the manager, the read-only member, the role packs and the service account hear nothing")
		require.Len(t, digests, 4, "one digest each, and none that says nothing")

		// Each digest carries what a notification needs, and names the holder.
		for _, digest := range digests {
			assert.NotEmpty(t, digest.Recipient.Email)
		}
		assert.Equal(t, 80, digests[0].Alerts[0].Level)
		assert.Equal(t, "staff-of-Acme", digests[0].Alerts[0].Holder)
		assert.Equal(t, "CI", digests[0].Alerts[4].Holder, "a service account is called what the company named it")
		assert.Equal(t, orgmodel.AlertDetail{Key: "Staff key", Used: 800, Limit: 1000}, digests[0].Alerts[0].Detail)
	})
}

// A holder who can no longer be told is skipped, and so is an admin whose
// account is disabled: the others still hear.
func TestDigestsFor_SkipsWhoeverCannotBeTold(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		c := newCast(t, db, "Acme")
		org := c.org
		disabled := seedMemberIn(t, db, org, "disabled", presetRoleID(t, db, orgmodel.RoleStaff), c.sales.Id)
		removed := seedMemberIn(t, db, org, "removed", presetRoleID(t, db, orgmodel.RoleStaff), c.sales.Id)
		stranger := seedMember(t, db, seedOrg(t, db, "Other"), "stranger", orgmodel.RoleStaff)
		alerts := []orgmodel.OrgAlert{
			seedAlert(t, db, seedKey(t, db, org, disabled.Id, "Disabled's key"), orgmodel.AlertRuleQuota, 80),
			seedAlert(t, db, seedKey(t, db, org, removed.Id, "Removed's key"), orgmodel.AlertRuleQuota, 80),
		}
		// An alert that names somebody from another company as the holder —
		// which nothing should ever write — still tells them nothing.
		foreign := seedAlert(t, db, seedKey(t, db, org, org.owner.Id, "Foreign"), orgmodel.AlertRuleQuota, 80)
		foreign.UserId = stranger.Id
		alerts = append(alerts, foreign)
		require.NoError(t, db.Model(&platformmodel.User{}).Where("id IN ?", []int{disabled.Id, c.actors[kindAdmin].UserId}).
			Update("status", common.UserStatusDisabled).Error)
		require.NoError(t, db.Delete(&platformmodel.User{}, removed.Id).Error)

		digests, err := DigestsFor(db, org.id, alerts)
		require.NoError(t, err)
		require.Len(t, digests, 1)
		assert.Equal(t, org.owner.Id, digests[0].Recipient.Id)
		assert.Len(t, digests[0].Alerts, 3)
		assert.Equal(t, "removed", digests[0].Alerts[1].Holder, "the owner is still told whose key it was")

		digests, err = DigestsFor(db, org.id, nil)
		require.NoError(t, err)
		assert.Empty(t, digests, "nothing to tell, nobody to tell it to")
	})
}

// Alerts wait until they have gone out, and go out once.
func TestUnsentAlerts_WaitUntilTheyAreMarkedSent(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		acme, _, acmeKey := keyInSales(t, db, "Acme")
		globex, _, globexKey := keyInSales(t, db, "Globex")
		first := seedAlert(t, db, acmeKey, orgmodel.AlertRuleQuota, 80)
		second := seedAlert(t, db, acmeKey, orgmodel.AlertRuleSpike, 0)
		third := seedAlert(t, db, globexKey, orgmodel.AlertRuleNewIP, 0)
		stale := seedAlert(t, db, globexKey, orgmodel.AlertRuleSpike, 0)
		require.NoError(t, db.Model(&orgmodel.OrgAlert{}).Where("id = ?", stale.Id).
			Update("created_time", scanNow.Add(-alertSendWindow-time.Second).Unix()).Error)

		unsent, err := UnsentAlerts(db, scanNow)
		require.NoError(t, err)
		require.Len(t, unsent, 2)
		require.Len(t, unsent[acme.id], 2)
		assert.Equal(t, []int{first.Id, second.Id}, []int{unsent[acme.id][0].Id, unsent[acme.id][1].Id}, "oldest first")
		require.Len(t, unsent[globex.id], 1, "an alert more than a day old is no longer worth sending")
		assert.Equal(t, third.Id, unsent[globex.id][0].Id)

		require.NoError(t, MarkAlertsSent(db, unsent[acme.id], scanNow))
		unsent, err = UnsentAlerts(db, scanNow.Add(time.Minute))
		require.NoError(t, err)
		require.Len(t, unsent, 1)
		assert.Contains(t, unsent, globex.id)
		var sent orgmodel.OrgAlert
		require.NoError(t, db.First(&sent, first.Id).Error)
		assert.Equal(t, scanNow.Unix(), sent.NotifiedTime)
		assert.Equal(t, orgmodel.AlertStateOpen, sent.State, "sending an alert does not deal with it")
	})
}

// --- reading the list -------------------------------------------------------

// alert.read reaches as far as the role does: a role limited to departments is
// sent the alerts raised on keys held there, cut on the server.
func TestListAlerts_IsCutToTheActorsReach(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		c := newCast(t, db, "Acme")
		org := c.org
		inSales := seedMemberIn(t, db, org, "in-sales", presetRoleID(t, db, orgmodel.RoleStaff), c.sales.Id)
		inProduct := seedMemberIn(t, db, org, "in-product", presetRoleID(t, db, orgmodel.RoleStaff), c.product.Id)
		salesAlert := seedAlert(t, db, seedKey(t, db, org, inSales.Id, "Sales key"), orgmodel.AlertRuleQuota, 80)
		productAlert := seedAlert(t, db, seedKey(t, db, org, inProduct.Id, "Product key"), orgmodel.AlertRuleSpike, 0)
		parkedAlert := seedAlert(t, db, seedKey(t, db, org, org.owner.Id, "Parked key"), orgmodel.AlertRuleNewIP, 0)
		// Another company's alert, which nobody here may ever see.
		other := seedOrg(t, db, "Other")
		seedAlert(t, db, seedKey(t, db, other, other.owner.Id, "Their key"), orgmodel.AlertRuleSpike, 0)

		idsFor := func(kind string) []int {
			alerts, total, err := ListAlerts(db, c.actors[kind], false, 0, 50)
			require.NoError(t, err, kind)
			require.Len(t, alerts, int(total), kind)
			ids := make([]int, 0, len(alerts))
			for _, alert := range alerts {
				ids = append(ids, alert.Id)
			}
			return ids
		}
		everything := []int{parkedAlert.Id, productAlert.Id, salesAlert.Id} // newest first
		for _, kind := range []string{kindOwner, kindAdmin, kindReadonly, kindITOps} {
			assert.Equal(t, everything, idsFor(kind), kind)
		}
		assert.Equal(t, []int{salesAlert.Id}, idsFor(kindManager), "the manager of Sales sees what was raised in Sales")
		for _, kind := range []string{kindStaff, kindBot, kindHROps, kindFinance, kindKeyDesk} {
			_, _, err := ListAlerts(db, c.actors[kind], false, 0, 50)
			assert.ErrorIs(t, err, ErrForbidden, kind)
		}

		// An alert stays with the department it was raised in: moving the
		// holder afterwards does not hand it to another manager.
		require.NoError(t, UpdateMember(db, c.actors[kindOwner], inProduct.Id, MemberPatch{DepartmentId: intPtr(c.sales.Id)}))
		assert.Equal(t, []int{salesAlert.Id}, idsFor(kindManager))
	})
}

// What a row of the list says, the filter on open alerts, and paging.
func TestListAlerts_NamesTheHolderAndPagesNewestFirst(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		org, staff, key := keyInSales(t, db, "Acme")
		sales := departmentNamed(t, db, org.id, "Sales")
		owner := actorFor(t, db, org.owner.Id)
		require.NoError(t, db.Model(&platformmodel.User{}).Where("id = ?", staff.Id).Update("display_name", "Ada").Error)
		first := seedAlert(t, db, key, orgmodel.AlertRuleQuota, 80)
		second := seedAlert(t, db, key, orgmodel.AlertRuleSpike, 0)
		third := seedAlert(t, db, key, orgmodel.AlertRuleNewIP, 0)
		require.NoError(t, HandleAlert(db, owner, second.Id, orgmodel.AlertStateFalseAlarm))

		alerts, total, err := ListAlerts(db, owner, false, 0, 2)
		require.NoError(t, err)
		assert.EqualValues(t, 3, total)
		require.Len(t, alerts, 2)
		assert.Equal(t, []int{third.Id, second.Id}, []int{alerts[0].Id, alerts[1].Id})
		alerts, _, err = ListAlerts(db, owner, false, 2, 2)
		require.NoError(t, err)
		require.Len(t, alerts, 1)
		row := alerts[0]
		assert.Equal(t, first.Id, row.Id)
		assert.Equal(t, orgmodel.AlertRuleQuota, row.Rule)
		assert.Equal(t, 80, row.Level)
		assert.Equal(t, key.Id, row.KeyId)
		assert.Equal(t, staff.Id, row.HolderId)
		assert.Equal(t, "Ada", row.Holder)
		assert.Equal(t, sales.Id, row.DepartmentId)
		assert.Equal(t, "Sales", row.Department)
		assert.JSONEq(t, `{"key":"Design tools","used":800,"limit":1000}`, string(row.Detail))
		assert.Equal(t, scanNow.Unix(), row.CreatedTime)
		assert.Equal(t, orgmodel.AlertStateOpen, row.State)
		assert.Zero(t, row.AckedBy)
		assert.Empty(t, row.AckedByName)

		open, total, err := ListAlerts(db, owner, true, 0, 50)
		require.NoError(t, err)
		assert.EqualValues(t, 2, total)
		require.Len(t, open, 2)
		assert.Equal(t, []int{third.Id, first.Id}, []int{open[0].Id, open[1].Id}, "the one that was dealt with is left out")

		// The alert goes on naming its department and its holder after both
		// are gone.
		require.NoError(t, DeleteDepartment(db, owner, sales.Id))
		_, err = RemoveMember(db, owner, staff.Id)
		require.NoError(t, err)
		alerts, _, err = ListAlerts(db, owner, false, 2, 2)
		require.NoError(t, err)
		assert.Equal(t, "Sales", alerts[0].Department)
		assert.Equal(t, "Ada", alerts[0].Holder)
	})
}

// --- dealing with an alert ------------------------------------------------------

// Marking an alert is the owner's and the admins' (PRD §2), is recorded, and
// says who did it and when.
func TestHandleAlert_MarksItAndRecordsWhoDid(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		c := newCast(t, db, "Acme")
		org := c.org
		admin := c.actors[kindAdmin]
		alert := seedAlert(t, db, seedKey(t, db, org, c.actors[kindStaff].UserId, "Design tools"), orgmodel.AlertRuleSpike, 0)
		reload := func() orgmodel.OrgAlert {
			var row orgmodel.OrgAlert
			require.NoError(t, db.First(&row, alert.Id).Error)
			return row
		}
		before := time.Now().Unix()

		require.NoError(t, HandleAlert(db, admin, alert.Id, orgmodel.AlertStateHandled))
		row := reload()
		assert.Equal(t, orgmodel.AlertStateHandled, row.State)
		assert.Equal(t, admin.UserId, row.AckedBy)
		assert.GreaterOrEqual(t, row.AckedTime, before)
		record := lastAudit(t, db, org.id)
		assert.Equal(t, orgmodel.AuditAlertHandle, record.Action)
		assert.Equal(t, orgmodel.AuditTargetAlert, record.TargetType)
		assert.Equal(t, alert.Id, record.TargetId)
		assert.Equal(t, admin.UserId, record.ActorUserId)
		assert.Equal(t, testIP, record.Ip)
		assert.JSONEq(t, `{"before":{"rule":"spike","key":"Design tools","state":""},"after":{"rule":"spike","key":"Design tools","state":"handled"}}`, record.Detail)

		// The list says who dealt with it.
		alerts, _, err := ListAlerts(db, c.actors[kindReadonly], false, 0, 10)
		require.NoError(t, err)
		require.Len(t, alerts, 1)
		assert.Equal(t, admin.UserId, alerts[0].AckedBy)
		assert.Equal(t, "admin-of-Acme", alerts[0].AckedByName)
		assert.Equal(t, row.AckedTime, alerts[0].AckedTime)

		// Marking it what it already is changes nothing and records nothing.
		records := len(auditRecords(t, db, org.id))
		require.NoError(t, HandleAlert(db, c.actors[kindOwner], alert.Id, orgmodel.AlertStateHandled))
		assert.Equal(t, row, reload())
		assert.Len(t, auditRecords(t, db, org.id), records)

		// Somebody else may think again.
		require.NoError(t, HandleAlert(db, c.actors[kindOwner], alert.Id, orgmodel.AlertStateFalseAlarm))
		assert.Equal(t, orgmodel.AlertStateFalseAlarm, reload().State)
		assert.Equal(t, org.owner.Id, reload().AckedBy)
		assert.Len(t, auditRecords(t, db, org.id), records+1)
	})
}

// What marking an alert refuses: a state that is not one of the two, an alert
// that is not there, and one that belongs to another company.
func TestHandleAlert_Refusals(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		org, _, key := keyInSales(t, db, "Acme")
		owner := actorFor(t, db, org.owner.Id)
		alert := seedAlert(t, db, key, orgmodel.AlertRuleSpike, 0)
		other := seedOrg(t, db, "Other")
		theirs := seedAlert(t, db, seedKey(t, db, other, other.owner.Id, "Their key"), orgmodel.AlertRuleSpike, 0)

		for _, state := range []string{"", "open", "Handled", "ignored"} {
			assert.ErrorIs(t, HandleAlert(db, owner, alert.Id, state), ErrInvalidAlertState, "state %q", state)
		}
		assert.ErrorIs(t, HandleAlert(db, owner, 424242, orgmodel.AlertStateHandled), ErrAlertNotFound)
		assert.ErrorIs(t, HandleAlert(db, owner, theirs.Id, orgmodel.AlertStateHandled), ErrAlertNotFound,
			"another company's alert reads as not there")

		var untouched []orgmodel.OrgAlert
		require.NoError(t, db.Order("id").Find(&untouched).Error)
		for _, row := range untouched {
			assert.Equal(t, orgmodel.AlertStateOpen, row.State)
			assert.Zero(t, row.AckedBy)
		}
		for _, record := range append(auditRecords(t, db, org.id), auditRecords(t, db, other.id)...) {
			assert.NotEqual(t, orgmodel.AuditAlertHandle, record.Action)
		}
	})
}

// --- the settings -----------------------------------------------------------------

// Acceptance: …（默认 80% 与 100% 两档，可配）. An organization starts on the
// defaults, an owner or admin changes them, and the change is recorded with
// what it was and what it became.
func TestAlertSettings_StartOnTheDefaultsAndAreTheOrganizationsOwn(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		acme := seedOrg(t, db, "Acme")
		globex := seedOrg(t, db, "Globex")
		owner := actorFor(t, db, acme.owner.Id)

		settings, err := GetAlertSettings(db, owner)
		require.NoError(t, err)
		assert.Equal(t, orgmodel.DefaultAlertSettings(), settings)

		wanted := orgmodel.AlertSettings{
			WarnAt: []int{90, 50, 90}, SpikeMultiple: 3, OffHoursPercent: 25, MinSpend: 2 * dollar,
			WorkHours: &orgmodel.WorkHours{Timezone: "Asia/Shanghai", Days: []int{5, 4, 3, 2, 1}, Start: 9 * 60, End: 18 * 60},
		}
		saved, err := UpdateAlertSettings(db, owner, wanted)
		require.NoError(t, err)
		stored := orgmodel.AlertSettings{
			WarnAt: []int{50, 90}, SpikeMultiple: 3, OffHoursPercent: 25, MinSpend: 2 * dollar,
			WorkHours: &orgmodel.WorkHours{Timezone: "Asia/Shanghai", Days: []int{1, 2, 3, 4, 5}, Start: 9 * 60, End: 18 * 60},
		}
		assert.Equal(t, stored, saved, "answered in the form it is stored in")
		settings, err = GetAlertSettings(db, owner)
		require.NoError(t, err)
		assert.Equal(t, stored, settings)

		record := lastAudit(t, db, acme.id)
		assert.Equal(t, orgmodel.AuditSettingsUpdate, record.Action)
		assert.Equal(t, orgmodel.AuditTargetOrganization, record.TargetType)
		assert.Equal(t, acme.id, record.TargetId)
		assert.Equal(t, testIP, record.Ip)
		assert.JSONEq(t, `{
			"before": {"warn_at":[80,100],"spike_multiple":5,"off_hours_percent":50,"min_spend":500000,"work_hours":null},
			"after":  {"warn_at":[50,90],"spike_multiple":3,"off_hours_percent":25,"min_spend":1000000,
			           "work_hours":{"timezone":"Asia/Shanghai","days":[1,2,3,4,5],"start":540,"end":1080}}
		}`, record.Detail)

		// Saving the same again changes nothing and records nothing.
		records := len(auditRecords(t, db, acme.id))
		_, err = UpdateAlertSettings(db, owner, stored)
		require.NoError(t, err)
		assert.Len(t, auditRecords(t, db, acme.id), records)

		// Taking the working hours away again switches the off-hours rule off.
		stored.WorkHours = nil
		saved, err = UpdateAlertSettings(db, owner, stored)
		require.NoError(t, err)
		assert.Nil(t, saved.WorkHours)

		// The company next door is still on the defaults.
		theirs, err := GetAlertSettings(db, actorFor(t, db, globex.owner.Id))
		require.NoError(t, err)
		assert.Equal(t, orgmodel.DefaultAlertSettings(), theirs)
		assert.Empty(t, auditRecords(t, db, globex.id))
	})
}

// Settings that are out of range are refused whole: nothing is saved and
// nothing is recorded.
func TestAlertSettings_OutOfRangeIsRefusedWhole(t *testing.T) {
	forEachDialect(t, func(t *testing.T, db *gorm.DB) {
		org := seedOrg(t, db, "Acme")
		owner := actorFor(t, db, org.owner.Id)
		bad := orgmodel.DefaultAlertSettings()
		bad.WarnAt = []int{50}
		bad.SpikeMultiple = 1
		_, err := UpdateAlertSettings(db, owner, bad)
		require.ErrorIs(t, err, ErrInvalidAlertSettings)
		_, err = UpdateAlertSettings(db, owner, orgmodel.AlertSettings{})
		require.ErrorIs(t, err, ErrInvalidAlertSettings, "an empty body is not a setting")

		settings, err := GetAlertSettings(db, owner)
		require.NoError(t, err)
		assert.Equal(t, orgmodel.DefaultAlertSettings(), settings)
		assert.Empty(t, auditRecords(t, db, org.id))
	})
}
