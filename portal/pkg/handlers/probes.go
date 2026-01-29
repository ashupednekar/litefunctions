package handlers

import (
	"github.com/ashupednekar/litefunctions/portal/pkg/state"
	"github.com/gin-gonic/gin"
)

type ProbeHandler struct {
	state *state.AppState
}

func (s *ProbeHandler) Livez(ctx *gin.Context) {

}

func (s *ProbeHandler) Healthz(ctx *gin.Context) {

}
