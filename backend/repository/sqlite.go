package repository

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"
)

// IAuthRepository 认证数据访问接口
type IAuthRepository interface {
	// User operations
	CreateUser(username, password string) error
	GetUser(username string) (*UserRecord, error)
	GetUserByID(id int) (*UserRecord, error)
	GetAllUsers() ([]*UserRecord, error)
	UpdatePassword(username, newPasswordHash string) error
	VerifyPassword(userID int, password string) error
	ChangePassword(username, oldPassword, newPassword string) error
	UserExists() (bool, error)

	// Session operations
	GetSession(token string) (*SessionRecord, bool)
	SetSession(token string, session *SessionRecord)
	DeleteSession(token string)

	// Chat history operations
	SaveChatMessage(role, content string) error
	GetChatHistory(limit int) ([]ChatHistoryRecord, error)
	CleanupOldMessages(olderThan time.Duration) (int64, error)
	ClearChatHistory() error
}

// UserRecord 用户记录
type UserRecord struct {
	ID                  int
	Username            string
	PasswordHash        string
	NeedChangePassword bool
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

// SessionRecord 会话记录
type SessionRecord struct {
	Username            string
	ExpiresAt           time.Time
	NeedChangePassword bool
}

// ChatHistoryRecord 聊天历史记录
type ChatHistoryRecord struct {
	ID        int64     `json:"id"`
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

// SQLiteRepository SQLite 数据库实现
type SQLiteRepository struct {
	db           *sql.DB
	sessions     map[string]*SessionRecord
	sessionMutex sync.RWMutex
}

// NewSQLiteRepository 创建 SQLite 仓库实例
func NewSQLiteRepository(dbPath string) (*SQLiteRepository, error) {
	// Ensure data directory exists
	dir := "./data"
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("创建数据目录失败: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}

	repo := &SQLiteRepository{
		db:       db,
		sessions: make(map[string]*SessionRecord),
	}

	if err := repo.initTables(); err != nil {
		db.Close()
		return nil, err
	}

	return repo, nil
}

// initTables 初始化数据库表
func (r *SQLiteRepository) initTables() error {
	// Users table
	_, err := r.db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			need_change_password INTEGER DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`)
	if err != nil {
		return fmt.Errorf("创建用户表失败: %w", err)
	}

	// Chat history table
	_, err = r.db.Exec(`
		CREATE TABLE IF NOT EXISTS chat_messages (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			role TEXT NOT NULL,
			content TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`)
	if err != nil {
		return fmt.Errorf("创建聊天记录表失败: %w", err)
	}

	// SSH credentials table
	_, err = r.db.Exec(`
		CREATE TABLE IF NOT EXISTS ssh_credentials (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			node_id TEXT NOT NULL,
			host TEXT NOT NULL,
			port INTEGER NOT NULL DEFAULT 22,
			username TEXT NOT NULL,
			password_enc TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`)
	if err != nil {
		return fmt.Errorf("创建 SSH 凭证表失败: %w", err)
	}

	// Check if default admin user exists
	var count int
	err = r.db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	if err != nil {
		return fmt.Errorf("查询用户数失败: %w", err)
	}

	if count == 0 {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte("admin"), bcrypt.DefaultCost)
		if err != nil {
			return fmt.Errorf("生成密码哈希失败: %w", err)
		}
		_, err = r.db.Exec(
			"INSERT INTO users (username, password_hash, need_change_password) VALUES (?, ?, ?)",
			"admin", string(hashedPassword), 1,
		)
		if err != nil {
			return fmt.Errorf("创建默认用户失败: %w", err)
		}
		log.Println("已创建默认管理员账户: admin/admin")
	}

	return nil
}

// === User Operations ===

// CreateUser 创建用户
func (r *SQLiteRepository) CreateUser(username, password string) error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("生成密码哈希失败: %w", err)
	}
	_, err = r.db.Exec(
		"INSERT INTO users (username, password_hash, need_change_password) VALUES (?, ?, ?)",
		username, string(hashedPassword), 0,
	)
	return err
}

// GetUser 获取用户
func (r *SQLiteRepository) GetUser(username string) (*UserRecord, error) {
	var user UserRecord
	var needChangePassword int
	err := r.db.QueryRow(
		"SELECT id, username, password_hash, need_change_password, created_at, updated_at FROM users WHERE username = ?",
		username,
	).Scan(&user.ID, &user.Username, &user.PasswordHash, &needChangePassword, &user.CreatedAt, &user.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	user.NeedChangePassword = needChangePassword == 1
	return &user, nil
}

// UpdatePassword 更新密码
func (r *SQLiteRepository) UpdatePassword(username, newPasswordHash string) error {
	_, err := r.db.Exec(
		"UPDATE users SET password_hash = ?, need_change_password = 0, updated_at = CURRENT_TIMESTAMP WHERE username = ?",
		newPasswordHash, username,
	)
	return err
}

// UserExists 检查用户是否存在
func (r *SQLiteRepository) UserExists() (bool, error) {
	var count int
	err := r.db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	return count > 0, err
}

// VerifyPassword 验证用户密码
func (r *SQLiteRepository) VerifyPassword(userID int, password string) error {
	var hash string
	err := r.db.QueryRow("SELECT password_hash FROM users WHERE id = ?", userID).Scan(&hash)
	if err != nil {
		return fmt.Errorf("用户不存在")
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

// ChangePassword 修改用户密码
func (r *SQLiteRepository) ChangePassword(username, oldPassword, newPassword string) error {
	// Get user
	user, err := r.GetUser(username)
	if err != nil || user == nil {
		return fmt.Errorf("用户不存在")
	}

	// Verify old password (if not first time)
	var needChangePassword bool
	var hash string
	err = r.db.QueryRow("SELECT password_hash, need_change_password FROM users WHERE username = ?", username).Scan(&hash, &needChangePassword)
	if err != nil {
		return fmt.Errorf("查询用户失败")
	}

	if !needChangePassword {
		err = bcrypt.CompareHashAndPassword([]byte(hash), []byte(oldPassword))
		if err != nil {
			return fmt.Errorf("旧密码错误")
		}
	}

	// Generate new hash
	newHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("生成密码哈希失败")
	}

	return r.UpdatePassword(username, string(newHash))
}

// GetUserByID 根据ID获取用户
func (r *SQLiteRepository) GetUserByID(id int) (*UserRecord, error) {
	var user UserRecord
	var needChangePassword int
	err := r.db.QueryRow(
		"SELECT id, username, password_hash, need_change_password, created_at, updated_at FROM users WHERE id = ?",
		id,
	).Scan(&user.ID, &user.Username, &user.PasswordHash, &needChangePassword, &user.CreatedAt, &user.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	user.NeedChangePassword = needChangePassword == 1
	return &user, nil
}

// GetAllUsers 获取所有用户
func (r *SQLiteRepository) GetAllUsers() ([]*UserRecord, error) {
	rows, err := r.db.Query("SELECT id, username, password_hash, need_change_password, created_at, updated_at FROM users")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*UserRecord
	for rows.Next() {
		var user UserRecord
		var needChangePassword int
		if err := rows.Scan(&user.ID, &user.Username, &user.PasswordHash, &needChangePassword, &user.CreatedAt, &user.UpdatedAt); err != nil {
			return nil, err
		}
		user.NeedChangePassword = needChangePassword == 1
		users = append(users, &user)
	}
	return users, nil
}

// === Session Operations ===

// GetSession 获取会话
func (r *SQLiteRepository) GetSession(token string) (*SessionRecord, bool) {
	r.sessionMutex.RLock()
	defer r.sessionMutex.RUnlock()
	session, exists := r.sessions[token]
	return session, exists
}

// SetSession 设置会话
func (r *SQLiteRepository) SetSession(token string, session *SessionRecord) {
	r.sessionMutex.Lock()
	r.sessions[token] = session
	r.sessionMutex.Unlock()
}

// DeleteSession 删除会话
func (r *SQLiteRepository) DeleteSession(token string) {
	r.sessionMutex.Lock()
	delete(r.sessions, token)
	r.sessionMutex.Unlock()
}

// === Chat History Operations ===

// SaveChatMessage 保存聊天消息
func (r *SQLiteRepository) SaveChatMessage(role, content string) error {
	_, err := r.db.Exec(
		"INSERT INTO chat_messages (role, content, created_at) VALUES (?, ?, ?)",
		role, content, time.Now(),
	)
	return err
}

// GetChatHistory 获取聊天历史
func (r *SQLiteRepository) GetChatHistory(limit int) ([]ChatHistoryRecord, error) {
	rows, err := r.db.Query(
		"SELECT id, role, content, created_at FROM chat_messages ORDER BY created_at ASC LIMIT ?",
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []ChatHistoryRecord
	for rows.Next() {
		var rec ChatHistoryRecord
		if err := rows.Scan(&rec.ID, &rec.Role, &rec.Content, &rec.CreatedAt); err != nil {
			return nil, err
		}
		records = append(records, rec)
	}
	return records, nil
}

// CleanupOldMessages 清理旧消息
func (r *SQLiteRepository) CleanupOldMessages(olderThan time.Duration) (int64, error) {
	cutoff := time.Now().Add(-olderThan)
	result, err := r.db.Exec("DELETE FROM chat_messages WHERE created_at < ?", cutoff)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// ClearChatHistory 清空聊天历史
func (r *SQLiteRepository) ClearChatHistory() error {
	_, err := r.db.Exec("DELETE FROM chat_messages")
	return err
}

// DB 返回底层数据库连接（用于初始化聊天历史表等）
func (r *SQLiteRepository) DB() *sql.DB {
	return r.db
}
