package router

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	ws "github.com/gorilla/websocket"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"

	"rabbit-panel/middleware"
	"rabbit-panel/repository"
)

// === SSH 凭证管理 ===

// handleSshCredentialsList GET /api/ssh/credentials?node_id=
// 返回某节点已保存的 SSH 用户列表（绝不含密码）
func (r *Router) handleSshCredentialsList(c *gin.Context) {
	nodeID := c.Query("node_id")
	if nodeID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "node_id 不能为空"})
		return
	}
	list, err := r.app.SshService.ListCredentials(nodeID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if list == nil {
		list = []*repository.SshCredentialRecord{}
	}
	c.JSON(http.StatusOK, list)
}

type sshCredentialCreateRequest struct {
	NodeID   string `json:"node_id" binding:"required"`
	Host     string `json:"host" binding:"required"`
	Port     int    `json:"port"`
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// handleSshCredentialCreate POST /api/ssh/credentials
// 保存一条 SSH 用户名/密码凭证（密码加密存储）
func (r *Router) handleSshCredentialCreate(c *gin.Context) {
	var req sshCredentialCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数不完整"})
		return
	}
	if req.Port <= 0 || req.Port > 65535 {
		req.Port = 22
	}
	rec, err := r.app.SshService.SaveCredential(req.NodeID, strings.TrimSpace(req.Host), req.Port, req.Username, req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, rec)
}

// handleSshCredentialDelete DELETE /api/ssh/credentials?id=
func (r *Router) handleSshCredentialDelete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Query("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id 无效"})
		return
	}
	if err := r.app.SshService.DeleteCredential(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已删除"})
}

// === SSH WebSocket 终端 ===

// sshConnectMessage 是 WS 建立后客户端发送的第一条 JSON 消息：
//   - credential_id 非零：使用已保存凭证（密码由服务端解密，不经客户端）
//   - 否则直接携带 host/port/username/password
//   - cols/rows 为初始终端尺寸
type sshConnectMessage struct {
	Type         string `json:"type"`
	CredentialID int64  `json:"credential_id"`
	Host         string `json:"host"`
	Port         int    `json:"port"`
	Username     string `json:"username"`
	Password     string `json:"password"`
	Cols         int    `json:"cols"`
	Rows         int    `json:"rows"`
}

// handleSshTerminalWS GET /api/ssh/terminal?token=
// 协议：升级 WS → 客户端先发 connect JSON → 服务端建立 SSH+PTY → 双向转发，
// 后续 {"type":"resize"} 调整窗口尺寸，其余消息作为 stdin。
func (r *Router) handleSshTerminalWS(c *gin.Context) {
	token := c.Query("token")
	if token == "" || !middleware.ValidateToken(token, r.app.JWTSecret) {
		c.String(http.StatusUnauthorized, "未授权")
		return
	}

	conn, err := ginWsUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	// 第一条消息必须是连接参数
	conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	mt, message, err := conn.ReadMessage()
	if err != nil || mt != ws.TextMessage {
		conn.WriteMessage(ws.TextMessage, []byte("\r\n连接参数超时或格式错误\r\n"))
		return
	}
	conn.SetReadDeadline(time.Time{})

	var req sshConnectMessage
	if err := json.Unmarshal(message, &req); err != nil || req.Type != "connect" {
		conn.WriteMessage(ws.TextMessage, []byte("\r\n连接参数格式错误\r\n"))
		return
	}

	host, port, user, password, err := r.resolveSshConnParams(req.CredentialID, req.Host, req.Port, req.Username, req.Password)
	if err != nil {
		conn.WriteMessage(ws.TextMessage, []byte("\r\nError: "+err.Error()+"\r\n"))
		return
	}
	if req.Cols <= 0 {
		req.Cols = 80
	}
	if req.Rows <= 0 {
		req.Rows = 24
	}

	conn.WriteMessage(ws.TextMessage, []byte(fmt.Sprintf("连接 %s@%s:%d ...\r\n", user, host, port)))

	client, err := r.app.SshService.NewClient(host, port, user, password)
	if err != nil {
		conn.WriteMessage(ws.TextMessage, []byte("\r\nError: "+sshFriendlyError(err)+"\r\n"))
		return
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		conn.WriteMessage(ws.TextMessage, []byte("\r\nError: "+err.Error()+"\r\n"))
		return
	}
	defer session.Close()

	stdin, err := session.StdinPipe()
	if err != nil {
		conn.WriteMessage(ws.TextMessage, []byte("\r\nError: "+err.Error()+"\r\n"))
		return
	}
	stdout, err := session.StdoutPipe()
	if err != nil {
		conn.WriteMessage(ws.TextMessage, []byte("\r\nError: "+err.Error()+"\r\n"))
		return
	}
	stderr, err := session.StderrPipe()
	if err != nil {
		conn.WriteMessage(ws.TextMessage, []byte("\r\nError: "+err.Error()+"\r\n"))
		return
	}

	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
	if err := session.RequestPty("xterm-256color", req.Rows, req.Cols, modes); err != nil {
		conn.WriteMessage(ws.TextMessage, []byte("\r\nError: "+err.Error()+"\r\n"))
		return
	}
	if err := session.Shell(); err != nil {
		conn.WriteMessage(ws.TextMessage, []byte("\r\nError: "+err.Error()+"\r\n"))
		return
	}
	conn.WriteMessage(ws.TextMessage, []byte("\x1b[32m已连接\x1b[0m\r\n\r\n"))

	done := make(chan struct{})
	var writeMu sync.Mutex
	sendMsg := func(mt int, data []byte) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		return conn.WriteMessage(mt, data)
	}
	var closeOnce = make(chan struct{}, 1)
	finish := func() {
		select {
		case closeOnce <- struct{}{}:
			client.Close()
			conn.Close()
		default:
		}
	}

	// SSH stdout/stderr → WebSocket（BinaryMessage）
	for _, src := range []interface{ Read([]byte) (int, error) }{stdout, stderr} {
		go func(rd interface{ Read([]byte) (int, error) }) {
			buf := make([]byte, 4096)
			for {
				n, err := rd.Read(buf)
				if n > 0 {
					if werr := sendMsg(ws.BinaryMessage, buf[:n]); werr != nil {
						finish()
						return
					}
				}
				if err != nil {
					finish()
					return
				}
			}
		}(src)
	}

	// WebSocket → SSH stdin（JSON resize 消息拦截处理）
	go func() {
		defer close(done)
		for {
			mt, message, err := conn.ReadMessage()
			if err != nil {
				if ws.IsUnexpectedCloseError(err, ws.CloseGoingAway, ws.CloseAbnormalClosure, ws.CloseNoStatusReceived) {
					log.Printf("[SSH] WebSocket read error: %v", err)
				}
				finish()
				return
			}
			if mt == ws.TextMessage && len(message) > 0 && message[0] == '{' {
				var resize struct {
					Type string `json:"type"`
					Cols int    `json:"cols"`
					Rows int    `json:"rows"`
				}
				if json.Unmarshal(message, &resize) == nil && resize.Type == "resize" {
					if resize.Cols > 0 && resize.Rows > 0 {
						session.WindowChange(resize.Rows, resize.Cols)
					}
					continue
				}
			}
			if _, err := stdin.Write(message); err != nil {
				finish()
				return
			}
		}
	}()

	<-done
}

// resolveSshConnParams 统一解析 SSH 连接参数：
// credential_id > 0 时读已保存凭证并解密密码；否则校验 host/user 并规范化端口。
// WS 终端与 SFTP 接口共用。
func (r *Router) resolveSshConnParams(credID int64, host string, port int, user, password string) (string, int, string, string, error) {
	if credID > 0 {
		rec, plain, err := r.app.SshService.CredentialForConnect(credID)
		if err != nil {
			return "", 0, "", "", err
		}
		return rec.Host, rec.Port, rec.Username, plain, nil
	}
	host = strings.TrimSpace(host)
	if host == "" || user == "" {
		return "", 0, "", "", fmt.Errorf("主机与用户名不能为空")
	}
	// host 自带 "host:port" 时拆开，避免 JoinHostPort 歧义
	if h, p, err := net.SplitHostPort(host); err == nil && p != "" {
		if pp, perr := strconv.Atoi(p); perr == nil && pp > 0 && pp <= 65535 {
			host, port = h, pp
		} else {
			host = h
		}
	}
	if port <= 0 || port > 65535 {
		port = 22
	}
	return host, port, user, password, nil
}

// === SFTP 文件管理 ===

// sshSftpRequest SFTP 请求体：连接参数（凭证ID或明文参数）+ 操作字段
type sshSftpRequest struct {
	CredentialID int64  `json:"credential_id"`
	Host         string `json:"host"`
	Port         int    `json:"port"`
	Username     string `json:"username"`
	Password     string `json:"password"`
	Path         string `json:"path"`
	NewPath      string `json:"new_path"`
	Name         string `json:"name"`
	Content      string `json:"content"`
}

// openSftp 解析请求体与连接参数，建立 SSH+SFTP 会话。
// 失败时已写出错误响应，返回 ok=false；成功时调用方负责 defer 关闭两个 client。
func (r *Router) openSftp(c *gin.Context) (cl *sftp.Client, conn *ssh.Client, req *sshSftpRequest, ok bool) {
	var body sshSftpRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数不完整"})
		return nil, nil, nil, false
	}
	host, port, user, password, err := r.resolveSshConnParams(body.CredentialID, body.Host, body.Port, body.Username, body.Password)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return nil, nil, nil, false
	}
	cl, conn, err = r.app.SshService.NewSftpClient(host, port, user, password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": sshFriendlyError(err)})
		return nil, nil, nil, false
	}
	return cl, conn, &body, true
}

func sftpErr(c *gin.Context, err error) {
	c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
}

// handleSshSftpList POST /api/ssh/sftp/list  列目录
func (r *Router) handleSshSftpList(c *gin.Context) {
	cl, conn, req, ok := r.openSftp(c)
	if !ok {
		return
	}
	defer conn.Close()
	defer cl.Close()
	entries, err := r.app.SshService.SftpList(cl, req.Path)
	if err != nil {
		sftpErr(c, err)
		return
	}
	c.JSON(http.StatusOK, entries)
}

// handleSshSftpRead POST /api/ssh/sftp/read  读文本文件（限 2MB）
func (r *Router) handleSshSftpRead(c *gin.Context) {
	cl, conn, req, ok := r.openSftp(c)
	if !ok {
		return
	}
	defer conn.Close()
	defer cl.Close()
	content, err := r.app.SshService.SftpRead(cl, req.Path)
	if err != nil {
		sftpErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"content": content})
}

// handleSshSftpWrite POST /api/ssh/sftp/write  覆盖写文本文件
func (r *Router) handleSshSftpWrite(c *gin.Context) {
	cl, conn, req, ok := r.openSftp(c)
	if !ok {
		return
	}
	defer conn.Close()
	defer cl.Close()
	if req.Path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "path 不能为空"})
		return
	}
	if err := r.app.SshService.SftpWrite(cl, req.Path, req.Content); err != nil {
		sftpErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已保存"})
}

// handleSshSftpMkdir POST /api/ssh/sftp/mkdir  递归创建目录
func (r *Router) handleSshSftpMkdir(c *gin.Context) {
	cl, conn, req, ok := r.openSftp(c)
	if !ok {
		return
	}
	defer conn.Close()
	defer cl.Close()
	if req.Path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "path 不能为空"})
		return
	}
	if err := r.app.SshService.SftpMkdir(cl, req.Path); err != nil {
		sftpErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已创建"})
}

// handleSshSftpDelete POST /api/ssh/sftp/delete  删除文件/目录（目录递归）
func (r *Router) handleSshSftpDelete(c *gin.Context) {
	cl, conn, req, ok := r.openSftp(c)
	if !ok {
		return
	}
	defer conn.Close()
	defer cl.Close()
	if req.Path == "" || req.Path == "/" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "path 无效"})
		return
	}
	if err := r.app.SshService.SftpDelete(cl, req.Path); err != nil {
		sftpErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已删除"})
}

// handleSshSftpRename POST /api/ssh/sftp/rename  重命名/移动
func (r *Router) handleSshSftpRename(c *gin.Context) {
	cl, conn, req, ok := r.openSftp(c)
	if !ok {
		return
	}
	defer conn.Close()
	defer cl.Close()
	if req.Path == "" || req.NewPath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "path 与 new_path 不能为空"})
		return
	}
	if err := r.app.SshService.SftpRename(cl, req.Path, req.NewPath); err != nil {
		sftpErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已重命名"})
}

// handleSshSftpDownload POST /api/ssh/sftp/download  流式下载文件
func (r *Router) handleSshSftpDownload(c *gin.Context) {
	cl, conn, req, ok := r.openSftp(c)
	if !ok {
		return
	}
	defer conn.Close()
	defer cl.Close()
	f, size, err := r.app.SshService.SftpOpen(cl, req.Path)
	if err != nil {
		sftpErr(c, err)
		return
	}
	defer f.Close()
	filename := path.Base(req.Path)
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename*=UTF-8''%s", url.QueryEscape(filename)))
	c.Header("Content-Length", strconv.FormatInt(size, 10))
	c.DataFromReader(http.StatusOK, size, "application/octet-stream", f, nil)
}

// handleSshSftpUpload POST /api/ssh/sftp/upload  上传文件（body content 为 base64）
func (r *Router) handleSshSftpUpload(c *gin.Context) {
	cl, conn, req, ok := r.openSftp(c)
	if !ok {
		return
	}
	defer conn.Close()
	defer cl.Close()
	if req.Path == "" || req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "path 与 name 不能为空"})
		return
	}
	data, err := base64.StdEncoding.DecodeString(req.Content)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "文件内容解码失败"})
		return
	}
	if err := r.app.SshService.SftpUpload(cl, req.Path, req.Name, data); err != nil {
		sftpErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已上传"})
}
func sshFriendlyError(err error) string {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "unable to authenticate"):
		return "认证失败：用户名或密码错误"
	case strings.Contains(msg, "connection refused"):
		return "连接被拒绝：请确认目标主机的 SSH 服务已启动且端口正确"
	case strings.Contains(msg, "i/o timeout"), strings.Contains(msg, "timed out"):
		return "连接超时：目标主机不可达或端口不通"
	case strings.Contains(msg, "no route to host"):
		return "无法到达目标主机（no route to host）"
	default:
		return msg
	}
}
