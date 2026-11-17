package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/models"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/services"
)

const (
	CustomerSessionCookie = "snappsy_customer"
	CustomerCSRFCookie    = "snappsy_customer_csrf"
)

func CustomerRequired(customerSvc *services.CustomerService) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := c.Cookie(CustomerSessionCookie)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "customer authentication required"})
			return
		}
		customer, session, err := customerSvc.Authenticate(c.Request.Context(), token)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "customer authentication required"})
			return
		}
		c.Set("customer", customer)
		c.Set("customerSession", session)
		c.Next()
	}
}

func CustomerCSRF(customerSvc *services.CustomerService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if isSafeMethod(c.Request.Method) {
			c.Next()
			return
		}
		session, ok := c.Get("customerSession")
		if !ok {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "invalid csrf token"})
			return
		}
		header := c.GetHeader(CSRFHeader)
		cookie, err := c.Cookie(CustomerCSRFCookie)
		if err != nil || header == "" || header != cookie {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "invalid csrf token"})
			return
		}
		if !customerSvc.ValidateCSRF(session.(models.CustomerSession), header) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "invalid csrf token"})
			return
		}
		c.Next()
	}
}
