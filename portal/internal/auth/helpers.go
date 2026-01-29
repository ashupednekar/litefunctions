package auth

import (
	"encoding/json"

	"github.com/gin-gonic/gin"
)

func GetUsername(ctx *gin.Context) (string, error) {
	type Username struct {
		Username string `json:"username"`
	}
	var u Username
	if err := json.NewDecoder(ctx.Request.Body).Decode(&u); err != nil {
		return "", err
	}
	return u.Username, nil
}
