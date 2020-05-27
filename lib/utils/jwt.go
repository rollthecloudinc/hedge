package utils

import (
	"strings"

	jwt "github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
)

func GetSubject(context *gin.Context) string {
	authPieces := strings.Split(context.GetHeader("Authorization"), " ")
	if len(authPieces) == 2 {
		claims := jwt.StandardClaims{}
		jwt.ParseWithClaims(authPieces[1], &claims, nil)
		return claims.Subject
	}
	return ""
}
