package controller

import (
	"errors"

	"github.com/QuantumNous/new-api/common"
	orgservice "github.com/QuantumNous/new-api/internal/org/service"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// msgOrgNameInvalid is the i18n key for a blank or over-long organization name
// (i18n/locales/*.yaml).
const msgOrgNameInvalid = "org.name_invalid"

// GetOrgSelf returns the caller's organization, role and department, or null
// data for a personal account (Enterprise Org P2).
func GetOrgSelf(c *gin.Context) {
	membership, err := orgservice.GetMembership(model.DB, c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, membership)
}

// insertRegisteredUser inserts a newly registered account and returns the id
// of the organization created with it (0 for a personal sign-up).
//
// With an orgName the account and the organization are created in one
// transaction, the new user becoming its owner — a sign-up that asked for a
// company must never leave behind a personal account without one. Without an
// orgName this is exactly the upstream insert, so personal sign-ups do not
// change at all.
func insertRegisteredUser(user *model.User, inviterId int, orgName string) (int, error) {
	if orgName == "" {
		return 0, user.Insert(inviterId)
	}
	// Checked before anything is written, so a bad name costs nothing.
	if _, err := orgservice.NormalizeName(orgName); err != nil {
		return 0, err
	}
	orgId := 0
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		if err := user.InsertWithTx(tx, inviterId); err != nil {
			return err
		}
		org, err := orgservice.CreateForOwnerTx(tx, user.Id, orgName)
		if err != nil {
			return err
		}
		orgId = org.Id
		return nil
	})
	if err != nil {
		return 0, err
	}
	// InsertWithTx plus this call is what Insert does in one go: the sidebar
	// config, the sign-up log line and the inviter rewards run after the commit.
	user.FinalizeOAuthUserCreation(inviterId)
	return orgId, nil
}

// isOrgNameError reports whether a sign-up failed on the organization name.
func isOrgNameError(err error) bool {
	return errors.Is(err, orgservice.ErrInvalidName)
}
