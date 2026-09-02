package controller

import (
	"errors"
	"net/http"
	"strconv"

	mktsvc "github.com/QuantumNous/new-api/internal/skill-marketplace/service"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func adminSkillSvc() *mktsvc.AdminSkillService {
	return mktsvc.NewAdminSkillService(model.DB)
}

func adminVersionSvc() *mktsvc.AdminVersionService {
	return mktsvc.NewAdminVersionService(model.DB)
}

func skillIDParam(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid skill id"})
		return 0, false
	}
	return id, true
}

func versionIDParam(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("vid"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid version id"})
		return 0, false
	}
	return id, true
}

func AdminListSkills(c *gin.Context) {
	var req mktsvc.ListSkillsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	resp, err := adminSkillSvc().ListSkills(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": resp})
}

func AdminCreateSkill(c *gin.Context) {
	var req mktsvc.CreateSkillRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	skill, err := adminSkillSvc().CreateSkill(req, c.GetInt("id"))
	if err != nil {
		if errors.Is(err, mktsvc.ErrSlugTaken) {
			c.JSON(http.StatusConflict, gin.H{"success": false, "message": "slug already exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": skill})
}

func AdminUpdateSkill(c *gin.Context) {
	id, ok := skillIDParam(c)
	if !ok {
		return
	}
	var req mktsvc.UpdateSkillRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	skill, err := adminSkillSvc().UpdateSkill(id, req)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "skill not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": skill})
}

func AdminPublishSkill(c *gin.Context) {
	id, ok := skillIDParam(c)
	if !ok {
		return
	}
	skill, err := adminSkillSvc().PublishSkill(id, c.GetInt("id"))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "skill not found"})
			return
		}
		if errors.Is(err, mktsvc.ErrInvalidTransition) {
			c.JSON(http.StatusConflict, gin.H{"success": false, "message": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": skill})
}

func AdminDeprecateSkill(c *gin.Context) {
	id, ok := skillIDParam(c)
	if !ok {
		return
	}
	skill, err := adminSkillSvc().DeprecateSkill(id, c.GetInt("id"))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "skill not found"})
			return
		}
		if errors.Is(err, mktsvc.ErrInvalidTransition) {
			c.JSON(http.StatusConflict, gin.H{"success": false, "message": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": skill})
}

func AdminDeleteSkill(c *gin.Context) {
	id, ok := skillIDParam(c)
	if !ok {
		return
	}
	if err := adminSkillSvc().DeleteSkill(id, c.GetInt("id")); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "skill not found"})
			return
		}
		if errors.Is(err, mktsvc.ErrInvalidTransition) {
			c.JSON(http.StatusConflict, gin.H{"success": false, "message": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "deleted"})
}

func AdminUpdateSkillFeatured(c *gin.Context) {
	id, ok := skillIDParam(c)
	if !ok {
		return
	}
	var req mktsvc.FeaturedRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	skill, err := adminSkillSvc().UpdateFeatured(id, req, c.GetInt("id"))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "skill not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": skill})
}

func AdminGetSkillLogs(c *gin.Context) {
	id, ok := skillIDParam(c)
	if !ok {
		return
	}
	logs, err := adminSkillSvc().GetLogs(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": logs})
}

func AdminUploadVersion(c *gin.Context) {
	skillID, ok := skillIDParam(c)
	if !ok {
		return
	}
	var req mktsvc.UploadVersionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	version, err := adminVersionSvc().UploadVersion(skillID, req, c.GetInt("id"))
	if err != nil {
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "skill not found"})
		case errors.Is(err, mktsvc.ErrInvalidVersionFormat),
			errors.Is(err, mktsvc.ErrManifestInvalid),
			errors.Is(err, mktsvc.ErrManifestSlugMismatch),
			errors.Is(err, mktsvc.ErrManifestVersionMismatch):
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		case errors.Is(err, mktsvc.ErrVersionAlreadyExists):
			c.JSON(http.StatusConflict, gin.H{"success": false, "message": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": version})
}

func AdminUpdateVersion(c *gin.Context) {
	skillID, ok := skillIDParam(c)
	if !ok {
		return
	}
	versionID, ok := versionIDParam(c)
	if !ok {
		return
	}
	var req mktsvc.UpdateVersionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	version, err := adminVersionSvc().UpdateVersion(skillID, versionID, req, c.GetInt("id"))
	if err != nil {
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "version not found"})
		case errors.Is(err, mktsvc.ErrVersionNotDraft):
			c.JSON(http.StatusConflict, gin.H{"success": false, "message": err.Error()})
		case errors.Is(err, mktsvc.ErrManifestInvalid),
			errors.Is(err, mktsvc.ErrManifestSlugMismatch),
			errors.Is(err, mktsvc.ErrManifestVersionMismatch):
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": version})
}

func AdminActivateVersion(c *gin.Context) {
	skillID, ok := skillIDParam(c)
	if !ok {
		return
	}
	versionID, ok := versionIDParam(c)
	if !ok {
		return
	}
	version, err := adminVersionSvc().ActivateVersion(skillID, versionID, c.GetInt("id"))
	if err != nil {
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "skill or version not found"})
		case errors.Is(err, mktsvc.ErrProviderKeyDetected), errors.Is(err, mktsvc.ErrRuntimeDependencyMissing):
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": version})
}

func AdminDeleteVersion(c *gin.Context) {
	skillID, ok := skillIDParam(c)
	if !ok {
		return
	}
	versionID, ok := versionIDParam(c)
	if !ok {
		return
	}
	if err := adminVersionSvc().DeleteVersion(skillID, versionID, c.GetInt("id")); err != nil {
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "version not found"})
		case errors.Is(err, mktsvc.ErrVersionNotDraft):
			c.JSON(http.StatusConflict, gin.H{"success": false, "message": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "deleted"})
}
