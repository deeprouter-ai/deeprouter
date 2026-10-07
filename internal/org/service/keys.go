package service

import (
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	orgmodel "github.com/QuantumNous/new-api/internal/org/model"
	platformmodel "github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"gorm.io/gorm"
)

// KeyNameMaxLength is the longest key name, in characters.
const KeyNameMaxLength = 50

// keyCacheSweepDelay is how long after a change the gateway's cached copy of a
// key is dropped a second time (see forgetKeyValue).
var keyCacheSweepDelay = 2 * time.Second

// dropCachedKey makes the gateway's token cache forget a key value, and reports
// whether there is such a cache. It is a variable so that tests, which run
// without Redis, can watch it being called.
var dropCachedKey = platformmodel.InvalidateTokenCacheByKey

var (
	// ErrKeyNotFound means the key is not one of this organization's.
	ErrKeyNotFound = errors.New("key not found in this organization")
	// ErrInvalidKeyName means the key name is blank or too long.
	ErrInvalidKeyName = errors.New("key name must be 1 to 50 characters")
	// ErrInvalidKeyQuota means the quota is negative or beyond what the platform accepts.
	ErrInvalidKeyQuota = errors.New("key quota is out of range")
	// ErrInvalidKeyLimit means a rate limit or the monthly limit is negative.
	ErrInvalidKeyLimit = errors.New("a key's limits cannot be negative")
	// ErrInvalidKeyExpiry means the expiry is a moment that has already passed.
	ErrInvalidKeyExpiry = errors.New("a key's expiry must lie in the future")
	// ErrKeyTemplateUnknown means the policy template is not one the platform
	// offers. An unknown name is refused rather than ignored: ignoring it would
	// hand out a key that may call every model (PRD §5).
	ErrKeyTemplateUnknown = errors.New("unknown policy template")
	// ErrKeyTemplateEmpty means the policy template resolves to no model at all
	// right now. Such a key is refused too, for the same reason.
	ErrKeyTemplateEmpty = errors.New("the policy template resolves to no model")
	// ErrKeyModelsWithTemplate means a key was given a policy template and a
	// hand-picked list of models at once. It takes one or the other.
	ErrKeyModelsWithTemplate = errors.New("a key takes a policy template or a list of models, not both")
	// ErrKeyModelsUnavailable means a hand-picked model is not one the key's
	// holder can be served. What is returned is a *KeyModelsUnavailableError,
	// which names the models.
	ErrKeyModelsUnavailable = errors.New("a chosen model is not available to the key's holder")
	// ErrKeyLimitReached means the holder already has as many keys as one
	// account may hold.
	ErrKeyLimitReached = errors.New("the holder already has the maximum number of keys")
	// ErrKeyExpired means the key cannot be unfrozen because its expiry has passed.
	ErrKeyExpired = errors.New("an expired key cannot be unfrozen")
	// ErrKeyExhausted means the key cannot be unfrozen because it has no quota left.
	ErrKeyExhausted = errors.New("a key with no quota left cannot be unfrozen")
)

// KeyModelsUnavailableError is ErrKeyModelsUnavailable together with the
// models it is about, so the refusal can say which ones to take out.
type KeyModelsUnavailableError struct {
	Models []string
}

// Error names the models the holder cannot be served.
func (e *KeyModelsUnavailableError) Error() string {
	return fmt.Sprintf("%s: %s", ErrKeyModelsUnavailable.Error(), strings.Join(e.Models, ", "))
}

// Is makes the error answer to ErrKeyModelsUnavailable.
func (e *KeyModelsUnavailableError) Is(target error) bool {
	return target == ErrKeyModelsUnavailable
}

// PurposeModels resolves one purpose of a policy template to what a key held
// by holderID may call for it: model names, or "prefix*" rules.
type PurposeModels func(holderID int, purpose string) ([]string, error)

// ModelCatalogue is what the platform says a key held by a member may call.
// The controller package supplies it, because both answers live with the
// personal-key code they are shared with; this package only knows which
// purposes a template is made of, and that a hand-picked list is chosen from
// what its holder can be served.
type ModelCatalogue interface {
	// PurposeModels resolves one purpose of a policy template, as the
	// PurposeModels type describes.
	PurposeModels(holderID int, purpose string) ([]string, error)
	// ServableModels names every model a key held by holderID can be served
	// right now, in any order; a name may come more than once.
	ServableModels(holderID int) ([]string, error)
}

// KeyView is one organization key as the keys page lists it. Key is the masked
// form: the value itself never leaves the server through this type (PRD D15).
type KeyView struct {
	Id              int    `json:"id"`
	Name            string `json:"name"`
	Key             string `json:"key"`
	Status          int    `json:"status"`
	HolderId        int    `json:"holder_id"`
	Holder          string `json:"holder"` // empty when that account no longer exists
	HolderIsService bool   `json:"holder_is_service"`
	DepartmentId    int    `json:"department_id"` // the holder's department
	Department      string `json:"department"`
	PolicyTemplate  string `json:"policy_template"`
	// ModelLimits is what the key may call; empty means every model.
	ModelLimits    []string `json:"model_limits"`
	RemainQuota    int      `json:"remain_quota"`
	UsedQuota      int      `json:"used_quota"`
	UnlimitedQuota bool     `json:"unlimited_quota"`
	ExpiredTime    int64    `json:"expired_time"` // -1 means never
	RpmLimit       int      `json:"rpm_limit"`
	TpmLimit       int      `json:"tpm_limit"`
	MonthlyLimit   int      `json:"monthly_limit"`
	CreatedTime    int64    `json:"created_time"`
	AccessedTime   int64    `json:"accessed_time"`
}

// KeyGrant is a key together with its value. The value is set only for a key
// held by a service account, and only in the answer to the request that made
// that value — a creation or a rotation (PRD D15). A person's key has none:
// its holder takes it through one-click setup.
type KeyGrant struct {
	KeyView
	Value string `json:"value,omitempty"`
}

// KeyHolderView is a member a new key can be made out to.
type KeyHolderView struct {
	Id           int    `json:"id"`
	Name         string `json:"name"`
	DepartmentId int    `json:"department_id"`
	Department   string `json:"department"`
	// RoleId and Role say which role the member holds, so that the key form can
	// narrow a long list down by it. Both are empty for an actor who may not
	// read that member: who holds which role is the member list's to tell.
	RoleId    int    `json:"role_id"`
	Role      string `json:"role"`
	IsService bool   `json:"is_service"`
	IsOwner   bool   `json:"is_owner"`
}

// KeyInput is what a key is created with.
type KeyInput struct {
	Name string `json:"name"`
	// HolderId is who the key is for. Zero parks it under the owner.
	HolderId int `json:"holder_id"`
	// PolicyTemplate is the key of a policy template; empty for none.
	PolicyTemplate string `json:"policy_template"`
	// ModelLimits is a hand-picked list of the models the key may call, for a
	// key without a policy template. With neither, the key may call every model.
	ModelLimits    []string `json:"model_limits"`
	RemainQuota    int      `json:"remain_quota"`
	UnlimitedQuota bool     `json:"unlimited_quota"`
	ExpiredTime    int64    `json:"expired_time"` // -1 or 0 means never
	RpmLimit       int      `json:"rpm_limit"`
	TpmLimit       int      `json:"tpm_limit"`
	MonthlyLimit   int      `json:"monthly_limit"`
}

// KeyPatch is a change to one key; a nil field is left as it is. That matters
// most for the quota, which the key is spending while someone has the form open.
type KeyPatch struct {
	Name *string `json:"name"`
	// PolicyTemplate and ModelLimits are one thing — what the key may call —
	// and are replaced together: when either is given, the other counts as
	// empty unless it is given too. A template is applied afresh, also when it
	// names the one the key already has, which is how a key picks up models
	// the catalogue gained since.
	PolicyTemplate *string   `json:"policy_template"`
	ModelLimits    *[]string `json:"model_limits"`
	RemainQuota    *int      `json:"remain_quota"`
	UnlimitedQuota *bool     `json:"unlimited_quota"`
	ExpiredTime    *int64    `json:"expired_time"`
	RpmLimit       *int      `json:"rpm_limit"`
	TpmLimit       *int      `json:"tpm_limit"`
	MonthlyLimit   *int      `json:"monthly_limit"`
}

// keyHolder is the member a key belongs to, as far as the permission engine
// and the keys page need to know them.
type keyHolder struct {
	Id           int
	Name         string
	DepartmentId int
	Department   string
	IsService    bool
}

// keyRecord is what the audit log keeps of a key: who holds it and what it is
// allowed — never its value.
type keyRecord struct {
	Name           string   `json:"name"`
	HolderId       int      `json:"holder_id"`
	Holder         string   `json:"holder"`
	DepartmentId   int      `json:"department_id"`
	Department     string   `json:"department"`
	PolicyTemplate string   `json:"policy_template"`
	ModelLimits    []string `json:"model_limits"`
	RemainQuota    int      `json:"remain_quota"`
	UnlimitedQuota bool     `json:"unlimited_quota"`
	ExpiredTime    int64    `json:"expired_time"`
	RpmLimit       int      `json:"rpm_limit"`
	TpmLimit       int      `json:"tpm_limit"`
	MonthlyLimit   int      `json:"monthly_limit"`
	// ValueShown says the full value was shown to whoever did this, which only
	// happens with a service account's key.
	ValueShown bool `json:"value_shown,omitempty"`
}

// keyValueRecord is one side of a key.rotate record: the value that went or the
// one that came, masked.
type keyValueRecord struct {
	Name       string `json:"name"`
	Key        string `json:"key"`
	ValueShown bool   `json:"value_shown,omitempty"`
}

// keyStatusRecord is one side of a key.freeze or key.unfreeze record.
type keyStatusRecord struct {
	Name   string `json:"name"`
	Status int    `json:"status"`
}

// keyDeliveryRecord is what a key.deliver record keeps: which tools the value
// was installed into.
type keyDeliveryRecord struct {
	Name  string   `json:"name"`
	Tools []string `json:"tools"`
}

// holderName is what to call an account: its display name, or its username
// when it never set one.
func holderName(user *platformmodel.User) string {
	if user.DisplayName != "" {
		return user.DisplayName
	}
	return user.Username
}

// keyHolders loads the holders of keys by user id. Accounts deleted since are
// still returned — their keys go on existing and have to be shown as somebody's
// — while an id outside the organization is simply absent.
func keyHolders(db *gorm.DB, orgID int, userIDs []int) (map[int]keyHolder, error) {
	holders := make(map[int]keyHolder, len(userIDs))
	if len(userIDs) == 0 {
		return holders, nil
	}
	var users []platformmodel.User
	if err := db.Unscoped().Select("id", "username", "display_name", "department_id", "is_service").
		Where("id IN ? AND org_id = ?", userIDs, orgID).Find(&users).Error; err != nil {
		return nil, err
	}
	departments, err := departmentNames(db, orgID)
	if err != nil {
		return nil, err
	}
	for i := range users {
		holders[users[i].Id] = keyHolder{
			Id:           users[i].Id,
			Name:         holderName(&users[i]),
			DepartmentId: users[i].DepartmentId,
			Department:   departments[users[i].DepartmentId],
			IsService:    users[i].IsService,
		}
	}
	return holders, nil
}

// departmentNames returns the names of an organization's departments, by id.
func departmentNames(db *gorm.DB, orgID int) (map[int]string, error) {
	var departments []orgmodel.Department
	if err := db.Select("id", "name").Where("org_id = ?", orgID).Find(&departments).Error; err != nil {
		return nil, err
	}
	names := make(map[int]string, len(departments))
	for _, department := range departments {
		names[department.Id] = department.Name
	}
	return names, nil
}

// keyTarget is what the permission engine judges a key by: the member holding
// it and the department that member sits in. A key whose holder has left the
// organization has no department, so only a role reaching all of it can act.
func keyTarget(key *platformmodel.Token, holder keyHolder) orgmodel.Target {
	return orgmodel.Target{DepartmentId: holder.DepartmentId, UserId: key.UserId}
}

// findKey loads one key of an organization together with its holder.
func findKey(db *gorm.DB, orgID int, keyID int) (*platformmodel.Token, keyHolder, error) {
	var found []platformmodel.Token
	if err := db.Where("id = ? AND org_id = ?", keyID, orgID).Limit(1).Find(&found).Error; err != nil {
		return nil, keyHolder{}, err
	}
	if len(found) == 0 {
		return nil, keyHolder{}, ErrKeyNotFound
	}
	holders, err := keyHolders(db, orgID, []int{found[0].UserId})
	if err != nil {
		return nil, keyHolder{}, err
	}
	return &found[0], holders[found[0].UserId], nil
}

// permitOnKey is the gate of every change to an existing key: it loads the key
// and gives leave to do primitive on it. Whoever holds the primitive nowhere is
// turned away before anything is read, so their answer cannot depend on
// whether the key exists.
func permitOnKey(db *gorm.DB, actor *Actor, primitive string, keyID int) (*platformmodel.Token, keyHolder, *writePermit, error) {
	if !actor.Holds(primitive) {
		return nil, keyHolder{}, nil, ErrForbidden
	}
	key, holder, err := findKey(db, actor.OrgId, keyID)
	if err != nil {
		return nil, keyHolder{}, nil, err
	}
	permit, err := actor.permit(primitive, keyTarget(key, holder))
	if err != nil {
		return nil, keyHolder{}, nil, err
	}
	return key, holder, permit, nil
}

// keyModelLimits returns what a key may call; empty means every model.
func keyModelLimits(key *platformmodel.Token) []string {
	if !key.ModelLimitsEnabled {
		return []string{}
	}
	return key.GetModelLimits()
}

// viewKey renders a key for the keys page, value masked.
func viewKey(key *platformmodel.Token, holder keyHolder) KeyView {
	return KeyView{
		Id:              key.Id,
		Name:            key.Name,
		Key:             key.GetMaskedKey(),
		Status:          key.Status,
		HolderId:        key.UserId,
		Holder:          holder.Name,
		HolderIsService: holder.IsService,
		DepartmentId:    holder.DepartmentId,
		Department:      holder.Department,
		PolicyTemplate:  key.PolicyTemplate,
		ModelLimits:     keyModelLimits(key),
		RemainQuota:     key.RemainQuota,
		UsedQuota:       key.UsedQuota,
		UnlimitedQuota:  key.UnlimitedQuota,
		ExpiredTime:     key.ExpiredTime,
		RpmLimit:        key.RpmLimit,
		TpmLimit:        key.TpmLimit,
		MonthlyLimit:    key.MonthlyLimit,
		CreatedTime:     key.CreatedTime,
		AccessedTime:    key.AccessedTime,
	}
}

// recordKey renders a key for the audit log.
func recordKey(key *platformmodel.Token, holder keyHolder) keyRecord {
	return keyRecord{
		Name:           key.Name,
		HolderId:       key.UserId,
		Holder:         holder.Name,
		DepartmentId:   holder.DepartmentId,
		Department:     holder.Department,
		PolicyTemplate: key.PolicyTemplate,
		ModelLimits:    keyModelLimits(key),
		RemainQuota:    key.RemainQuota,
		UnlimitedQuota: key.UnlimitedQuota,
		ExpiredTime:    key.ExpiredTime,
		RpmLimit:       key.RpmLimit,
		TpmLimit:       key.TpmLimit,
		MonthlyLimit:   key.MonthlyLimit,
	}
}

// normalizeKeyName trims a key name and checks its length.
func normalizeKeyName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if name == "" || utf8.RuneCountInString(name) > KeyNameMaxLength {
		return "", ErrInvalidKeyName
	}
	return name, nil
}

// checkKeyQuota checks a quota against the range the platform accepts for a key.
func checkKeyQuota(quota int) error {
	if quota < 0 || quota > int(1000000000*common.QuotaPerUnit) {
		return ErrInvalidKeyQuota
	}
	return nil
}

// checkKeyLimits checks the per-minute and monthly limits; zero means none.
func checkKeyLimits(limits ...int) error {
	for _, limit := range limits {
		if limit < 0 {
			return ErrInvalidKeyLimit
		}
	}
	return nil
}

// normalizeKeyExpiry turns a requested expiry into the stored form: -1 for
// never, otherwise a moment that has not passed yet.
func normalizeKeyExpiry(expiry int64) (int64, error) {
	if expiry <= 0 {
		return -1, nil
	}
	if expiry <= common.GetTimestamp() {
		return 0, ErrInvalidKeyExpiry
	}
	return expiry, nil
}

// resolveTemplate turns a policy template into the models a key held by
// holderID may call. An empty template means every model and returns nothing;
// a template that is unknown, or that resolves to nothing, is an error — it
// must never fall through to "every model" (PRD §5).
func resolveTemplate(templateKey string, holderID int, models PurposeModels) ([]string, error) {
	if templateKey == "" {
		return []string{}, nil
	}
	template, ok := orgmodel.FindPolicyTemplate(templateKey)
	if !ok {
		return nil, ErrKeyTemplateUnknown
	}
	limits := []string{}
	for _, purpose := range template.Purposes {
		names, err := models(holderID, purpose)
		if err != nil {
			return nil, err
		}
		for _, name := range names {
			if name = strings.TrimSpace(name); name != "" && !slices.Contains(limits, name) {
				limits = append(limits, name)
			}
		}
	}
	if len(limits) == 0 {
		return nil, ErrKeyTemplateEmpty
	}
	// One order whatever the sources returned, so the same template always
	// stores — and records — the same list.
	slices.Sort(limits)
	return limits, nil
}

// resolveAllowance turns what a key is asked to be allowed into the list its
// whitelist stores: the models of a policy template, a hand-picked list, or —
// with neither — nothing, which means every model. A key takes a template or a
// list, never both.
//
// A hand-picked model has to be one the holder can be served, so a mistyped or
// retired name is caught here and not by whoever ends up using the key. The
// entries the key is limited to already (kept) may stay whatever the catalogue
// says today: a change is only ever refused for what it adds.
func resolveAllowance(templateKey string, chosen []string, kept []string, holderID int, catalogue ModelCatalogue) ([]string, error) {
	picked := []string{}
	for _, name := range chosen {
		if name = strings.TrimSpace(name); name != "" && !slices.Contains(picked, name) {
			picked = append(picked, name)
		}
	}
	if templateKey != "" {
		if len(picked) > 0 {
			return nil, ErrKeyModelsWithTemplate
		}
		return resolveTemplate(templateKey, holderID, catalogue.PurposeModels)
	}
	added := []string{}
	for _, name := range picked {
		if !slices.Contains(kept, name) {
			added = append(added, name)
		}
	}
	if len(added) > 0 {
		servable, err := catalogue.ServableModels(holderID)
		if err != nil {
			return nil, err
		}
		unavailable := []string{}
		for _, name := range added {
			if !slices.Contains(servable, name) {
				unavailable = append(unavailable, name)
			}
		}
		if len(unavailable) > 0 {
			return nil, &KeyModelsUnavailableError{Models: unavailable}
		}
	}
	slices.Sort(picked)
	return picked, nil
}

// forgetKeyValue makes the gateway stop trusting what it cached under a key
// value. The gateway answers a request from that cache for up to a minute
// without looking at the database, so a key that was just rotated, frozen,
// changed or deleted would otherwise go on working as it was.
//
// It drops the entry now and once more shortly after: a request already in
// flight may have read the key before the change and write it back into the
// cache just after the first drop. Where there is no cache there is nothing to
// come back for, and no timer is left behind.
func forgetKeyValue(value string) {
	dropFromCache := dropCachedKey
	drop := func() bool {
		cached, err := dropFromCache(value)
		if err != nil {
			common.SysError("org: failed to drop a key from the token cache: " + err.Error())
		}
		return cached
	}
	if drop() {
		time.AfterFunc(keyCacheSweepDelay, func() { drop() })
	}
}

// ListKeys returns the organization's keys the actor may see, newest first:
// all of them for a role that reaches the whole organization, those held in
// the departments they manage for a department-scoped one.
func ListKeys(db *gorm.DB, actor *Actor) ([]KeyView, error) {
	everywhere, departments, err := actor.reach("key.read")
	if err != nil {
		return nil, err
	}
	var keys []platformmodel.Token
	if err := db.Where("org_id = ?", actor.OrgId).Order("id DESC").Find(&keys).Error; err != nil {
		return nil, err
	}
	holderIDs := make([]int, 0, len(keys))
	for i := range keys {
		holderIDs = append(holderIDs, keys[i].UserId)
	}
	holders, err := keyHolders(db, actor.OrgId, holderIDs)
	if err != nil {
		return nil, err
	}
	views := make([]KeyView, 0, len(keys))
	for i := range keys {
		holder := holders[keys[i].UserId]
		if !everywhere && !slices.Contains(departments, holder.DepartmentId) {
			continue
		}
		views = append(views, viewKey(&keys[i], holder))
	}
	return views, nil
}

// ListKeyTemplates returns the policy templates a key can be given.
func ListKeyTemplates(actor *Actor) ([]orgmodel.PolicyTemplate, error) {
	if !actor.Holds("key.read") {
		return nil, ErrForbidden
	}
	return orgmodel.PolicyTemplates, nil
}

// ListKeyModels returns the models a key held by a member can be limited to by
// hand: what that member can be served right now, in name order. It is for
// whoever fills in the key form, so it asks what the form's two uses ask — the
// right to create a key for that member, or to change one they hold.
func ListKeyModels(db *gorm.DB, actor *Actor, holderID int, catalogue ModelCatalogue) ([]string, error) {
	// Whoever fills in no key form at all is turned away before anything is
	// read: the answer must not depend on whether the member exists.
	if !actor.Holds("key.create") && !actor.Holds("key.update") {
		return nil, ErrForbidden
	}
	member, err := findMember(db, actor.OrgId, holderID)
	if err != nil {
		return nil, err
	}
	target := orgmodel.Target{DepartmentId: member.DepartmentId, UserId: member.Id}
	if !actor.can("key.create", target) && !actor.can("key.update", target) {
		return nil, ErrForbidden
	}
	servable, err := catalogue.ServableModels(member.Id)
	if err != nil {
		return nil, err
	}
	models := append([]string{}, servable...)
	slices.Sort(models)
	return slices.Compact(models), nil
}

// mayCreateKeyFor reports whether the actor may create a key held by the
// member the target describes. Making a key out to someone is handing it out
// as well as creating it, so it takes key.assign too (PRD D30); a key parked
// under the owner has been handed to nobody yet.
func (a *Actor) mayCreateKeyFor(target orgmodel.Target, ownerID int) bool {
	if !a.can("key.create", target) {
		return false
	}
	return target.UserId == ownerID || a.can("key.assign", target)
}

// ListKeyHolders returns the members the actor may create a key for. It asks
// for nothing but the right to create keys, so the create form works for a
// role that cannot read the member list — and tells such a role no more than a
// name and a department: a member's role is added only for an actor who may
// read that member.
func ListKeyHolders(db *gorm.DB, actor *Actor) ([]KeyHolderView, error) {
	if !actor.Holds("key.create") {
		return nil, ErrForbidden
	}
	ownerID, err := ownerUserID(db, actor.OrgId)
	if err != nil {
		return nil, err
	}
	var users []platformmodel.User
	if err := db.Select("id", "username", "display_name", "role_id", "department_id", "is_service").
		Where("org_id = ?", actor.OrgId).Order("id").Find(&users).Error; err != nil {
		return nil, err
	}
	departments, err := departmentNames(db, actor.OrgId)
	if err != nil {
		return nil, err
	}
	roles, err := loadRoles(db, actor.OrgId)
	if err != nil {
		return nil, err
	}
	roleNames := make(map[int]string, len(roles))
	for _, role := range roles {
		roleNames[role.Id] = role.Name
	}
	views := make([]KeyHolderView, 0, len(users))
	for i := range users {
		target := orgmodel.Target{DepartmentId: users[i].DepartmentId, UserId: users[i].Id}
		if !actor.mayCreateKeyFor(target, ownerID) {
			continue
		}
		view := KeyHolderView{
			Id:           users[i].Id,
			Name:         holderName(&users[i]),
			DepartmentId: users[i].DepartmentId,
			Department:   departments[users[i].DepartmentId],
			IsService:    users[i].IsService,
			IsOwner:      users[i].Id == ownerID,
		}
		if actor.can("member.read", target) {
			view.RoleId = users[i].OrgRoleId
			view.Role = roleNames[users[i].OrgRoleId]
		}
		views = append(views, view)
	}
	return views, nil
}

// CreateKey creates a key for a member of the organization — or, when no
// holder is named, parks it under the owner — with its quota, its limits and
// what it may call: the models of a policy template, a hand-picked list, or
// every model.
//
// The value comes back only when the holder is a service account, which has no
// other way to receive it. A person's key is never shown to whoever created it.
func CreateKey(db *gorm.DB, actor *Actor, input KeyInput, catalogue ModelCatalogue) (*KeyGrant, error) {
	// Whoever cannot create keys at all is turned away before anything is read:
	// the answer must not depend on whether the holder they named exists.
	if !actor.Holds("key.create") {
		return nil, ErrForbidden
	}
	ownerID, err := ownerUserID(db, actor.OrgId)
	if err != nil {
		return nil, err
	}
	holderID := input.HolderId
	if holderID == 0 {
		holderID = ownerID
	}
	member, err := findMember(db, actor.OrgId, holderID)
	if err != nil {
		return nil, err
	}
	target := orgmodel.Target{DepartmentId: member.DepartmentId, UserId: member.Id}
	permit, err := actor.permit("key.create", target)
	if err != nil {
		return nil, err
	}
	if !actor.mayCreateKeyFor(target, ownerID) {
		return nil, ErrForbidden
	}

	name, err := normalizeKeyName(input.Name)
	if err != nil {
		return nil, err
	}
	if err := checkKeyQuota(input.RemainQuota); err != nil {
		return nil, err
	}
	if err := checkKeyLimits(input.RpmLimit, input.TpmLimit, input.MonthlyLimit); err != nil {
		return nil, err
	}
	expiry, err := normalizeKeyExpiry(input.ExpiredTime)
	if err != nil {
		return nil, err
	}
	limits, err := resolveAllowance(input.PolicyTemplate, input.ModelLimits, nil, member.Id, catalogue)
	if err != nil {
		return nil, err
	}
	value, err := common.GenerateKey()
	if err != nil {
		return nil, err
	}
	now := common.GetTimestamp()
	key := platformmodel.Token{
		UserId:             member.Id,
		Key:                value,
		Status:             common.TokenStatusEnabled,
		Name:               name,
		CreatedTime:        now,
		AccessedTime:       now,
		ExpiredTime:        expiry,
		RemainQuota:        input.RemainQuota,
		UnlimitedQuota:     input.UnlimitedQuota,
		ModelLimitsEnabled: len(limits) > 0,
		ModelLimits:        strings.Join(limits, ","),
		RpmLimit:           input.RpmLimit,
		TpmLimit:           input.TpmLimit,
		MonthlyLimit:       input.MonthlyLimit,
		OrgId:              actor.OrgId,
		CreatedBy:          actor.UserId,
		PolicyTemplate:     input.PolicyTemplate,
	}
	var holder keyHolder
	err = db.Transaction(func(tx *gorm.DB) error {
		var held int64
		if err := tx.Model(&platformmodel.Token{}).Where("user_id = ?", member.Id).Count(&held).Error; err != nil {
			return err
		}
		if int(held) >= operation_setting.GetMaxUserTokens() {
			return ErrKeyLimitReached
		}
		if err := tx.Create(&key).Error; err != nil {
			return err
		}
		holders, err := keyHolders(tx, actor.OrgId, []int{member.Id})
		if err != nil {
			return err
		}
		holder = holders[member.Id]
		record := recordKey(&key, holder)
		record.ValueShown = holder.IsService
		return permit.record(tx, orgmodel.AuditKeyCreate, orgmodel.AuditTargetKey, key.Id, nil, record)
	})
	if err != nil {
		return nil, err
	}
	grant := &KeyGrant{KeyView: viewKey(&key, holder)}
	if holder.IsService {
		grant.Value = value
	}
	return grant, nil
}

// UpdateKey changes what a key is allowed: its name, quota, expiry, limits and
// the models it may call. Who holds it and whether it is frozen are other
// actions.
func UpdateKey(db *gorm.DB, actor *Actor, keyID int, patch KeyPatch, catalogue ModelCatalogue) (*KeyView, error) {
	key, holder, permit, err := permitOnKey(db, actor, "key.update", keyID)
	if err != nil {
		return nil, err
	}
	changed := *key
	updates := map[string]any{}
	if patch.Name != nil {
		if changed.Name, err = normalizeKeyName(*patch.Name); err != nil {
			return nil, err
		}
		updates["name"] = changed.Name
	}
	if patch.RemainQuota != nil {
		if err := checkKeyQuota(*patch.RemainQuota); err != nil {
			return nil, err
		}
		changed.RemainQuota = *patch.RemainQuota
		updates["remain_quota"] = changed.RemainQuota
	}
	if patch.UnlimitedQuota != nil {
		changed.UnlimitedQuota = *patch.UnlimitedQuota
		updates["unlimited_quota"] = changed.UnlimitedQuota
	}
	if patch.ExpiredTime != nil {
		if changed.ExpiredTime, err = normalizeKeyExpiry(*patch.ExpiredTime); err != nil {
			return nil, err
		}
		updates["expired_time"] = changed.ExpiredTime
	}
	for column, limit := range map[string]struct {
		given *int
		field *int
	}{
		"rpm_limit":     {patch.RpmLimit, &changed.RpmLimit},
		"tpm_limit":     {patch.TpmLimit, &changed.TpmLimit},
		"monthly_limit": {patch.MonthlyLimit, &changed.MonthlyLimit},
	} {
		if limit.given == nil {
			continue
		}
		if err := checkKeyLimits(*limit.given); err != nil {
			return nil, err
		}
		*limit.field = *limit.given
		updates[column] = *limit.given
	}
	if patch.PolicyTemplate != nil || patch.ModelLimits != nil {
		template, chosen := "", []string(nil)
		if patch.PolicyTemplate != nil {
			template = *patch.PolicyTemplate
		}
		if patch.ModelLimits != nil {
			chosen = *patch.ModelLimits
		}
		limits, err := resolveAllowance(template, chosen, keyModelLimits(key), key.UserId, catalogue)
		if err != nil {
			return nil, err
		}
		changed.PolicyTemplate = template
		changed.ModelLimitsEnabled = len(limits) > 0
		changed.ModelLimits = strings.Join(limits, ",")
		updates["policy_template"] = changed.PolicyTemplate
		updates["model_limits_enabled"] = changed.ModelLimitsEnabled
		updates["model_limits"] = changed.ModelLimits
	}

	view := viewKey(&changed, holder)
	wrote := false
	err = db.Transaction(func(tx *gorm.DB) error {
		before := recordKey(key, holder)
		after := recordKey(&changed, holder)
		// Saving a form nobody touched changes nothing and is nothing to record.
		if reflect.DeepEqual(before, after) {
			return nil
		}
		if err := tx.Model(&platformmodel.Token{}).
			Where("id = ? AND org_id = ?", key.Id, actor.OrgId).Updates(updates).Error; err != nil {
			return err
		}
		wrote = true
		return permit.record(tx, orgmodel.AuditKeyUpdate, orgmodel.AuditTargetKey, key.Id, before, after)
	})
	if err != nil {
		return nil, err
	}
	if wrote {
		forgetKeyValue(key.Key)
	}
	return &view, nil
}

// RotateKey replaces the value of a key. The row stays: holder, settings, used
// quota and usage history are all kept, and the old value stops working at
// once (PRD §3 — there is no grace period).
//
// The new value comes back only for a service account's key. The holder of a
// person's key runs one-click setup again to receive it.
func RotateKey(db *gorm.DB, actor *Actor, keyID int) (*KeyGrant, error) {
	key, holder, permit, err := permitOnKey(db, actor, "key.rotate", keyID)
	if err != nil {
		return nil, err
	}
	value, err := common.GenerateKey()
	if err != nil {
		return nil, err
	}
	rotated := *key
	rotated.Key = value
	err = db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&platformmodel.Token{}).
			Where("id = ? AND org_id = ?", key.Id, actor.OrgId).Update("key", value).Error; err != nil {
			return err
		}
		return permit.record(tx, orgmodel.AuditKeyRotate, orgmodel.AuditTargetKey, key.Id,
			keyValueRecord{Name: key.Name, Key: key.GetMaskedKey()},
			keyValueRecord{Name: key.Name, Key: rotated.GetMaskedKey(), ValueShown: holder.IsService})
	})
	if err != nil {
		return nil, err
	}
	forgetKeyValue(key.Key)
	grant := &KeyGrant{KeyView: viewKey(&rotated, holder)}
	if holder.IsService {
		grant.Value = value
	}
	return grant, nil
}

// FreezeKey stops a key from working until it is unfrozen. Freezing a key that
// is already frozen does nothing.
func FreezeKey(db *gorm.DB, actor *Actor, keyID int) error {
	key, _, permit, err := permitOnKey(db, actor, "key.freeze", keyID)
	if err != nil {
		return err
	}
	if key.Status == common.TokenStatusDisabled {
		return nil
	}
	return setKeyStatus(db, actor, permit, key, common.TokenStatusDisabled, orgmodel.AuditKeyFreeze)
}

// UnfreezeKey makes a frozen key work again. A key that ran out of quota or
// passed its expiry stays unusable until that is put right, so it is refused.
func UnfreezeKey(db *gorm.DB, actor *Actor, keyID int) error {
	key, _, permit, err := permitOnKey(db, actor, "key.freeze", keyID)
	if err != nil {
		return err
	}
	if key.Status == common.TokenStatusEnabled {
		return nil
	}
	if key.ExpiredTime != -1 && key.ExpiredTime <= common.GetTimestamp() {
		return ErrKeyExpired
	}
	if !key.UnlimitedQuota && key.RemainQuota <= 0 {
		return ErrKeyExhausted
	}
	return setKeyStatus(db, actor, permit, key, common.TokenStatusEnabled, orgmodel.AuditKeyUnfreeze)
}

// setKeyStatus writes a key's new status with its audit record, then makes the
// gateway forget the old one.
func setKeyStatus(db *gorm.DB, actor *Actor, permit *writePermit, key *platformmodel.Token, status int, audited string) error {
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&platformmodel.Token{}).
			Where("id = ? AND org_id = ?", key.Id, actor.OrgId).Update("status", status).Error; err != nil {
			return err
		}
		return permit.record(tx, audited, orgmodel.AuditTargetKey, key.Id,
			keyStatusRecord{Name: key.Name, Status: key.Status},
			keyStatusRecord{Name: key.Name, Status: status})
	})
	if err != nil {
		return err
	}
	forgetKeyValue(key.Key)
	return nil
}

// DeleteKey removes a key for good. Its usage history stays: the log lines
// carry the key's id and name, and the row itself is only marked deleted.
func DeleteKey(db *gorm.DB, actor *Actor, keyID int) error {
	key, holder, permit, err := permitOnKey(db, actor, "key.delete", keyID)
	if err != nil {
		return err
	}
	err = db.Transaction(func(tx *gorm.DB) error {
		before := recordKey(key, holder)
		if err := tx.Where("id = ? AND org_id = ?", key.Id, actor.OrgId).
			Delete(&platformmodel.Token{}).Error; err != nil {
			return err
		}
		return permit.record(tx, orgmodel.AuditKeyDelete, orgmodel.AuditTargetKey, key.Id, before, nil)
	})
	if err != nil {
		return err
	}
	forgetKeyValue(key.Key)
	return nil
}

// RecordKeyDelivery writes down that the value of an organization key left the
// server through one-click setup (PRD D15, D32): whose key, into which tools,
// and the address of the machine that took it. It is the holder's own doing,
// not a management action, so it goes to the log without a permit. A personal
// key is nobody's business and records nothing.
func RecordKeyDelivery(db *gorm.DB, key *platformmodel.Token, tools []string, ip string) error {
	if key.OrgId == 0 {
		return nil
	}
	return RecordAudit(db, AuditEntry{
		OrgId:       key.OrgId,
		ActorUserId: key.UserId,
		Ip:          ip,
		Action:      orgmodel.AuditKeyDeliver,
		TargetType:  orgmodel.AuditTargetKey,
		TargetId:    key.Id,
		After:       keyDeliveryRecord{Name: key.Name, Tools: tools},
	})
}
