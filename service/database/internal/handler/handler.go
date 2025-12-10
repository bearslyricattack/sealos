package handler

import (
	"fmt"
	"log"
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
	log.Printf("\n========================================")
	log.Printf("=== 收到查询请求 ===")
	log.Printf("Method: %s", c.Request.Method)
	log.Printf("URL: %s", c.Request.URL.String())
	log.Printf("Query Params: %v", c.Request.URL.Query())
	log.Printf("========================================\n")

	// 1. 解析请求
	req, err := h.parseRequest(c)
	if err != nil {
		log.Printf("❌ 请求解析失败: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请求参数解析失败",
			"error":   err.Error(),
		})
		return
	}
	log.Printf("✅ 请求解析成功")

	// 2. 验证请求
	if err := h.validateRequest(req); err != nil {
		log.Printf("❌ 请求验证失败: %v", err)
		log.Printf("Request details: Namespace=%s, Type=%s, Query=%s, Cluster=%s",
			req.Namespace, req.Type, req.Query, req.Cluster)
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请求参数验证失败",
			"error":   err.Error(),
			"debug": gin.H{ // 添加调试信息
				"namespace": req.Namespace,
				"type":      req.Type,
				"query":     req.Query,
				"cluster":   req.Cluster,
			},
		})
		return
	}
	log.Printf("✅ 请求验证成功")

	// 3. 通过服务层执行查询
	log.Printf("🔍 执行查询: namespace=%s, type=%s, query=%s, cluster=%s",
		req.Namespace, req.Type, req.Query, req.Cluster)
	result, err := h.service.ExecuteQuery(c.Request.Context(), req)
	if err != nil {
		log.Printf("❌ 查询执行失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "查询执行失败",
			"error":   err.Error(),
		})
		return
	}
	log.Printf("✅ 查询执行成功")

	// 4. 返回成功响应
	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "查询成功",
		"data":    result,
	})
	log.Printf("=== 请求处理完成 ===\n")
}

// parseRequest extracts request parameters from context
func (h *DatabaseHandler) parseRequest(c *gin.Context) (*api.DatabaseRequest, error) {
	log.Printf("--- 解析请求参数 ---")

	req := &api.DatabaseRequest{}
	req.Namespace = getParam(c, "namespace")
	req.Type = getParam(c, "type")
	req.Query = getParam(c, "query")
	req.Cluster = getParam(c, "app")
	req.Range.Start = getParam(c, "start")
	req.Range.End = getParam(c, "end")
	req.Range.Step = getParam(c, "step")
	req.Range.Time = getParam(c, "time")

	log.Printf("解析结果:")
	log.Printf("  Namespace: '%s' (empty: %v)", req.Namespace, req.Namespace == "")
	log.Printf("  Type: '%s' (empty: %v)", req.Type, req.Type == "")
	log.Printf("  Query: '%s' (empty: %v)", req.Query, req.Query == "")
	log.Printf("  Cluster: '%s' (empty: %v)", req.Cluster, req.Cluster == "")
	log.Printf("  Range.Start: '%s'", req.Range.Start)
	log.Printf("  Range.End: '%s'", req.Range.End)
	log.Printf("  Range.Step: '%s'", req.Range.Step)

	return req, nil
}

func getParam(c *gin.Context, key string) string {
	var value string

	// 尝试从 Query 获取
	if v := c.Query(key); v != "" {
		value = v
		log.Printf("  getParam('%s') from Query = '%s'", key, value)
		return value
	}

	// 尝试从 PostForm 获取
	if v := c.PostForm(key); v != "" {
		value = v
		log.Printf("  getParam('%s') from PostForm = '%s'", key, value)
		return value
	}

	log.Printf("  getParam('%s') = '' (not found)", key)
	return ""
}

// validateRequest validates required fields
func (h *DatabaseHandler) validateRequest(req *api.DatabaseRequest) error {
	log.Printf("--- 验证请求参数 ---")

	if req.Namespace == "" {
		log.Printf("❌ 验证失败: namespace 为空")
		return fmt.Errorf("namespace is required")
	}
	log.Printf("✓ namespace: %s", req.Namespace)

	if req.Type == "" {
		log.Printf("❌ 验证失败: type 为空")
		return fmt.Errorf("type (database type) is required")
	}
	log.Printf("✓ type: %s", req.Type)

	if req.Query == "" {
		log.Printf("❌ 验证失败: query 为空")
		return fmt.Errorf("query is required")
	}
	log.Printf("✓ query: %s", req.Query)

	log.Printf("✅ 所有必需参数验证通过")
	return nil
}
