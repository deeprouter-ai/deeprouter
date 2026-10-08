package service

import (
	"errors"

	"github.com/QuantumNous/new-api/common"
	orgmodel "github.com/QuantumNous/new-api/internal/org/model"
	platformmodel "github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

// ErrNoWallet means a request made with an organization key cannot be billed:
// the key's holder is not a member of that organization any more, or the
// owner's account — the company wallet — is gone or disabled. Such a request
// is refused rather than served for free or at somebody else's expense.
var ErrNoWallet = errors.New("organization key has no company wallet to spend from")

// Spend is what the gateway needs to bill and to label a request made with an
// organization key (PRD §7.4).
type Spend struct {
	OrgId        int // the organization the key belongs to
	DepartmentId int // the department the key's holder is in right now
	WalletUserId int // the organization's owner: the company wallet is their balance
}

// SpendOf resolves whose balance pays for a request made with an organization
// key, and the department its usage is stamped with: the one its holder is in
// right now, so a later move never rewrites what was already spent.
//
// 🔴 This is the one thing the relay path reads from the organization tables,
// and it is not a permission question: who may do what is never asked on that
// path (PRD §7.3, red line 4). It is one query, made only for a key whose
// org_id is set — a personal key never gets here.
//
// Nothing is cached, on purpose. A member who changes department is stamped
// with the new one from their next request, and an owner whose account the
// platform disables stops the whole organization's spending at once.
func SpendOf(db *gorm.DB, orgID int, holderID int) (Spend, error) {
	var rows []struct {
		DepartmentId int
		OwnerUserId  int
		WalletStatus int
	}
	// The table is named, not a model: the row is read under two names, the
	// key's holder and the organization's owner. So the soft-delete filter GORM
	// would add for a model is written out for both.
	err := db.Table("users AS holder").
		Select("holder.department_id AS department_id, organizations.owner_user_id AS owner_user_id, wallet.status AS wallet_status").
		Joins("JOIN organizations ON organizations.id = holder.org_id").
		Joins("JOIN users AS wallet ON wallet.id = organizations.owner_user_id AND wallet.deleted_at IS NULL").
		Where("holder.id = ? AND holder.org_id = ? AND holder.deleted_at IS NULL", holderID, orgID).
		Limit(1).Scan(&rows).Error
	if err != nil {
		return Spend{}, err
	}
	if len(rows) == 0 || rows[0].WalletStatus != common.UserStatusEnabled {
		return Spend{}, ErrNoWallet
	}
	return Spend{OrgId: orgID, DepartmentId: rows[0].DepartmentId, WalletUserId: rows[0].OwnerUserId}, nil
}

// WalletWatchers returns the members who are told when the company wallet
// runs low: its holder — the owner, who tops it up — and the admins (PRD
// §7.4). Never the member whose request happened to cross the line: the
// balance is not theirs to see. Each comes with what a notification needs: id,
// email and settings.
func WalletWatchers(db *gorm.DB, orgID int, walletUserID int) ([]platformmodel.User, error) {
	adminRoleID, err := orgmodel.PresetRoleID(db, orgmodel.RoleAdmin)
	if err != nil {
		return nil, err
	}
	var watchers []platformmodel.User
	err = db.Select("id", "email", "setting").
		Where("org_id = ? AND (id = ? OR role_id = ?) AND status = ?",
			orgID, walletUserID, adminRoleID, common.UserStatusEnabled).
		Order("id").Find(&watchers).Error
	return watchers, err
}
