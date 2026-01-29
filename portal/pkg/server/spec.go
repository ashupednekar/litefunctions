package server

import (
	"fmt"

	"github.com/ashupednekar/litefunctions/portal/pkg"
	"github.com/ashupednekar/litefunctions/portal/pkg/state"
	"github.com/gin-gonic/gin"
)

type Server struct {
	Port   int
	router *gin.Engine
	state  *state.AppState
}

func NewServer() (*Server, error) {
	state, err := state.NewState()
	if err != nil {
		return nil, err
	}
	s := &Server{
		Port:   pkg.Cfg.Port,
		router: gin.Default(),
		state:  state,
	}
	s.BuildRoutes()
	return s, nil
}

func (s *Server) Start() {
	s.router.Run(fmt.Sprintf("0.0.0.0:%d", s.Port))
}
