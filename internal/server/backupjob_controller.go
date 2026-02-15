package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-cryo/cryo/internal/backupjob"
	"github.com/rs/zerolog/log"
)

func (s *Server) registerBackupJobRoutes() {
	base := s.Options.ApiBaseUrl + "/backupjobs"
	s.Engine.GET(base, s.listBackupJobsHandler())
	s.Engine.GET(base+"/:namespace/:name", s.getBackupJobHandler())
	s.Engine.POST(base, s.createBackupJobHandler())
	s.Engine.PUT(base+"/:namespace/:name", s.updateBackupJobHandler())
	s.Engine.DELETE(base+"/:namespace/:name", s.deleteBackupJobHandler())
	s.Engine.POST(base+"/:namespace/:name/trigger", s.triggerBackupJobHandler())
	s.Engine.GET(base+"/:namespace/:name/runs", s.listBackupRunsHandler())
}

func (s *Server) listBackupJobsHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		jobs, err := s.BackupJobProvider.List(c.Request.Context())
		if err != nil {
			log.Error().Err(err).Msg("failed to list backup jobs")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list backup jobs"})
			return
		}

		for _, job := range jobs {
			if s.Scheduler != nil {
				job.NextRun = s.Scheduler.NextRun(job.Namespace, job.Name)
			}
			if s.RunStore != nil {
				job.LastRun = s.RunStore.GetActive(job.Namespace, job.Name)
			}
		}

		c.JSON(http.StatusOK, jobs)
	}
}

func (s *Server) getBackupJobHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		namespace := c.Param("namespace")
		name := c.Param("name")

		job, err := s.BackupJobProvider.Get(c.Request.Context(), namespace, name)
		if err != nil {
			log.Error().Err(err).Str("namespace", namespace).Str("name", name).Msg("failed to get backup job")
			c.JSON(http.StatusNotFound, gin.H{"error": "backup job not found"})
			return
		}

		if s.Scheduler != nil {
			job.NextRun = s.Scheduler.NextRun(namespace, name)
		}
		if s.RunStore != nil {
			job.LastRun = s.RunStore.GetActive(namespace, name)
		}

		c.JSON(http.StatusOK, job)
	}
}

func (s *Server) createBackupJobHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req backupjob.CreateBackupJobRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
			return
		}

		if req.Name == "" || req.Type == "" || req.Schedule == "" || req.RepositoryRef == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "name, type, schedule, and repositoryRef are required"})
			return
		}

		job, err := s.BackupJobProvider.Create(c.Request.Context(), &req)
		if err != nil {
			log.Error().Err(err).Str("name", req.Name).Msg("failed to create backup job")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create backup job"})
			return
		}

		if s.Scheduler != nil {
			s.Scheduler.SyncJob(c.Request.Context(), job.Namespace, job.Name)
		}

		c.JSON(http.StatusCreated, job)
	}
}

func (s *Server) updateBackupJobHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		namespace := c.Param("namespace")
		name := c.Param("name")

		var req backupjob.UpdateBackupJobRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
			return
		}

		job, err := s.BackupJobProvider.Update(c.Request.Context(), namespace, name, &req)
		if err != nil {
			log.Error().Err(err).Str("namespace", namespace).Str("name", name).Msg("failed to update backup job")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update backup job"})
			return
		}

		if s.Scheduler != nil {
			s.Scheduler.SyncJob(c.Request.Context(), namespace, name)
		}

		c.JSON(http.StatusOK, job)
	}
}

func (s *Server) deleteBackupJobHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		namespace := c.Param("namespace")
		name := c.Param("name")

		if err := s.BackupJobProvider.Delete(c.Request.Context(), namespace, name); err != nil {
			log.Error().Err(err).Str("namespace", namespace).Str("name", name).Msg("failed to delete backup job")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete backup job"})
			return
		}

		if s.Scheduler != nil {
			s.Scheduler.RemoveJob(namespace, name)
		}

		c.Status(http.StatusNoContent)
	}
}

func (s *Server) triggerBackupJobHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		namespace := c.Param("namespace")
		name := c.Param("name")

		if s.Scheduler == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "scheduler not available"})
			return
		}

		if err := s.Scheduler.TriggerNow(c.Request.Context(), namespace, name); err != nil {
			log.Error().Err(err).Str("namespace", namespace).Str("name", name).Msg("failed to trigger backup job")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to trigger backup job"})
			return
		}

		c.Status(http.StatusAccepted)
	}
}

func (s *Server) listBackupRunsHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		namespace := c.Param("namespace")
		name := c.Param("name")

		if s.RunStore == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "run store not available"})
			return
		}

		runs, err := s.RunStore.ListRuns(c.Request.Context(), namespace, name)
		if err != nil {
			log.Error().Err(err).Str("namespace", namespace).Str("name", name).Msg("failed to list backup runs")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list backup runs"})
			return
		}

		c.JSON(http.StatusOK, runs)
	}
}
