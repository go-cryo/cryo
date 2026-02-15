package server

import (
	"github.com/gin-gonic/gin"
)

func (s *Server) registerVersionRoute() error {
	s.Engine.GET(s.Options.ApiBaseUrl+"/version", s.getVersionHandler())
	return nil
}

func (s *Server) getVersionHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(200, gin.H{"version": s.Options.ServiceVersion})
	}
}
