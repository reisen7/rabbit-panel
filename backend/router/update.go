package router

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

func (r *Router) handleUpdateCheck(c *gin.Context) {
	force := c.Query("force")
	result, err := r.app.UpdateService.Check(c.Request.Context(), force == "1" || force == "true")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

func (r *Router) handleUpdateRun(c *gin.Context) {
	log.Printf("system update requested by %s", c.GetString("username"))
	ctx, cancel := context.WithTimeout(c.Request.Context(), 20*time.Second)
	defer cancel()

	if err := r.app.UpdateService.PrepareUpdate(ctx); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "update package downloaded"})
}

func (r *Router) handleUpdateIgnore(c *gin.Context) {
	var reqBody struct {
		Version string `json:"version"`
	}
	if err := c.ShouldBindJSON(&reqBody); err != nil || reqBody.Version == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid version"})
		return
	}

	if err := r.app.UpdateService.IgnoreVersion(reqBody.Version); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "ignored version saved"})
}

func (r *Router) handleUpdateStatus(c *gin.Context) {
	status, err := r.app.UpdateService.GetTaskStatus()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, status)
}

func (r *Router) handleUpdateApply(c *gin.Context) {
	log.Printf("apply update requested by %s", c.GetString("username"))
	if err := r.app.UpdateService.ApplyPreparedUpdate(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "update apply started"})
}

func (r *Router) handleUpdateClearIgnore(c *gin.Context) {
	if err := r.app.UpdateService.ClearIgnoredVersion(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "ignored version cleared"})
}

func (r *Router) handleUpdateClearState(c *gin.Context) {
	if err := r.app.UpdateService.ClearUpdateState(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "update state cleared"})
}
