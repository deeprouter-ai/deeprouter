package service

import (
	"slices"
	"strings"

	"github.com/QuantumNous/new-api/common"
	platformmodel "github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

// Enterprise Org P6 (meta-repo docs/enterprise-org-prd.md D16): a member of an
// organization makes no keys of their own. Where the console used to make one
// for a purpose — "make a video", "write code" — it asks here instead which of
// the keys the member was handed can serve that purpose.

// OwnKeyView is one of the caller's own keys as far as one purpose goes.
type OwnKeyView struct {
	Id   int    `json:"id"`
	Name string `json:"name"`
	// Models are the models of the purpose the key may call, where the purpose
	// names models; what a purpose says by rule ("claude-*") is not listed.
	Models []string `json:"models"`
}

// ListOwnKeysFor returns the actor's own keys that work right now and may call
// something the purpose is made of, newest first. A member reads what is their
// own whatever their role (PRD §2), so it asks for no permission.
func ListOwnKeysFor(db *gorm.DB, actor *Actor, purpose string, catalogue ModelCatalogue) ([]OwnKeyView, error) {
	entries, err := catalogue.PurposeModels(actor.UserId, purpose)
	if err != nil {
		return nil, err
	}
	var keys []platformmodel.Token
	if err := db.Where("user_id = ? AND org_id = ? AND status = ?", actor.UserId, actor.OrgId, common.TokenStatusEnabled).
		Order("id DESC").Find(&keys).Error; err != nil {
		return nil, err
	}
	views := []OwnKeyView{}
	for i := range keys {
		if spent(&keys[i]) != nil {
			continue
		}
		models, serves := purposeModelsOf(&keys[i], entries)
		if !serves {
			continue
		}
		views = append(views, OwnKeyView{Id: keys[i].Id, Name: keys[i].Name, Models: models})
	}
	return views, nil
}

// purposeModelsOf reports whether a key may call anything a purpose is made
// of, and names the models among it. A purpose is a list of entries, each a
// model name or a "prefix*" rule — the same two forms a key's whitelist takes.
func purposeModelsOf(key *platformmodel.Token, entries []string) (models []string, serves bool) {
	limits := keyModelLimits(key)
	models = []string{}
	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		// A key without a whitelist may call every model.
		if len(limits) > 0 && !slices.ContainsFunc(limits, func(limit string) bool { return allowancesMeet(limit, entry) }) {
			continue
		}
		serves = true
		if !strings.HasSuffix(entry, "*") && !slices.Contains(models, entry) {
			models = append(models, entry)
		}
	}
	slices.Sort(models)
	return models, serves
}

// allowancesMeet reports whether two whitelist entries have a model in common.
// Each is a model name or a "prefix*" rule, which is how the gateway reads a
// key's whitelist (platform model.MatchModelLimit).
func allowancesMeet(a string, b string) bool {
	aPrefix, aIsRule := strings.CutSuffix(a, "*")
	bPrefix, bIsRule := strings.CutSuffix(b, "*")
	switch {
	case aIsRule && bIsRule:
		return strings.HasPrefix(aPrefix, bPrefix) || strings.HasPrefix(bPrefix, aPrefix)
	case aIsRule:
		return strings.HasPrefix(b, aPrefix)
	case bIsRule:
		return strings.HasPrefix(a, bPrefix)
	default:
		return a == b
	}
}
