package handler

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/labring/sealos/service/launchpad/internal/service"
	"github.com/labring/sealos/service/pkg/api"
)

// LaunchpadHandler handles HTTP requests for launchpad metrics
type LaunchpadHandler struct {
	service *service.LaunchpadService
}

// NewLaunchpadHandler creates a new LaunchpadHandler
func NewLaunchpadHandler(service *service.LaunchpadService) *LaunchpadHandler {
	return &LaunchpadHandler{
		service: service,
	}
}

// HandleQuery 处理launchpad查询请求
func (h *LaunchpadHandler) HandleQuery(c *gin.Context) {
	log.Printf("\n========================================")
	log.Printf("=== 收到Launchpad查询请求 ===")
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
		log.Printf("Request details: Namespace=%s, LaunchPadName=%s, Type=%s, PvcName=%s",
			req.Namespace, req.LaunchPadName, req.Type, req.PvcName)
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请求参数验证失败",
			"error":   err.Error(),
			"debug": gin.H{
				"namespace":     req.Namespace,
				"launchPadName": req.LaunchPadName,
				"type":          req.Type,
				"pvcName":       req.PvcName,
			},
		})
		return
	}
	log.Printf("✅ 请求验证成功")

	log.Printf("🔍 执行查询: namespace=%s, launchPadName=%s, type=%s, pvcName=%s", req.Namespace, req.LaunchPadName, req.Type, req.PvcName)
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

	var res api.QueryResult

	if err := json.Unmarshal(result, &res); err != nil {
		log.Printf("❌ 解析失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "数据解析失败",
			"error":   err.Error(),
		})
		return
	}
	totalPoints := 0
	for _, series := range res.Data.Result {
		if res.Data.ResultType == "vector" {
			totalPoints++
		} else {
			totalPoints += len(series.Values)
		}
	}
	log.Printf("✅ 查询成功: type=%s, series=%d, points=%d", res.Data.ResultType, len(res.Data.Result), totalPoints)
	c.JSON(http.StatusOK, res)
	log.Printf("=== 请求处理完成 ===\n")
}

func (h *LaunchpadHandler) parseRequest(c *gin.Context) (*api.LaunchpadRequest, error) {
	log.Printf("--- 解析请求参数 ---")

	req := &api.LaunchpadRequest{}
	req.Namespace = getParam(c, "namespace")
	req.LaunchPadName = getParam(c, "launchPadName")
	req.Type = getParam(c, "type")
	req.Service = getParam(c, "service")
	req.Port = getParam(c, "port")
	req.PvcName = getParam(c, "pvcName")
	req.Range.Start = getParam(c, "start")
	req.Range.End = getParam(c, "end")
	req.Range.Step = getParam(c, "step")
	req.Range.Time = getParam(c, "time")

	log.Printf("解析结果:")
	log.Printf("  Namespace: '%s' (empty: %v)", req.Namespace, req.Namespace == "")
	log.Printf("  LaunchPadName: '%s' (empty: %v)", req.LaunchPadName, req.LaunchPadName == "")
	log.Printf("  Type: '%s' (empty: %v)", req.Type, req.Type == "")
	log.Printf("  PvcName: '%s' (empty: %v)", req.PvcName, req.PvcName == "")
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
func (h *LaunchpadHandler) validateRequest(req *api.LaunchpadRequest) error {
	log.Printf("--- 验证请求参数 ---")

	if req.Namespace == "" {
		log.Printf("❌ 验证失败: namespace 为空")
		return fmt.Errorf("namespace is required")
	}
	log.Printf("✓ namespace: %s", req.Namespace)

	if req.Type == "" {
		log.Printf("❌ 验证失败: type 为空")
		return fmt.Errorf("type is required")
	}
	log.Printf("✓ type: %s", req.Type)
	// PvcName is only required for storage queries
	if req.Type == "storage" && req.PvcName == "" {
		log.Printf("❌ 验证失败: storage查询需要pvcName")
		return fmt.Errorf("pvcName is required for storage queries")
	}
	log.Printf("✅ 所有必需参数验证通过")
	return nil
}
