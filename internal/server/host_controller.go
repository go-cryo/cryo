package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-cryo/cryo/internal/repositoryhost"
	"github.com/rs/zerolog/log"
)

func (s *Server) registerHostRoutes() {
	base := s.Options.ApiBaseUrl + "/hosts"
	s.Engine.GET(base, s.listHostsHandler())
	s.Engine.GET(base+"/:namespace/:name", s.getHostHandler())
	s.Engine.POST(base, s.createHostHandler())
	s.Engine.PUT(base+"/:namespace/:name", s.updateHostHandler())
	s.Engine.DELETE(base+"/:namespace/:name", s.deleteHostHandler())
}

func (s *Server) createHostHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req repositoryhost.CreateHostRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
			return
		}

		if req.Name == "" || req.BaseURL == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "name and baseUrl are required"})
			return
		}

		host, err := s.HostProvider.Create(c.Request.Context(), &req)
		if err != nil {
			log.Error().Err(err).Str("name", req.Name).Msg("failed to create repository host")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create repository host"})
			return
		}
		c.JSON(http.StatusCreated, host)
	}
}

func (s *Server) updateHostHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		namespace := c.Param("namespace")
		name := c.Param("name")

		var req repositoryhost.UpdateHostRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
			return
		}

		if req.BaseURL == "" && req.AwsAccessKeyID == "" && req.AwsSecretAccessKey == "" && req.AwsDefaultRegion == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "at least one field must be provided"})
			return
		}

		host, err := s.HostProvider.Update(c.Request.Context(), namespace, name, &req)
		if err != nil {
			log.Error().Err(err).Str("namespace", namespace).Str("name", name).Msg("failed to update repository host")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update repository host"})
			return
		}
		c.JSON(http.StatusOK, host)
	}
}

func (s *Server) deleteHostHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		namespace := c.Param("namespace")
		name := c.Param("name")
		hostRef := namespace + "/" + name

		if s.RepositoryProvider != nil {
			repos, err := s.RepositoryProvider.List(c.Request.Context())
			if err != nil {
				log.Error().Err(err).Msg("failed to list repositories for host reference check")
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check host references"})
				return
			}

			var referencing []string
			for _, repo := range repos {
				if repo.HostRef == hostRef {
					referencing = append(referencing, repo.Namespace+"/"+repo.Name)
				}
			}

			if len(referencing) > 0 {
				c.JSON(http.StatusConflict, gin.H{
					"error":        "host is still referenced by repositories",
					"repositories": referencing,
				})
				return
			}
		}

		if err := s.HostProvider.Delete(c.Request.Context(), namespace, name); err != nil {
			log.Error().Err(err).Str("namespace", namespace).Str("name", name).Msg("failed to delete repository host")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete repository host"})
			return
		}
		c.Status(http.StatusNoContent)
	}
}

func (s *Server) listHostsHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		hosts, err := s.HostProvider.List(c.Request.Context())
		if err != nil {
			log.Error().Err(err).Msg("failed to list repository hosts")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list repository hosts"})
			return
		}
		c.JSON(http.StatusOK, hosts)
	}
}

func (s *Server) getHostHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		namespace := c.Param("namespace")
		name := c.Param("name")
		host, err := s.HostProvider.Get(c.Request.Context(), namespace, name)
		if err != nil {
			log.Error().Err(err).Str("namespace", namespace).Str("name", name).Msg("failed to get repository host")
			c.JSON(http.StatusNotFound, gin.H{"error": "repository host not found"})
			return
		}
		c.JSON(http.StatusOK, host)
	}
}
