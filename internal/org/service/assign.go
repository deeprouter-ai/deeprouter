package service

import (
	"github.com/QuantumNous/new-api/common"
	orgmodel "github.com/QuantumNous/new-api/internal/org/model"
	platformmodel "github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"gorm.io/gorm"
)

// Enterprise Org P6 (meta-repo docs/enterprise-org-prd.md §3): who holds a
// key. A key goes to another holder by being assigned and comes back to the
// owner by being reclaimed; both take key.assign.
//
// 🔴 A key's value never outlives a change of holder. Whoever held the key has
// its value in their tools, and a value that went on working under the next
// holder would be spent by one person and billed to another. So every function
// here that moves a key gives it a new value in the same transaction — taking
// it back included, or the next person to unfreeze a reclaimed key would wake
// the copy of somebody who has left.

// keyHandover is one side of a key.assign or key.reclaim record: who held the
// key, whether it worked, and its value — masked.
type keyHandover struct {
	Name         string `json:"name"`
	HolderId     int    `json:"holder_id"`
	Holder       string `json:"holder"`
	DepartmentId int    `json:"department_id"`
	Department   string `json:"department"`
	Status       int    `json:"status"`
	Key          string `json:"key"`
	// ValueShown says the new value was shown to whoever did this, which only
	// happens when the key goes to a service account.
	ValueShown bool `json:"value_shown,omitempty"`
}

// describeHandover renders a key under one holder for the audit log.
func describeHandover(key *platformmodel.Token, holder keyHolder) keyHandover {
	return keyHandover{
		Name:         key.Name,
		HolderId:     key.UserId,
		Holder:       holder.Name,
		DepartmentId: holder.DepartmentId,
		Department:   holder.Department,
		Status:       key.Status,
		Key:          key.GetMaskedKey(),
	}
}

// handOver moves a key from one holder to another inside tx: a new value, the
// status it is to have from now on, and the record of it under the permit. It
// returns the key as it is afterwards; the caller makes the gateway forget the
// old value once the transaction has committed.
func handOver(tx *gorm.DB, permit *writePermit, audited string, key *platformmodel.Token, from keyHolder, to keyHolder, status int) (*platformmodel.Token, error) {
	value, err := common.GenerateKey()
	if err != nil {
		return nil, err
	}
	moved := *key
	moved.UserId, moved.Key, moved.Status = to.Id, value, status
	if err := tx.Model(&platformmodel.Token{}).Where("id = ? AND org_id = ?", key.Id, key.OrgId).
		Updates(map[string]any{"user_id": to.Id, "key": value, "status": status}).Error; err != nil {
		return nil, err
	}
	after := describeHandover(&moved, to)
	after.ValueShown = to.IsService
	if err := permit.record(tx, audited, orgmodel.AuditTargetKey, key.Id, describeHandover(key, from), after); err != nil {
		return nil, err
	}
	return &moved, nil
}

// ListKeyAssignees returns the members the actor may hand a key to. Like the
// list a new key is made out from, it asks for the one right the action takes
// and tells a member's role only to an actor who may read that member. The
// owner is not on it: a key goes back to the owner by being reclaimed.
func ListKeyAssignees(db *gorm.DB, actor *Actor) ([]KeyHolderView, error) {
	if !actor.Holds("key.assign") {
		return nil, ErrForbidden
	}
	return listKeyHolders(db, actor, func(target orgmodel.Target, ownerID int) bool {
		return target.UserId != ownerID && actor.can("key.assign", target)
	})
}

// AssignKey hands a key to another member of the organization. It takes
// key.assign twice over: reaching whoever holds the key now, and reaching
// whoever is to hold it — so a manager moves keys between the people of their
// departments and nowhere else.
//
// The key gets a new value on the way (see the top of this file). That value
// comes back only when the new holder is a service account, which has no other
// way to receive it; a person takes theirs through one-click setup.
//
// A frozen key is unfrozen for its new holder when the actor may unfreeze keys
// there and nothing else stops it from working. Otherwise it arrives frozen:
// handing a key out never gives the actor a power their role does not hold.
func AssignKey(db *gorm.DB, actor *Actor, keyID int, holderID int) (*KeyGrant, error) {
	key, holder, permit, err := permitOnKey(db, actor, "key.assign", keyID)
	if err != nil {
		return nil, err
	}
	owner, err := ownerAsHolder(db, actor.OrgId)
	if err != nil {
		return nil, err
	}
	// The owner holds what has been handed to nobody, so a key assigned to the
	// owner is a key taken back.
	if holderID == owner.Id {
		view, err := takeBack(db, permit, key, holder, owner)
		if err != nil {
			return nil, err
		}
		return &KeyGrant{KeyView: *view}, nil
	}
	member, err := findMember(db, actor.OrgId, holderID)
	if err != nil {
		return nil, err
	}
	target := orgmodel.Target{DepartmentId: member.DepartmentId, UserId: member.Id}
	if !actor.can("key.assign", target) {
		return nil, ErrForbidden
	}
	// Handing a key to whoever holds it already changes nothing, its value
	// included, and is nothing to record.
	if member.Id == key.UserId {
		return &KeyGrant{KeyView: viewKey(key, holder)}, nil
	}
	status := key.Status
	if status == common.TokenStatusDisabled && actor.can("key.freeze", target) && spent(key) == nil {
		status = common.TokenStatusEnabled
	}

	var moved *platformmodel.Token
	var recipient keyHolder
	err = db.Transaction(func(tx *gorm.DB) error {
		var held int64
		if err := tx.Model(&platformmodel.Token{}).Where("user_id = ?", member.Id).Count(&held).Error; err != nil {
			return err
		}
		if int(held) >= operation_setting.GetMaxUserTokens() {
			return ErrKeyLimitReached
		}
		holders, err := keyHolders(tx, actor.OrgId, []int{member.Id})
		if err != nil {
			return err
		}
		recipient = holders[member.Id]
		moved, err = handOver(tx, permit, orgmodel.AuditKeyAssign, key, holder, recipient, status)
		return err
	})
	if err != nil {
		return nil, err
	}
	forgetKeyValue(key.Key)
	grant := &KeyGrant{KeyView: viewKey(moved, recipient)}
	if recipient.IsService {
		grant.Value = moved.Key
	}
	return grant, nil
}

// ReclaimKey takes a key back from its holder: it stops working at once, gets
// a new value and is parked under the owner, from where it can be handed to
// someone else. What it has spent stays on it, and in the usage records under
// the name of whoever spent it.
func ReclaimKey(db *gorm.DB, actor *Actor, keyID int) (*KeyView, error) {
	key, holder, permit, err := permitOnKey(db, actor, "key.assign", keyID)
	if err != nil {
		return nil, err
	}
	owner, err := ownerAsHolder(db, actor.OrgId)
	if err != nil {
		return nil, err
	}
	return takeBack(db, permit, key, holder, owner)
}

// ownerAsHolder returns the owner of an organization as the holder of the keys
// parked under them.
func ownerAsHolder(db *gorm.DB, orgID int) (keyHolder, error) {
	ownerID, err := ownerUserID(db, orgID)
	if err != nil {
		return keyHolder{}, err
	}
	holders, err := keyHolders(db, orgID, []int{ownerID})
	if err != nil {
		return keyHolder{}, err
	}
	return holders[ownerID], nil
}

// takeBack parks a key under the owner, frozen and with a new value. A key
// that is there already has been handed to nobody and is left as it is.
//
// It does not ask how many keys the owner holds: taking a key back must not
// fail because the place it returns to is full.
func takeBack(db *gorm.DB, permit *writePermit, key *platformmodel.Token, holder keyHolder, owner keyHolder) (*KeyView, error) {
	if key.UserId == owner.Id {
		view := viewKey(key, holder)
		return &view, nil
	}
	var parked *platformmodel.Token
	err := db.Transaction(func(tx *gorm.DB) error {
		var err error
		parked, err = handOver(tx, permit, orgmodel.AuditKeyReclaim, key, holder, owner, common.TokenStatusDisabled)
		return err
	})
	if err != nil {
		return nil, err
	}
	forgetKeyValue(key.Key)
	view := viewKey(parked, owner)
	return &view, nil
}
