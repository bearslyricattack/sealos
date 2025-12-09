package handler

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/labring/sealos/service/database/internal/service"
	"github.com/labring/sealos/service/pkg/api"
)

// DatabaseHandler handles HTTP requests for database metrics
type DatabaseHandler struct {
	service *service.DatabaseService
}

// NewDatabaseHandler creates a new DatabaseHandler
func NewDatabaseHandler(service *service.DatabaseService) *DatabaseHandler {
	return &DatabaseHandler{
		service: service,
	}
}

// HandleQuery handles database query requests
func (h *DatabaseHandler) HandleQuery(c *gin.Context) (interface{}, error) {
	// Parse request
	req, err := h.parseRequest(c)
	if err != nil {
		return nil, err
	}

	// Validate request
	if err := h.validateRequest(req); err != nil {
		return nil, err
	}

	// Execute query via service layer
	return h.service.ExecuteQuery(c.Request.Context(), req)
}

// parseRequest extracts request parameters from context
func (h *DatabaseHandler) parseRequest(c *gin.Context) (*api.DatabaseRequest, error) {
	req := &api.DatabaseRequest{}

	// Bind form parameters
	req.Namespace = c.PostForm("namespace")
	req.Type = c.PostForm("type")
	req.Query = c.PostForm("query")
	req.Cluster = c.PostForm("app")
	req.Range.Start = c.PostForm("start")
	req.Range.End = c.PostForm("end")
	req.Range.Step = c.PostForm("step")
	req.Range.Time = c.PostForm("time")

	return req, nil
}

// validateRequest validates required fields
func (h *DatabaseHandler) validateRequest(req *api.DatabaseRequest) error {
	if req.Namespace == "" {
		return fmt.Errorf("namespace is required")
	}
	if req.Type == "" {
		return fmt.Errorf("type (database type) is required")
	}
	if req.Query == "" {
		return fmt.Errorf("query is required")
	}
	return nil
}
