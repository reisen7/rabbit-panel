package service

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"

	"rabbit-panel/repository"
)

// SshService 负责节点 SSH 凭证的安全存储与 SSH 会话建立。
// 密码以 AES-GCM 密文落库，密钥独立保存在 data 目录下（0600），
// 不依赖 JWT_SECRET/NODE_SECRET —— 那些密钥在每次启动时可能轮换，会导致已存凭证永久失效。
type SshService struct {
	repo repository.ISshRepository
	gcm  cipher.AEAD
}

// NewSshService 初始化 SSH 服务，加载或生成加密密钥
func NewSshService(repo repository.ISshRepository, dataDir string) (*SshService, error) {
	key, err := loadOrCreateSshKey(filepath.Join(dataDir, ".ssh_secret_key"))
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(mustAesCipher(key))
	if err != nil {
		return nil, err
	}
	return &SshService{repo: repo, gcm: gcm}, nil
}

// loadOrCreateSshKey 读取密钥文件；不存在则生成 32 字节随机密钥并落盘（0600）
func loadOrCreateSshKey(path string) ([]byte, error) {
	if data, err := os.ReadFile(path); err == nil {
		if key, err := base64.StdEncoding.DecodeString(stringTrimSpace(data)); err == nil && len(key) == 32 {
			return key, nil
		}
		return nil, fmt.Errorf("SSH 密钥文件损坏: %s", path)
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("生成 SSH 密钥失败: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, []byte(base64.StdEncoding.EncodeToString(key)), 0600); err != nil {
		return nil, fmt.Errorf("写入 SSH 密钥失败: %w", err)
	}
	return key, nil
}

func mustAesCipher(key []byte) cipher.Block {
	block, err := aes.NewCipher(key)
	if err != nil {
		panic("invalid aes key: " + err.Error())
	}
	return block
}

func stringTrimSpace(b []byte) string {
	start, end := 0, len(b)
	for start < end && (b[start] == ' ' || b[start] == '\n' || b[start] == '\r' || b[start] == '\t') {
		start++
	}
	for end > start && (b[end-1] == ' ' || b[end-1] == '\n' || b[end-1] == '\r' || b[end-1] == '\t') {
		end--
	}
	return string(b[start:end])
}

// encrypt 加密明文密码，nonce 前置拼接后 base64 编码
func (s *SshService) encrypt(plaintext string) (string, error) {
	nonce := make([]byte, s.gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(s.gcm.Seal(nonce, nonce, []byte(plaintext), nil)), nil
}

// decrypt 解密密文；兼容历史明文数据（解密失败时原样返回）
func (s *SshService) decrypt(ciphertext string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("SSH 密码数据损坏")
	}
	ns := s.gcm.NonceSize()
	if len(raw) < ns {
		return "", fmt.Errorf("SSH 密码数据损坏")
	}
	plain, err := s.gcm.Open(nil, raw[:ns], raw[ns:], nil)
	if err != nil {
		return "", fmt.Errorf("SSH 密码解密失败（密钥不匹配）")
	}
	return string(plain), nil
}

// ListCredentials 列出某节点下已保存的凭证（不含密码）
func (s *SshService) ListCredentials(nodeID string) ([]*repository.SshCredentialRecord, error) {
	return s.repo.ListSshCredentials(nodeID)
}

// SaveCredential 加密密码后保存凭证。
// 同一节点下 host+port+username 已存在时更新密码（幂等 upsert），避免重复保存同一用户。
func (s *SshService) SaveCredential(nodeID, host string, port int, username, password string) (*repository.SshCredentialRecord, error) {
	enc, err := s.encrypt(password)
	if err != nil {
		return nil, err
	}

	existing, err := s.repo.ListSshCredentials(nodeID)
	if err != nil {
		return nil, err
	}
	for _, rec := range existing {
		if rec.Host == host && rec.Port == port && rec.Username == username {
			if err := s.repo.UpdateSshCredentialPassword(rec.ID, enc); err != nil {
				return nil, err
			}
			rec.PasswordEnc = ""
			return rec, nil
		}
	}

	rec := &repository.SshCredentialRecord{
		NodeID:      nodeID,
		Host:        host,
		Port:        port,
		Username:    username,
		PasswordEnc: enc,
		CreatedAt:   time.Now(),
	}
	id, err := s.repo.CreateSshCredential(rec)
	if err != nil {
		return nil, err
	}
	rec.ID = id
	rec.PasswordEnc = ""
	return rec, nil
}

// DeleteCredential 删除凭证
func (s *SshService) DeleteCredential(id int64) error {
	return s.repo.DeleteSshCredential(id)
}

// CredentialForConnect 读取凭证并解密，供建立连接使用
func (s *SshService) CredentialForConnect(id int64) (*repository.SshCredentialRecord, string, error) {
	rec, err := s.repo.GetSshCredential(id)
	if err != nil {
		return nil, "", err
	}
	password, err := s.decrypt(rec.PasswordEnc)
	if err != nil {
		return nil, "", err
	}
	return rec, password, nil
}

// NewClient 使用用户名/密码建立 SSH 客户端连接。
// HostKeyCallback 使用 InsecureIgnoreHostKey：这是管理面板场景，主机多为内网动态节点，
// 若强制 known_hosts 会让用户每次换 IP 都被卡；认证仍走密码校验。
func (s *SshService) NewClient(host string, port int, username, password string) (*ssh.Client, error) {
	cfg := &ssh.ClientConfig{
		User:            username,
		Auth:            []ssh.AuthMethod{ssh.Password(password)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	return ssh.Dial("tcp", addr, cfg)
}

// === SFTP 文件管理 ===

// SftpEntry 远端文件条目
type SftpEntry struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	IsDir   bool   `json:"is_dir"`
	IsLink  bool   `json:"is_link"`
	Size    int64  `json:"size"`
	Mode    string `json:"mode"`
	ModTime string `json:"mod_time"`
}

// NewSftpClient 建立 SSH 连接并在其上开启 SFTP 子系统。
// 返回的 sftp.Client 与底层 ssh.Client 都需要关闭（先关 sftp 再关 ssh）。
func (s *SshService) NewSftpClient(host string, port int, username, password string) (*sftp.Client, *ssh.Client, error) {
	conn, err := s.NewClient(host, port, username, password)
	if err != nil {
		return nil, nil, err
	}
	cl, err := sftp.NewClient(conn)
	if err != nil {
		conn.Close()
		return nil, nil, fmt.Errorf("SFTP 子系统不可用: %w", err)
	}
	return cl, conn, nil
}

// SftpList 列出远端目录内容（符号链接解析一次，判断其指向是否为目录）
func (s *SshService) SftpList(cl *sftp.Client, dir string) ([]*SftpEntry, error) {
	if dir == "" {
		dir = "/"
	}
	infos, err := cl.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	entries := make([]*SftpEntry, 0, len(infos))
	for _, fi := range infos {
		p := path.Join(dir, fi.Name())
		isDir := fi.IsDir()
		isLink := fi.Mode()&os.ModeSymlink != 0
		if isLink && !isDir {
			if st, err := cl.Stat(p); err == nil {
				isDir = st.IsDir()
			}
		}
		entries = append(entries, &SftpEntry{
			Name:    fi.Name(),
			Path:    p,
			IsDir:   isDir,
			IsLink:  isLink,
			Size:    fi.Size(),
			Mode:    fi.Mode().String(),
			ModTime: fi.ModTime().Format("2006-01-02 15:04:05"),
		})
	}
	return entries, nil
}

const sftpMaxReadSize = 2 * 1024 * 1024

// SftpRead 读取远端文本文件（限 2MB，超出提示下载查看）
func (s *SshService) SftpRead(cl *sftp.Client, p string) (string, error) {
	st, err := cl.Stat(p)
	if err != nil {
		return "", err
	}
	if st.IsDir() {
		return "", fmt.Errorf("该路径是目录")
	}
	if st.Size() > sftpMaxReadSize {
		return "", fmt.Errorf("文件过大（>2MB），请下载后查看")
	}
	f, err := cl.Open(p)
	if err != nil {
		return "", err
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// SftpWrite 覆盖写入远端文本文件
func (s *SshService) SftpWrite(cl *sftp.Client, p, content string) error {
	f, err := cl.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_TRUNC)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write([]byte(content))
	return err
}

// SftpMkdir 递归创建远端目录
func (s *SshService) SftpMkdir(cl *sftp.Client, p string) error {
	return cl.MkdirAll(p)
}

// SftpDelete 删除远端文件/目录；目录递归删除（符号链接只删链接本身，不跟随）
func (s *SshService) SftpDelete(cl *sftp.Client, p string) error {
	st, err := cl.Lstat(p)
	if err != nil {
		return err
	}
	if !st.IsDir() {
		return cl.Remove(p)
	}
	children, err := cl.ReadDir(p)
	if err != nil {
		return err
	}
	for _, child := range children {
		if err := s.SftpDelete(cl, path.Join(p, child.Name())); err != nil {
			return err
		}
	}
	return cl.RemoveDirectory(p)
}

// SftpRename 重命名/移动远端文件（优先 PosixRename 覆盖语义，不支持时回退 Rename）
func (s *SshService) SftpRename(cl *sftp.Client, oldPath, newPath string) error {
	if err := cl.PosixRename(oldPath, newPath); err != nil {
		return cl.Rename(oldPath, newPath)
	}
	return nil
}

// SftpOpen 打开远端文件供下载，返回文件句柄与大小（调用方负责关闭）
func (s *SshService) SftpOpen(cl *sftp.Client, p string) (*sftp.File, int64, error) {
	st, err := cl.Stat(p)
	if err != nil {
		return nil, 0, err
	}
	if st.IsDir() {
		return nil, 0, fmt.Errorf("目录不能直接下载")
	}
	f, err := cl.Open(p)
	if err != nil {
		return nil, 0, err
	}
	return f, st.Size(), nil
}

// SftpUpload 把数据写入远端目录下的文件（覆盖已存在文件）
func (s *SshService) SftpUpload(cl *sftp.Client, dir, name string, data []byte) error {
	f, err := cl.OpenFile(path.Join(dir, name), os.O_WRONLY|os.O_CREATE|os.O_TRUNC)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(data)
	return err
}
