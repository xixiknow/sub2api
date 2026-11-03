package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestCheckinHandlerRejectsDirectClaimsBeforeAward(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, body := range []string{"", `{}`, `{"turnstile_token":"invented"}`, `{"captcha_hash":"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"}`} {
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/user/checkin", strings.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42})
		// No captcha service/database: reaching the award would panic. Every request must fail closed.
		h := &UserHandler{}
		h.ClaimCheckin(c)
		require.GreaterOrEqual(t, recorder.Code, 400)
	}
}
