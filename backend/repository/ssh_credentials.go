package repository

import (
	"fmt"
	"time"
)

// SshCredentialRecord SSH 连接凭证。PasswordEnc 存放密文，接口返回时永远不外泄。
type SshCredentialRecord struct {
	ID          int64     `json:"id"`
	NodeID      string    `json:"node_id"`
	Host        string    `json:"host"`
	Port        int       `json:"port"`
	Username    string    `json:"username"`
	PasswordEnc string    `json:"-"`
	CreatedAt   time.Time `json:"created_at"`
}

// ISshRepository SSH 凭证数据访问接口
type ISshRepository interface {
	ListSshCredentials(nodeID string) ([]*SshCredentialRecord, error)
	GetSshCredential(id int64) (*SshCredentialRecord, error)
	CreateSshCredential(rec *SshCredentialRecord) (int64, error)
	UpdateSshCredentialPassword(id int64, passwordEnc string) error
	DeleteSshCredential(id int64) error
}

// ListSshCredentials 按节点列出已保存的 SSH 凭证（不含密码）
func (r *SQLiteRepository) ListSshCredentials(nodeID string) ([]*SshCredentialRecord, error) {
	rows, err := r.db.Query(
		`SELECT id, node_id, host, port, username, password_enc, created_at
		 FROM ssh_credentials WHERE node_id = ? ORDER BY id DESC`, nodeID)
	if err != nil {
		return nil, fmt.Errorf("查询 SSH 凭证失败: %w", err)
	}
	defer rows.Close()

	list := make([]*SshCredentialRecord, 0)
	for rows.Next() {
		rec := &SshCredentialRecord{}
		if err := rows.Scan(&rec.ID, &rec.NodeID, &rec.Host, &rec.Port, &rec.Username, &rec.PasswordEnc, &rec.CreatedAt); err != nil {
			return nil, fmt.Errorf("读取 SSH 凭证失败: %w", err)
		}
		list = append(list, rec)
	}
	return list, rows.Err()
}

// GetSshCredential 读取单条凭证（含密文，仅用于建立连接）
func (r *SQLiteRepository) GetSshCredential(id int64) (*SshCredentialRecord, error) {
	rec := &SshCredentialRecord{}
	err := r.db.QueryRow(
		`SELECT id, node_id, host, port, username, password_enc, created_at
		 FROM ssh_credentials WHERE id = ?`, id).
		Scan(&rec.ID, &rec.NodeID, &rec.Host, &rec.Port, &rec.Username, &rec.PasswordEnc, &rec.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("查询 SSH 凭证失败: %w", err)
	}
	return rec, nil
}

// CreateSshCredential 保存一条凭证，返回新 ID
func (r *SQLiteRepository) CreateSshCredential(rec *SshCredentialRecord) (int64, error) {
	result, err := r.db.Exec(
		`INSERT INTO ssh_credentials (node_id, host, port, username, password_enc) VALUES (?, ?, ?, ?, ?)`,
		rec.NodeID, rec.Host, rec.Port, rec.Username, rec.PasswordEnc)
	if err != nil {
		return 0, fmt.Errorf("保存 SSH 凭证失败: %w", err)
	}
	return result.LastInsertId()
}

// UpdateSshCredentialPassword 更新已存在凭证的密码密文
func (r *SQLiteRepository) UpdateSshCredentialPassword(id int64, passwordEnc string) error {
	result, err := r.db.Exec(`UPDATE ssh_credentials SET password_enc = ? WHERE id = ?`, passwordEnc, id)
	if err != nil {
		return fmt.Errorf("更新 SSH 凭证密码失败: %w", err)
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return fmt.Errorf("SSH 凭证不存在")
	}
	return nil
}

// DeleteSshCredential 删除一条凭证
func (r *SQLiteRepository) DeleteSshCredential(id int64) error {
	result, err := r.db.Exec(`DELETE FROM ssh_credentials WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("删除 SSH 凭证失败: %w", err)
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return fmt.Errorf("SSH 凭证不存在")
	}
	return nil
}
