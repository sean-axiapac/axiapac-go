package middlewares

import (
	"net/http"
	"strings"

	"axiapac.com/axiapac/web/common"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func parseJwt(tokenStr string, jwtSecret []byte) (*jwt.Token, error) {
	token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
		// Ensure the signing method is HMAC (or switch to RSA/ECDSA)
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return jwtSecret, nil
	},
		// TEMP WORKAROUND: skip exp/nbf/iat validation so the kiosk's hardcoded
		// (now-expired) token keeps working until it is rotated. Signature is
		// still verified. REVERT this option (and restore the exp check below)
		// once the kiosk ships a fresh token.
		jwt.WithoutClaimsValidation(),
	)
	return token, err
}

// AuthMiddleware checks for a valid Bearer token
func Authentication(jwtSecret []byte) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr := ""

		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			// Try to get from cookie
			cookie, err := c.Cookie("axiapac.ApplicationCookie")
			if err != nil {
				// Cookie not found either
				c.AbortWithStatus(http.StatusUnauthorized)
				return
			}

			tokenStr = cookie
		} else {
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				c.AbortWithStatus(http.StatusUnauthorized)
				return
			}

			tokenStr = parts[1]
		}

		// Parse and validate JWT
		token, err := parseJwt(tokenStr, jwtSecret)

		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, common.NewErrorResponse("invalid or expired token"))
			return
		}

		// Optional: check claims
		if claims, ok := token.Claims.(jwt.MapClaims); ok {
			// TEMP WORKAROUND: exp check disabled to let the kiosk's expired
			// hardcoded token through. Restore this block when reverting
			// jwt.WithoutClaimsValidation() in parseJwt.
			// if exp, ok := claims["exp"].(float64); ok && int64(exp) < time.Now().Unix() {
			// 	c.AbortWithStatusJSON(http.StatusUnauthorized, common.NewErrorResponse("token expired"))
			// 	return
			// }

			// Pass claims into context
			c.Set("claims", claims)
		}

		c.Next()
	}
}
