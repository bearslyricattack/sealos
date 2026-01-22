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

func (h *LaunchpadHandler) HandleQuery(c *gin.Context) {
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
	if prettyJSON, err := json.MarshalIndent(res, "", "  "); err == nil {
		log.Printf("格式化结果:\n%s", string(prettyJSON))
	}
	c.JSON(http.StatusOK, res)
}

func (h *LaunchpadHandler) parseRequest(c *gin.Context) (*api.LaunchpadRequest, error) {
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
	return req, nil
}

func getParam(c *gin.Context, key string) string {
	var value string
	if v := c.Query(key); v != "" {
		value = v
		return value
	}
	if v := c.PostForm(key); v != "" {
		value = v
		return value
	}
	return ""
}

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
	return nil
}
