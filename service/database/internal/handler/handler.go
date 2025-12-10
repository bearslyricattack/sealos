package handler

import (
	"fmt"
	"net/http"

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

// HandleQuery 处理数据库查询请求
func (h *DatabaseHandler) HandleQuery(c *gin.Context) {
	// 1. 解析请求
	req, err := h.parseRequest(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请求参数解析失败",
			"error":   err.Error(),
		})
		return
	}

	// 2. 验证请求
	if err := h.validateRequest(req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请求参数验证失败",
			"error":   err.Error(),
		})
		return
	}

	// 3. 通过服务层执行查询
	result, err := h.service.ExecuteQuery(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "查询执行失败",
			"error":   err.Error(),
		})
		return
	}

	// 4. 返回成功响应
	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "查询成功",
		"data":    result,
	})
}

// parseRequest extracts request parameters from context
func (h *DatabaseHandler) parseRequest(c *gin.Context) (*api.DatabaseRequest, error) {
	req := &api.DatabaseRequest{}
	req.Namespace = getParam(c, "namespace")
	req.Type = getParam(c, "type")
	req.Query = getParam(c, "query")
	req.Cluster = getParam(c, "app")
	req.Range.Start = getParam(c, "start")
	req.Range.End = getParam(c, "end")
	req.Range.Step = getParam(c, "step")
	req.Range.Time = getParam(c, "time")

	return req, nil
}

func getParam(c *gin.Context, key string) string {
	if value := c.Query(key); value != "" {
		return value
	}
	return c.PostForm(key)
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
