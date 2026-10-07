package controller

import (
	"errors"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/internal/keypurpose"
	orgservice "github.com/QuantumNous/new-api/internal/org/service"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/alias_setting"

	"github.com/gin-gonic/gin"
)

// Enterprise Org P5 (meta-repo docs/enterprise-org-prd.md §3): the keys of an
// organization. They are created, changed, rotated, frozen and deleted here,
// under /api/org/keys, by whoever the permission engine lets through — and
// nowhere else: the personal key endpoints in token.go turn them away.

// errOrgKeyManagedByOrg is what a personal key endpoint answers when it is
// asked to change, delete or reveal an organization key.
var errOrgKeyManagedByOrg = errors.New("organization keys are managed through the organization endpoints")

// refuseOrgKeys closes a personal key endpoint to organization keys (PRD §7.3,
// red line 2). A key handed to a member is theirs as far as those endpoints can
// tell — they only ask "is it your key" — so without this its holder could
// raise its quota, lift its model limits, unfreeze it, delete it or read its
// value. It answers the request and reports true when any of ids is an
// organization key of the caller; a personal key passes untouched.
func refuseOrgKeys(c *gin.Context, ids ...int) bool {
	if len(ids) == 0 {
		return false
	}
	var held int64
	err := model.DB.Model(&model.Token{}).
		Where("user_id = ? AND org_id <> ? AND id IN ?", c.GetInt("id"), 0, ids).Count(&held).Error
	if err != nil {
		common.ApiError(c, err)
		return true
	}
	if held == 0 {
		return false
	}
	orgError(c, errOrgKeyManagedByOrg)
	return true
}

// orgModelCatalogue answers the organization package's questions about models
// from what the platform already knows for personal keys.
type orgModelCatalogue struct{}

// PurposeModels resolves one purpose of a policy template to what a key held
// by holderID may call, through the same two sources a personal key's purpose
// goes through (applySimpleKeyPurpose): the live catalogue for media, the rule
// table for everything else.
func (orgModelCatalogue) PurposeModels(holderID int, purpose string) ([]string, error) {
	if keypurpose.IsMedia(purpose) {
		return mediaModelsForUser(holderID, purpose)
	}
	rules, _ := alias_setting.ModelWhitelistForToken(purpose, "", "")
	return rules, nil
}

// ServableModels names every model a key held by holderID can be served right
// now: enabled in the holder's group, priced, and — for a kids account —
// eligible. It is the list a policy template's media purposes are drawn from,
// before it is narrowed to one purpose. A model that several channels serve is
// named once for each of them.
func (orgModelCatalogue) ServableModels(holderID int) ([]string, error) {
	candidates, err := mediaCandidatesForUser(holderID)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		names = append(names, candidate.Name)
	}
	return names, nil
}

// ListOrgKeys returns the keys of the caller's organization they may see.
func ListOrgKeys(c *gin.Context) {
	actor, ok := orgActor(c)
	if !ok {
		return
	}
	keys, err := orgservice.ListKeys(model.DB, actor)
	if err != nil {
		orgError(c, err)
		return
	}
	common.ApiSuccess(c, keys)
}

// ListOrgKeyHolders returns the members the caller may create a key for.
func ListOrgKeyHolders(c *gin.Context) {
	actor, ok := orgActor(c)
	if !ok {
		return
	}
	holders, err := orgservice.ListKeyHolders(model.DB, actor)
	if err != nil {
		orgError(c, err)
		return
	}
	common.ApiSuccess(c, holders)
}

// ListOrgKeyTemplates returns the policy templates a key can be given.
func ListOrgKeyTemplates(c *gin.Context) {
	actor, ok := orgActor(c)
	if !ok {
		return
	}
	templates, err := orgservice.ListKeyTemplates(actor)
	if err != nil {
		orgError(c, err)
		return
	}
	common.ApiSuccess(c, templates)
}

// ListOrgKeyModels returns the models a key held by the member named in
// ?holder_id= can be limited to by hand.
func ListOrgKeyModels(c *gin.Context) {
	actor, ok := orgActor(c)
	if !ok {
		return
	}
	// A missing or malformed id names nobody. It is left to the service to
	// say so, after it has asked whether the caller may ask at all.
	holderID, _ := strconv.Atoi(c.Query("holder_id"))
	models, err := orgservice.ListKeyModels(model.DB, actor, holderID, orgModelCatalogue{})
	if err != nil {
		orgError(c, err)
		return
	}
	common.ApiSuccess(c, models)
}

// CreateOrgKey creates a key for a member of the caller's organization. The
// answer carries the key's value only when it was made for a service account.
func CreateOrgKey(c *gin.Context) {
	actor, ok := orgActor(c)
	if !ok {
		return
	}
	var input orgservice.KeyInput
	if !orgBody(c, &input) {
		return
	}
	grant, err := orgservice.CreateKey(model.DB, actor, input, orgModelCatalogue{})
	if err != nil {
		orgError(c, err)
		return
	}
	common.ApiSuccess(c, grant)
}

// UpdateOrgKey changes the name, quota, expiry, limits or allowed models of a key.
func UpdateOrgKey(c *gin.Context) {
	actor, ok := orgActor(c)
	if !ok {
		return
	}
	id, ok := orgPathID(c)
	if !ok {
		return
	}
	var patch orgservice.KeyPatch
	if !orgBody(c, &patch) {
		return
	}
	key, err := orgservice.UpdateKey(model.DB, actor, id, patch, orgModelCatalogue{})
	if err != nil {
		orgError(c, err)
		return
	}
	common.ApiSuccess(c, key)
}

// RotateOrgKey replaces the value of a key; the old one stops working at once.
// The answer carries the new value only for a service account's key.
func RotateOrgKey(c *gin.Context) {
	actor, ok := orgActor(c)
	if !ok {
		return
	}
	id, ok := orgPathID(c)
	if !ok {
		return
	}
	grant, err := orgservice.RotateKey(model.DB, actor, id)
	if err != nil {
		orgError(c, err)
		return
	}
	common.ApiSuccess(c, grant)
}

// FreezeOrgKey stops a key from working until it is unfrozen.
func FreezeOrgKey(c *gin.Context) {
	actor, ok := orgActor(c)
	if !ok {
		return
	}
	id, ok := orgPathID(c)
	if !ok {
		return
	}
	if err := orgservice.FreezeKey(model.DB, actor, id); err != nil {
		orgError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

// UnfreezeOrgKey makes a frozen key work again.
func UnfreezeOrgKey(c *gin.Context) {
	actor, ok := orgActor(c)
	if !ok {
		return
	}
	id, ok := orgPathID(c)
	if !ok {
		return
	}
	if err := orgservice.UnfreezeKey(model.DB, actor, id); err != nil {
		orgError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

// DeleteOrgKey deletes a key; its usage history stays.
func DeleteOrgKey(c *gin.Context) {
	actor, ok := orgActor(c)
	if !ok {
		return
	}
	id, ok := orgPathID(c)
	if !ok {
		return
	}
	if err := orgservice.DeleteKey(model.DB, actor, id); err != nil {
		orgError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}
