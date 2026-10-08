package controller

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/internal/keypurpose"
	"github.com/QuantumNous/new-api/internal/kids"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/setting/alias_setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

func mediaCandidatesForUser(userID int) ([]keypurpose.Candidate, error) {
	group, err := model.GetUserGroup(userID, false)
	if err != nil {
		return nil, err
	}
	user, err := model.GetUserById(userID, false)
	if err != nil {
		return nil, err
	}
	abilities, err := model.GetAllEnableAbilityWithChannels()
	if err != nil {
		return nil, err
	}
	settings, err := model.GetUserSetting(userID, false)
	if err != nil {
		return nil, err
	}
	// Refresh endpoint metadata from the same catalog used by /v1/models.
	model.GetPricing()
	candidates := make([]keypurpose.Candidate, 0)
	for _, ability := range abilities {
		if ability.Group != group || (user.KidsMode && !kids.IsModelEligible(ability.Model)) {
			continue
		}
		if !operation_setting.SelfUseModeEnabled && !settings.AcceptUnsetRatioModel {
			if !helper.HasModelBillingConfig(ability.Model) {
				continue
			}
		}
		endpoints := model.GetModelSupportEndpointTypes(ability.Model)
		if len(endpoints) == 0 {
			endpoints = common.GetEndpointTypesByChannelType(ability.ChannelType, ability.Model)
		}
		candidates = append(candidates, keypurpose.Candidate{Name: ability.Model, Endpoints: endpoints})
	}
	return candidates, nil
}

func mediaModelsForUser(userID int, purpose string) ([]string, error) {
	candidates, err := mediaCandidatesForUser(userID)
	if err != nil {
		return nil, err
	}
	return keypurpose.Models(purpose, candidates), nil
}

// checkSimpleKeyBinding refuses a purpose, brand or price tier the registry
// does not know. Each of them may be left empty; what is given has to exist.
func checkSimpleKeyBinding(token *model.Token) error {
	if token.SimplePurpose != "" && !alias_setting.KnownPurpose(token.SimplePurpose) {
		return errors.New("token.purpose_unknown")
	}
	if token.SimpleBrand != "" && !alias_setting.KnownBrand(token.SimpleBrand) {
		return errors.New("token.brand_unknown")
	}
	if token.SimplePriceTier != "" && !alias_setting.KnownPriceTier(token.SimplePriceTier) {
		return errors.New("token.price_tier_unknown")
	}
	return nil
}

// applySimpleKeyPurpose is shared by create/update; media may never silently
// become unrestricted when a purpose has no usable models, and no key may
// become unrestricted because its purpose or price tier was mistyped.
func applySimpleKeyPurpose(token *model.Token) error {
	if err := checkSimpleKeyBinding(token); err != nil {
		return err
	}
	if token.SimplePurpose == "" {
		return nil
	}
	if keypurpose.IsMedia(token.SimplePurpose) {
		models, err := mediaModelsForUser(token.UserId, token.SimplePurpose)
		if err != nil {
			return err
		}
		if len(models) == 0 {
			return errors.New("token.media_models_unavailable")
		}
		token.Group = ""
		token.CrossGroupRetry = false
		token.SimpleBrand = ""
		token.ModelLimits = strings.Join(models, ",")
		token.ModelLimitsEnabled = true
		return nil
	}
	if list, limited := alias_setting.ModelWhitelistForToken(token.SimplePurpose, token.SimpleBrand, token.SimplePriceTier); limited {
		token.ModelLimits = alias_setting.ModelWhitelistString(list)
		token.ModelLimitsEnabled = true
	} else {
		token.ModelLimits = ""
		token.ModelLimitsEnabled = false
	}
	return nil
}
