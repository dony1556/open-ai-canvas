package handler

import (
	"context"
	"github.com/gin-gonic/gin"
	"net/http"
	"time"
	"yingce/backend/internal/service"
)

func registerArkExistingAssetsRoutes(r *gin.RouterGroup, svc *service.Service) {
	r.GET("/ark-assets/groups", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		if !enforceRateLimit(c, "ark-existing-groups:"+user.ID, 60, time.Minute) {
			return
		}
		var query service.AdminArkAssetQuery
		if err := c.ShouldBindQuery(&query); err != nil {
			fail(c, http.StatusBadRequest, err)
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
		defer cancel()
		page, err := svc.ArkExistingGroups(ctx, user, query)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, page)
	})
	r.GET("/ark-assets", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		if !enforceRateLimit(c, "ark-existing-assets:"+user.ID, 60, time.Minute) {
			return
		}
		var query service.AdminArkAssetQuery
		if err := c.ShouldBindQuery(&query); err != nil {
			fail(c, http.StatusBadRequest, err)
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
		defer cancel()
		page, err := svc.ArkExistingAssets(ctx, user, query)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, page)
	})
}
