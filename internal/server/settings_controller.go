package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-cryo/cryo/internal/settings"
	"github.com/rs/zerolog/log"
)

func (s *Server) registerSettingsRoutes() {
	base := s.Options.ApiBaseUrl + "/settings"
	s.Engine.GET(base, s.getSettingsHandler())
	s.Engine.PUT(base, s.updateSettingsHandler())
}

func (s *Server) getSettingsHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		result, err := s.SettingsProvider.Get(c.Request.Context())
		if err != nil {
			log.Error().Err(err).Msg("failed to get settings")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get settings"})
			return
		}
		c.JSON(http.StatusOK, result)
	}
}

func (s *Server) updateSettingsHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req settings.UpdateSettingsRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
			return
		}

		result, err := s.SettingsProvider.Update(c.Request.Context(), &req)
		if err != nil {
			log.Error().Err(err).Msg("failed to update settings")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update settings"})
			return
		}
		c.JSON(http.StatusOK, result)
	}
}
