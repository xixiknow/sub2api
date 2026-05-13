package routes

import (
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	adminhandler "github.com/Wei-Shaw/sub2api/internal/handler/admin"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAdminGrowthBadgesRouteRegistered(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	v1 := router.Group("/api/v1")
	admin := v1.Group("/admin")

	registerGrowthRoutes(admin, &handler.Handlers{
		Admin: &handler.AdminHandlers{
			Growth: &adminhandler.GrowthHandler{},
		},
	})

	for _, route := range router.Routes() {
		if route.Method == http.MethodGet && route.Path == "/api/v1/admin/growth/badges" {
			return
		}
	}

	require.Fail(t, "GET /api/v1/admin/growth/badges route was not registered")
}
