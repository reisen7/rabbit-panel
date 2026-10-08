package repository

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// IFileRepository 文件系统操作接口
type IFileRepository interface {
	// Compose projects
	EnsureComposeDir() error
	ListComposeProjects() ([]string, error)
	GetComposeProjectDir(name string) string
	ComposeProjectExists(name string) bool
	CreateComposeProject(name, content string) error
	GetComposeFile(name string) (string, error)
	SaveComposeFile(name, content string) error
	DeleteComposeProject(name string) error

	// Registries
	LoadRegistries() (map[string]*RegistryRecord, error)
	SaveRegistries(registries map[string]*RegistryRecord) error

	// System update settings
	LoadUpdateSettings() (*UpdateSettingsRecord, error)
	SaveUpdateSettings(settings *UpdateSettingsRecord) error
}

// RegistryRecord 仓库记录
type RegistryRecord struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	URL       string `json:"url"`
	Username  string `json:"username"`
	Password  string `json:"password,omitempty"`
	IsDefault bool   `json:"isDefault"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

// UpdateSettingsRecord stores persisted update state.
type UpdateSettingsRecord struct {
	IgnoredVersion   string `json:"ignored_version"`
	LastCheckTime    string `json:"last_check_time"`
	LastUpdateTime   string `json:"last_update_time"`
	LastUpdateStatus string `json:"last_update_status"`
	LastUpdateError  string `json:"last_update_error"`
	PreparedBinaryPath string `json:"prepared_binary_path"`
	PreparedVersion    string `json:"prepared_version"`
}

// FileRepository 文件系统实现
type FileRepository struct {
	composeBaseDir string
	registriesFile string
	updateFile     string
	registries     map[string]*RegistryRecord
	registriesLock sync.RWMutex
	updateSettings *UpdateSettingsRecord
	updateLock     sync.RWMutex
}

// NewFileRepository 创建文件仓库实例
func NewFileRepository() *FileRepository {
	return &FileRepository{
		composeBaseDir: "./compose_projects",
		registriesFile: "./data/registries.json",
		updateFile:     "./data/system_update.json",
	}
}

// === Compose Project Operations ===

// EnsureComposeDir 确保 Compose 目录存在
func (r *FileRepository) EnsureComposeDir() error {
	return os.MkdirAll(r.composeBaseDir, 0755)
}

// ListComposeProjects 列出 Compose 项目
func (r *FileRepository) ListComposeProjects() ([]string, error) {
	entries, err := os.ReadDir(r.composeBaseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}

	var projects []string
	for _, entry := range entries {
		if entry.IsDir() {
			projects = append(projects, entry.Name())
		}
	}
	return projects, nil
}

// GetComposeProjectDir 获取项目目录路径
func (r *FileRepository) GetComposeProjectDir(name string) string {
	return filepath.Join(r.composeBaseDir, name)
}

// ComposeProjectExists 检查项目是否存在
func (r *FileRepository) ComposeProjectExists(name string) bool {
	path := r.GetComposeProjectDir(name)
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// CreateComposeProject 创建 Compose 项目
func (r *FileRepository) CreateComposeProject(name, content string) error {
	dir := r.GetComposeProjectDir(name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	ymlPath := filepath.Join(dir, "docker-compose.yml")
	yamlPath := filepath.Join(dir, "docker-compose.yaml")
	if _, err := os.Stat(ymlPath); err == nil {
		return nil
	}
	if _, err := os.Stat(yamlPath); err == nil {
		return nil
	}

	return os.WriteFile(ymlPath, []byte(content), 0644)
}

// GetComposeFile 获取 Compose 文件内容
func (r *FileRepository) GetComposeFile(name string) (string, error) {
	dir := r.GetComposeProjectDir(name)

	// 尝试 .yml 或 .yaml
	filePath := filepath.Join(dir, "docker-compose.yml")
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		filePath = filepath.Join(dir, "docker-compose.yaml")
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// SaveComposeFile 保存 Compose 文件
func (r *FileRepository) SaveComposeFile(name, content string) error {
	dir := r.GetComposeProjectDir(name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	filePath := filepath.Join(dir, "docker-compose.yml")
	return os.WriteFile(filePath, []byte(content), 0644)
}

// DeleteComposeProject 删除 Compose 项目
func (r *FileRepository) DeleteComposeProject(name string) error {
	dir := r.GetComposeProjectDir(name)
	return os.RemoveAll(dir)
}

// === Registry Operations ===

// LoadRegistries 加载仓库列表
func (r *FileRepository) LoadRegistries() (map[string]*RegistryRecord, error) {
	r.registriesLock.RLock()
	if r.registries != nil {
		defer r.registriesLock.RUnlock()
		return r.registries, nil
	}
	r.registriesLock.RUnlock()

	// Ensure data directory exists
	dir := filepath.Dir(r.registriesFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	data, err := os.ReadFile(r.registriesFile)
	if err != nil {
		if os.IsNotExist(err) {
			registries := make(map[string]*RegistryRecord)
			r.registriesLock.Lock()
			r.registries = registries
			r.registriesLock.Unlock()
			return registries, nil
		}
		return nil, err
	}

	var registries map[string]*RegistryRecord
	if err := json.Unmarshal(data, &registries); err != nil {
		return nil, err
	}

	r.registriesLock.Lock()
	r.registries = registries
	r.registriesLock.Unlock()

	return registries, nil
}

// SaveRegistries 保存仓库列表
func (r *FileRepository) SaveRegistries(registries map[string]*RegistryRecord) error {
	// Ensure data directory exists
	dir := filepath.Dir(r.registriesFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(registries, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(r.registriesFile, data, 0644); err != nil {
		return err
	}

	r.registriesLock.Lock()
	r.registries = registries
	r.registriesLock.Unlock()

	return nil
}

// LoadUpdateSettings loads persisted update settings.
func (r *FileRepository) LoadUpdateSettings() (*UpdateSettingsRecord, error) {
	r.updateLock.RLock()
	if r.updateSettings != nil {
		defer r.updateLock.RUnlock()
		copied := *r.updateSettings
		return &copied, nil
	}
	r.updateLock.RUnlock()

	dir := filepath.Dir(r.updateFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	data, err := os.ReadFile(r.updateFile)
	if err != nil {
		if os.IsNotExist(err) {
			settings := &UpdateSettingsRecord{}
			r.updateLock.Lock()
			r.updateSettings = settings
			r.updateLock.Unlock()
			return &UpdateSettingsRecord{}, nil
		}
		return nil, err
	}

	var settings UpdateSettingsRecord
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, err
	}

	r.updateLock.Lock()
	r.updateSettings = &settings
	r.updateLock.Unlock()

	return &settings, nil
}

// SaveUpdateSettings persists update settings.
func (r *FileRepository) SaveUpdateSettings(settings *UpdateSettingsRecord) error {
	dir := filepath.Dir(r.updateFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(r.updateFile, data, 0644); err != nil {
		return err
	}

	copied := *settings
	r.updateLock.Lock()
	r.updateSettings = &copied
	r.updateLock.Unlock()

	return nil
}
