package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (s *Server) registerAuthInfoRoute() {
	s.Engine.GET(s.Options.ApiBaseUrl+"/auth/info", s.getAuthInfoHandler())
}

// registerAuthSessionRoute registers a protected endpoint that confirms the
// caller has a valid session regardless of the auth method. combinedAuth gates
// it, so reaching the handler means the request is authenticated. The SPA router
// guard uses this instead of /auth/me, which only the BasicAuth handler serves.
func (s *Server) registerAuthSessionRoute(group *gin.RouterGroup) {
	group.GET("/auth/session", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"authenticated": true})
	})
}

func (s *Server) getAuthInfoHandler() gin.HandlerFunc {
	basicEnabled := s.BasicAuthHandler != nil
	oidcEnabled := s.OidcHandler != nil

	oidcLoginURL := ""
	if oidcEnabled {
		oidcLoginURL = s.OidcHandler.LoginURL()
	}

	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"basicEnabled": basicEnabled,
			"oidcEnabled":  oidcEnabled,
			"oidcLoginUrl": oidcLoginURL,
		})
	}
}
