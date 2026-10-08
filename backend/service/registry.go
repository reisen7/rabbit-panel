package service

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/docker/docker/api/types"

	"rabbit-panel/repository"
)

// RegistryService 镜像仓库服务
type RegistryService struct {
	fileRepo repository.IFileRepository
}

// NewRegistryService 创建仓库服务
func NewRegistryService(fr repository.IFileRepository) *RegistryService {
	return &RegistryService{fileRepo: fr}
}

// ListRegistries 列出仓库。返回副本，避免把内存里的密码清掉。
func (s *RegistryService) ListRegistries() ([]*repository.RegistryRecord, error) {
	registries, err := s.fileRepo.LoadRegistries()
	if err != nil {
		return nil, err
	}

	result := make([]*repository.RegistryRecord, 0, len(registries))
	for _, r := range registries {
		copied := *r
		copied.Password = ""
		result = append(result, &copied)
	}
	return result, nil
}

// SaveRegistry 创建或更新仓库。密码留空时保留原密码；地址变更时删掉旧记录。
func (s *RegistryService) SaveRegistry(r *repository.RegistryRecord, previousURL string) error {
	registries, err := s.fileRepo.LoadRegistries()
	if err != nil {
		return err
	}

	r.URL = strings.TrimSpace(r.URL)
	previousURL = strings.TrimSpace(previousURL)
	if existing, ok := registries[r.URL]; ok {
		if r.Password == "" {
			r.Password = existing.Password
		}
		if r.CreatedAt == "" {
			r.CreatedAt = existing.CreatedAt
		}
	}
	obsolete := ""
	if previousURL != "" && previousURL != r.URL {
		if existing, ok := registries[previousURL]; ok {
			if r.Password == "" {
				r.Password = existing.Password
			}
			if r.CreatedAt == "" {
				r.CreatedAt = existing.CreatedAt
			}
		}
		delete(registries, previousURL)
		obsolete = previousURL
	}
	if r.ID == "" {
		r.ID = r.URL
	}
	now := time.Now().Format(time.RFC3339)
	if r.CreatedAt == "" {
		r.CreatedAt = now
	}
	r.UpdatedAt = now

	registries[r.URL] = r
	if err := s.fileRepo.SaveRegistries(registries); err != nil {
		return err
	}
	s.syncDockerCLIAuth(registries, obsolete)
	return nil
}

// CreateRegistry 创建仓库
func (s *RegistryService) CreateRegistry(r *repository.RegistryRecord) error {
	return s.SaveRegistry(r, "")
}

// DeleteRegistry 删除仓库
func (s *RegistryService) DeleteRegistry(rawURL string) error {
	registries, err := s.fileRepo.LoadRegistries()
	if err != nil {
		return err
	}
	delete(registries, strings.TrimSpace(rawURL))
	if err := s.fileRepo.SaveRegistries(registries); err != nil {
		return err
	}
	s.syncDockerCLIAuth(registries, strings.TrimSpace(rawURL))
	return nil
}

// PullOptions 按镜像地址匹配已保存的仓库账号。
func (s *RegistryService) PullOptions(image string) types.ImagePullOptions {
	record := s.matchRegistry(image)
	if record == nil || record.Username == "" || record.Password == "" {
		return types.ImagePullOptions{}
	}
	host := imageRegistryHost(image)
	server := host
	if isDockerHub(host) {
		server = "https://index.docker.io/v1/"
	}
	payload, err := json.Marshal(map[string]string{
		"username":      record.Username,
		"password":      record.Password,
		"serveraddress": server,
	})
	if err != nil {
		return types.ImagePullOptions{}
	}
	return types.ImagePullOptions{RegistryAuth: base64.URLEncoding.EncodeToString(payload)}
}

// TestRegistry 测试仓库连接。未提交密码时使用已保存的密码。
func (s *RegistryService) TestRegistry(rawURL, username, password string) (bool, string) {
	if stored := s.lookup(rawURL); stored != nil {
		if username == "" {
			username = stored.Username
		}
		if password == "" {
			password = stored.Password
		}
	}

	_, bases := splitRegistryURL(rawURL)
	sawUnauthorized := false
	for _, base := range bases {
		ok, authRequired, failedAuth := probeRegistry(base, username, password)
		if ok {
			if username != "" {
				return true, "认证成功"
			}
			return true, "连接成功"
		}
		if failedAuth {
			sawUnauthorized = true
		}
		if authRequired && username == "" {
			return false, "仓库需要登录"
		}
	}
	if sawUnauthorized {
		return false, "用户名或密码错误"
	}
	return false, "连接失败，请检查地址和凭据"
}

func (s *RegistryService) lookup(rawURL string) *repository.RegistryRecord {
	registries, err := s.fileRepo.LoadRegistries()
	if err != nil {
		return nil
	}
	if record, ok := registries[strings.TrimSpace(rawURL)]; ok {
		return record
	}
	host, _ := splitRegistryURL(rawURL)
	for _, record := range registries {
		regHost, _ := splitRegistryURL(record.URL)
		if hostsMatch(host, regHost) {
			return record
		}
	}
	return nil
}

func (s *RegistryService) matchRegistry(image string) *repository.RegistryRecord {
	registries, err := s.fileRepo.LoadRegistries()
	if err != nil {
		return nil
	}
	host := imageRegistryHost(image)
	var fallback *repository.RegistryRecord
	for _, record := range registries {
		regHost, _ := splitRegistryURL(record.URL)
		if !hostsMatch(host, regHost) {
			continue
		}
		if record.Username != "" && record.Password != "" {
			return record
		}
		fallback = record
	}
	return fallback
}

func probeRegistry(base, username, password string) (ok, authRequired, failedAuth bool) {
	client := &http.Client{Timeout: 8 * time.Second}
	req, err := http.NewRequest(http.MethodGet, strings.TrimRight(base, "/")+"/v2/", nil)
	if err != nil {
		return false, false, false
	}
	if username != "" {
		req.SetBasicAuth(username, password)
	}
	resp, err := client.Do(req)
	if err != nil {
		return false, false, false
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode == http.StatusOK {
		return true, false, false
	}
	if resp.StatusCode != http.StatusUnauthorized {
		return false, false, false
	}
	if username == "" {
		return false, true, false
	}
	if exchangeRegistryToken(client, resp.Header.Get("Www-Authenticate"), username, password) {
		return true, false, false
	}
	return false, false, true
}

func exchangeRegistryToken(client *http.Client, challenge, username, password string) bool {
	params := parseAuthParams(challenge)
	realm := params["realm"]
	if realm == "" || !strings.HasPrefix(strings.ToLower(strings.TrimSpace(challenge)), "bearer") {
		return false
	}
	query := url.Values{}
	if params["service"] != "" {
		query.Set("service", params["service"])
	}
	if params["scope"] != "" {
		query.Set("scope", params["scope"])
	}
	tokenURL := realm
	if encoded := query.Encode(); encoded != "" {
		join := "?"
		if strings.Contains(realm, "?") {
			join = "&"
		}
		tokenURL += join + encoded
	}
	req, err := http.NewRequest(http.MethodGet, tokenURL, nil)
	if err != nil {
		return false
	}
	req.SetBasicAuth(username, password)
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode == http.StatusOK
}

func parseAuthParams(header string) map[string]string {
	params := map[string]string{}
	_, rest, ok := strings.Cut(header, " ")
	if !ok {
		return params
	}
	for _, part := range strings.Split(rest, ",") {
		key, val, found := strings.Cut(strings.TrimSpace(part), "=")
		if !found {
			continue
		}
		params[strings.ToLower(strings.TrimSpace(key))] = strings.Trim(strings.TrimSpace(val), `"`)
	}
	return params
}

func imageRegistryHost(image string) string {
	name := strings.TrimSpace(image)
	if i := strings.Index(name, "@"); i >= 0 {
		name = name[:i]
	}
	slash := strings.Index(name, "/")
	if slash < 0 {
		return "docker.io"
	}
	first := strings.ToLower(name[:slash])
	if first != "localhost" && !strings.Contains(first, ".") && !strings.Contains(first, ":") {
		return "docker.io"
	}
	return first
}

func splitRegistryURL(raw string) (string, []string) {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimRight(raw, "/")
	scheme := ""
	switch {
	case strings.HasPrefix(strings.ToLower(raw), "https://"):
		scheme = "https"
		raw = raw[len("https://"):]
	case strings.HasPrefix(strings.ToLower(raw), "http://"):
		scheme = "http"
		raw = raw[len("http://"):]
	}
	if i := strings.Index(raw, "/"); i >= 0 {
		raw = raw[:i]
	}
	host := strings.ToLower(raw)
	if host == "" {
		return "", nil
	}
	if scheme == "http" {
		return host, []string{"http://" + host, "https://" + host}
	}
	return host, []string{"https://" + host, "http://" + host}
}

func hostsMatch(imageHost, registryHost string) bool {
	if imageHost == "" || registryHost == "" {
		return false
	}
	if imageHost == registryHost {
		return true
	}
	return isDockerHub(imageHost) && isDockerHub(registryHost)
}

func isDockerHub(host string) bool {
	switch host {
	case "docker.io", "index.docker.io", "registry-1.docker.io":
		return true
	default:
		return false
	}
}

func dockerAuthKey(rawURL string) string {
	host, _ := splitRegistryURL(rawURL)
	if isDockerHub(host) {
		return "https://index.docker.io/v1/"
	}
	return host
}

func dockerCLIConfigPath() string {
	if dir := strings.TrimSpace(os.Getenv("DOCKER_CONFIG")); dir != "" {
		return filepath.Join(dir, "config.json")
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".docker", "config.json")
}

// syncDockerCLIAuth 把仓库账号写入 Docker CLI 配置，供 compose pull 使用。
func (s *RegistryService) syncDockerCLIAuth(registries map[string]*repository.RegistryRecord, obsolete ...string) {
	path := dockerCLIConfigPath()
	if path == "" {
		return
	}
	root := map[string]any{}
	if data, err := os.ReadFile(path); err == nil && len(bytesTrim(data)) > 0 {
		_ = json.Unmarshal(data, &root)
	}
	auths, _ := root["auths"].(map[string]any)
	if auths == nil {
		auths = map[string]any{}
	}

	for _, raw := range obsolete {
		if key := dockerAuthKey(raw); key != "" {
			delete(auths, key)
		}
	}
	for _, record := range registries {
		key := dockerAuthKey(record.URL)
		if key == "" {
			continue
		}
		if record.Username == "" || record.Password == "" {
			delete(auths, key)
			continue
		}
		auths[key] = map[string]string{
			"auth": base64.StdEncoding.EncodeToString([]byte(record.Username + ":" + record.Password)),
		}
	}
	root["auths"] = auths
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return
	}
	data, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(path, append(data, '\n'), 0o600)
}

func bytesTrim(data []byte) []byte {
	return []byte(strings.TrimSpace(string(data)))
}
