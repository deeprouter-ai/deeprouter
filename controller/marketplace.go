package controller

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	mktsvc "github.com/QuantumNous/new-api/internal/skill-marketplace/service"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

// User-facing Skill Marketplace endpoints (PRD §6.1 / §6.2). Admin endpoints
// live in admin_marketplace.go.

func publicSkillSvc() *mktsvc.PublicSkillService {
	return mktsvc.NewPublicSkillService(model.DB)
}

func downloadSvc() *mktsvc.DownloadService {
	return mktsvc.NewDownloadService(model.DB, func() float64 { return common.QuotaPerUnit })
}

func userSkillSvc() *mktsvc.UserSkillService {
	return mktsvc.NewUserSkillService(model.DB)
}

func pageLimitParams(c *gin.Context) (int, int) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	return page, limit
}

// ListMarketplaceSkills is the anonymous marketplace listing.
func ListMarketplaceSkills(c *gin.Context) {
	var req mktsvc.PublicListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	resp, err := publicSkillSvc().ListPublishedSkills(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": resp})
}

// GetMarketplaceSkill is the anonymous skill detail page.
func GetMarketplaceSkill(c *gin.Context) {
	detail, err := publicSkillSvc().GetSkillBySlug(c.Param("slug"))
	if err != nil {
		if errors.Is(err, mktsvc.ErrSkillNotAvailable) {
			c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "skill not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": detail})
}

// DownloadMarketplaceSkill runs the entitlement check (paid skills charge the
// user's quota once) and streams the packaged ZIP back.
func DownloadMarketplaceSkill(c *gin.Context) {
	userID := c.GetInt("id")
	result, err := downloadSvc().Download(int64(userID), c.Param("slug"))
	if err != nil {
		switch {
		case errors.Is(err, mktsvc.ErrSkillNotAvailable):
			c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "skill not found"})
		case errors.Is(err, mktsvc.ErrInsufficientQuota):
			resp := gin.H{"success": false, "message": err.Error()}
			var iqe *mktsvc.InsufficientQuotaError
			if errors.As(err, &iqe) {
				resp["data"] = gin.H{"price_usd": iqe.PriceUSD, "quota_needed": iqe.QuotaNeeded}
			}
			c.JSON(http.StatusPaymentRequired, resp)
		case errors.Is(err, mktsvc.ErrPackageMissing):
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		}
		return
	}
	if result.PurchaseMade {
		// The deduction ran as a direct DB transaction (it must commit or roll
		// back together with the purchase row), so refresh the gateway's quota
		// cache from the DB now rather than leaving it stale.
		_, _ = model.GetUserQuota(userID, true)
	}
	c.Header("Content-Disposition", `attachment; filename="`+result.Filename+`"`)
	c.Data(http.StatusOK, "application/zip", result.Zip)
}

// GetUserMarketplaceSkills is the logged-in My Skills list.
func GetUserMarketplaceSkills(c *gin.Context) {
	page, limit := pageLimitParams(c)
	resp, err := userSkillSvc().ListUserSkills(int64(c.GetInt("id")), page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": resp})
}

// GetUserMarketplacePurchases is the logged-in Purchase History list.
func GetUserMarketplacePurchases(c *gin.Context) {
	page, limit := pageLimitParams(c)
	resp, err := userSkillSvc().ListUserPurchases(int64(c.GetInt("id")), page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": resp})
}
