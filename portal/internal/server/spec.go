package server

import (
	"context"
	"fmt"
	"strings"

	"github.com/ashupednekar/litefunctions/portal/internal/config"
	"github.com/ashupednekar/litefunctions/portal/internal/function"
	"github.com/ashupednekar/litefunctions/portal/internal/state"
	"github.com/gin-gonic/gin"
	"golang.org/x/sync/errgroup"
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
		Port:   config.Cfg.Port,
		router: gin.Default(),
		state:  state,
	}

	// Never trust X-Forwarded-* headers by default. Set TRUSTED_PROXIES to the
	// ingress/gateway CIDR when the portal is behind a reverse proxy.
	var trustedProxies []string
	for _, proxy := range strings.Split(config.Cfg.TrustedProxies, ",") {
		if proxy = strings.TrimSpace(proxy); proxy != "" {
			trustedProxies = append(trustedProxies, proxy)
		}
	}
	if err := s.router.SetTrustedProxies(trustedProxies); err != nil {
		return nil, fmt.Errorf("configure trusted proxies: %w", err)
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
		return function.Consume(s.state)
	})
	g.Wait()
}
