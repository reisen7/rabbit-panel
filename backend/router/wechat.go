package router

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (r *Router) handleWeChatStatus(c *gin.Context) {
	if r.app.WeChatService == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "微信通道只在主节点可用"})
		return
	}
	c.JSON(http.StatusOK, r.app.WeChatService.Status())
}

func (r *Router) handleWeChatQRCode(c *gin.Context) {
	if r.app.WeChatService == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "微信通道只在主节点可用"})
		return
	}
	qr, err := r.app.WeChatService.StartQR(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, qr)
}

func (r *Router) handleWeChatQRStatus(c *gin.Context) {
	if r.app.WeChatService == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "微信通道只在主节点可用"})
		return
	}
	c.JSON(http.StatusOK, r.app.WeChatService.QR())
}

func (r *Router) handleWeChatVerify(c *gin.Context) {
	if r.app.WeChatService == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "微信通道只在主节点可用"})
		return
	}
	var req struct {
		Code string `json:"code"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}
	if err := r.app.WeChatService.SubmitVerify(req.Code); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "success"})
}

func (r *Router) handleWeChatUnbind(c *gin.Context) {
	if r.app.WeChatService == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "微信通道只在主节点可用"})
		return
	}
	if err := r.app.WeChatService.Unbind(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "success"})
}
