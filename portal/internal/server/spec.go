package server

import (
	"context"
	"fmt"

	"github.com/ashupednekar/litefunctions/portal/internal/config"
	"github.com/ashupednekar/litefunctions/portal/internal/function"
	"github.com/ashupednekar/litefunctions/portal/internal/state"
	"github.com/gin-gonic/gin"
	"github.com/nats-io/nats.go"
	"golang.org/x/sync/errgroup"
)

type Server struct {
	Port   int
	router *gin.Engine
	state  *state.AppState
	nc     *nats.Conn
}

func NewServer() (*Server, error) {
	state, err := state.NewState()
	if err != nil {
		return nil, err
	}
	nc, err := nats.Connect(config.Cfg.NatsUrl)
	if err != nil {
		return nil, err
	}
	s := &Server{
		Port:   config.Cfg.Port,
		router: gin.Default(),
		state:  state,
		nc:     nc,
	}
	s.BuildRoutes()
	return s, nil
}

func (s *Server) Start() {
	g, _ := errgroup.WithContext(context.Background())
	g.Go(func() error {
		return s.router.Run(fmt.Sprintf("0.0.0.0:%d", s.Port))
	})
	g.Go(func() error {
		return function.Consume(s.nc)
	})
	g.Wait()
}
