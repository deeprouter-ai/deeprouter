// Copyright (C) 2023-2026 QuantumNous
//
// API Key Simple-mode purpose & price-tier metadata (PRD
// docs/tasks/api-key-simple-advanced-prd.md). Drives the 6 cards rendered
// by the API Key create drawer in Simple mode.
package controller

import (
	"net/http"

	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/internal/keypurpose"
	"github.com/QuantumNous/new-api/setting/alias_setting"

	"github.com/gin-gonic/gin"
)

// GetApiKeyPurposes returns the localized purpose cards + price tiers used
// by the frontend's Simple-mode picker. Authenticated user route.
func GetApiKeyPurposes(c *gin.Context) {
	lang := i18n.GetLangFromContext(c)
	purposes := alias_setting.GetPurposeSummary(lang)
	candidates, catalogErr := mediaCandidatesForUser(c.GetInt("id"))
	for i := range purposes {
		if !keypurpose.IsMedia(purposes[i].ID) {
			continue
		}
		models := keypurpose.Models(purposes[i].ID, candidates)
		purposes[i].AvailableBrands = []string{}
		purposes[i].RecommendedBrand = ""
		purposes[i].AvailableModels = models
		purposes[i].AvailabilityError = catalogErr != nil
		purposes[i].HumanEstimate = ""
		purposes[i].PriceRange = ""
		available := catalogErr == nil && len(models) > 0
		purposes[i].Available = &available
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"purposes":           purposes,
			"price_tiers":        alias_setting.GetPriceTierSummary(lang),
			"default_price_tier": alias_setting.DefaultPriceTierID(),
		},
	})
}
