package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-cryo/cryo/internal/repository"
	"github.com/rs/zerolog/log"
)

func (s *Server) registerRepositoryRoutes(router gin.IRouter) {
	router.GET("/repositories", s.listRepositoriesHandler())
	router.GET("/repositories/:namespace/:name", s.getRepositoryHandler())
	router.GET("/repositories/:namespace/:name/check", s.checkRepositoryHandler())
	router.GET("/repositories/:namespace/:name/snapshots", s.listSnapshotsHandler())
	router.GET("/repositories/:namespace/:name/snapshots/:snapshotId/browse", s.browseSnapshotHandler())
	router.POST("/repositories", s.createRepositoryHandler())
	router.PUT("/repositories/:namespace/:name", s.updateRepositoryHandler())
	router.DELETE("/repositories/:namespace/:name", s.deleteRepositoryHandler())
}

func (s *Server) createRepositoryHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req repository.CreateRepositoryRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
			return
		}

		if req.Name == "" || req.HostRef == "" || req.Path == "" || req.ResticPassword == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "name, hostRef, path, and resticPassword are required"})
			return
		}

		repo, err := s.RepositoryProvider.Create(c.Request.Context(), &req)
		if err != nil {
			log.Error().Err(err).Str("name", req.Name).Msg("failed to create repository")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create repository"})
			return
		}
		c.JSON(http.StatusCreated, repo)
	}
}

func (s *Server) deleteRepositoryHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		namespace := c.Param("namespace")
		name := c.Param("name")

		if err := s.RepositoryProvider.Delete(c.Request.Context(), namespace, name); err != nil {
			log.Error().Err(err).Str("namespace", namespace).Str("name", name).Msg("failed to delete repository")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete repository"})
			return
		}
		c.Status(http.StatusNoContent)
	}
}

func (s *Server) updateRepositoryHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		namespace := c.Param("namespace")
		name := c.Param("name")

		var req repository.UpdateRepositoryRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
			return
		}

		if req.HostRef == "" && req.Path == "" && req.ResticPassword == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "at least one field must be provided"})
			return
		}

		repo, err := s.RepositoryProvider.Update(c.Request.Context(), namespace, name, &req)
		if err != nil {
			log.Error().Err(err).Str("namespace", namespace).Str("name", name).Msg("failed to update repository")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update repository"})
			return
		}
		c.JSON(http.StatusOK, repo)
	}
}

func (s *Server) listRepositoriesHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		repos, err := s.RepositoryProvider.List(c.Request.Context())
		if err != nil {
			log.Error().Err(err).Msg("failed to list repositories")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list repositories"})
			return
		}
		c.JSON(http.StatusOK, repos)
	}
}

func (s *Server) getRepositoryHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		namespace := c.Param("namespace")
		name := c.Param("name")
		repo, err := s.RepositoryProvider.Get(c.Request.Context(), namespace, name)
		if err != nil {
			log.Error().Err(err).Str("namespace", namespace).Str("name", name).Msg("failed to get repository")
			c.JSON(http.StatusNotFound, gin.H{"error": "repository not found"})
			return
		}
		c.JSON(http.StatusOK, repo)
	}
}

func (s *Server) checkRepositoryHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		namespace := c.Param("namespace")
		name := c.Param("name")
		repo, err := s.RepositoryProvider.Get(c.Request.Context(), namespace, name)
		if err != nil {
			log.Error().Err(err).Str("namespace", namespace).Str("name", name).Msg("failed to get repository")
			c.JSON(http.StatusNotFound, gin.H{"error": "repository not found"})
			return
		}

		status, err := s.ResticService.Check(c.Request.Context(), repo)
		if err != nil {
			log.Error().Err(err).Str("repository", name).Msg("failed to check repository")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check repository"})
			return
		}
		c.JSON(http.StatusOK, status)
	}
}

func (s *Server) listSnapshotsHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		namespace := c.Param("namespace")
		name := c.Param("name")
		repo, err := s.RepositoryProvider.Get(c.Request.Context(), namespace, name)
		if err != nil {
			log.Error().Err(err).Str("namespace", namespace).Str("name", name).Msg("failed to get repository")
			c.JSON(http.StatusNotFound, gin.H{"error": "repository not found"})
			return
		}

		snapshots, err := s.ResticService.ListSnapshots(c.Request.Context(), repo)
		if err != nil {
			log.Error().Err(err).Str("repository", name).Msg("failed to list snapshots")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list snapshots"})
			return
		}
		c.JSON(http.StatusOK, snapshots)
	}
}

func (s *Server) browseSnapshotHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		namespace := c.Param("namespace")
		name := c.Param("name")
		snapshotID := c.Param("snapshotId")
		browsePath := c.DefaultQuery("path", "/")

		repo, err := s.RepositoryProvider.Get(c.Request.Context(), namespace, name)
		if err != nil {
			log.Error().Err(err).Str("namespace", namespace).Str("name", name).Msg("failed to get repository")
			c.JSON(http.StatusNotFound, gin.H{"error": "repository not found"})
			return
		}

		result, err := s.ResticService.ListSnapshotFiles(c.Request.Context(), repo, snapshotID, browsePath)
		if err != nil {
			log.Error().Err(err).Str("repository", name).Str("snapshot", snapshotID).Msg("failed to browse snapshot")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to browse snapshot"})
			return
		}
		c.JSON(http.StatusOK, result)
	}
}
