package service

import (
	"errors"
	"reflect"

	"github.com/QuantumNous/new-api/common"
	orgmodel "github.com/QuantumNous/new-api/internal/org/model"
	"gorm.io/gorm"
)

// ErrInvalidAlertSettings means a warning level, a rule's number or the
// working hours are out of range.
var ErrInvalidAlertSettings = errors.New("alert settings are out of range")

// alertSettingsOf loads the settings an organization's warnings and alerts
// run on: the defaults, until an owner or admin changed something.
func alertSettingsOf(db *gorm.DB, orgID int) (orgmodel.AlertSettings, error) {
	var found []orgmodel.Organization
	if err := db.Select("id", "alert_settings").Where("id = ?", orgID).Limit(1).Find(&found).Error; err != nil {
		return orgmodel.AlertSettings{}, err
	}
	if len(found) == 0 {
		return orgmodel.DefaultAlertSettings(), nil
	}
	return orgmodel.ParseAlertSettings(found[0].AlertSettings), nil
}

// GetAlertSettings returns the organization's warning levels, alert rules and
// working hours. Reading them takes the power that changes them: they are the
// form an owner or admin fills in.
func GetAlertSettings(db *gorm.DB, actor *Actor) (orgmodel.AlertSettings, error) {
	if err := actor.allow(orgmodel.PowerSettings, wholeOrganization); err != nil {
		return orgmodel.AlertSettings{}, err
	}
	return alertSettingsOf(db, actor.OrgId)
}

// UpdateAlertSettings replaces the organization's warning levels, alert rules
// and working hours, and returns them as stored. Saving what is already there
// changes nothing and records nothing.
func UpdateAlertSettings(db *gorm.DB, actor *Actor, input orgmodel.AlertSettings) (orgmodel.AlertSettings, error) {
	permit, err := actor.permit(orgmodel.PowerSettings, wholeOrganization)
	if err != nil {
		return orgmodel.AlertSettings{}, err
	}
	settings, ok := input.Normalized()
	if !ok {
		return orgmodel.AlertSettings{}, ErrInvalidAlertSettings
	}
	err = db.Transaction(func(tx *gorm.DB) error {
		before, err := alertSettingsOf(tx, actor.OrgId)
		if err != nil {
			return err
		}
		if reflect.DeepEqual(before, settings) {
			return nil
		}
		stored, err := common.Marshal(settings)
		if err != nil {
			return err
		}
		if err := tx.Model(&orgmodel.Organization{}).Where("id = ?", actor.OrgId).
			Update("alert_settings", string(stored)).Error; err != nil {
			return err
		}
		return permit.record(tx, orgmodel.AuditSettingsUpdate, orgmodel.AuditTargetOrganization, actor.OrgId, before, settings)
	})
	if err != nil {
		return orgmodel.AlertSettings{}, err
	}
	return settings, nil
}
