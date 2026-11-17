package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/models"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/services"
)

const (
	AdminSessionCookie = "snappsy_admin"
	AdminCSRFCookie    = "snappsy_csrf"
	CSRFHeader         = "X-CSRF-Token"
)

func AdminRequired(adminSvc *services.AdminService) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := c.Cookie(AdminSessionCookie)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "admin authentication required"})
			return
		}
		session, err := adminSvc.Authenticate(c.Request.Context(), token)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "admin authentication required"})
			return
		}
		c.Set("adminSession", session)
		c.Set("adminUsername", adminSvc.Username())
		c.Next()
	}
}

func AdminCSRF(adminSvc *services.AdminService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if isSafeMethod(c.Request.Method) {
			c.Next()
			return
		}
		session, ok := c.Get("adminSession")
		if !ok {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "invalid csrf token"})
			return
		}
		header := c.GetHeader(CSRFHeader)
		cookie, err := c.Cookie(AdminCSRFCookie)
		if err != nil || header == "" || header != cookie {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "invalid csrf token"})
			return
		}
		if !adminSvc.ValidateCSRF(session.(models.AdminSession), header) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "invalid csrf token"})
			return
		}
		c.Next()
	}
}

func isSafeMethod(method string) bool {
	switch strings.ToUpper(method) {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
		return true
	default:
		return false
	}
}
