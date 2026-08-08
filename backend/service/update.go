package service

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"rabbit-panel/repository"
)

const (
	updateManifestURL         = "https://reisen7.github.io/rabbit-panel/latest.json"
	updateManifestFallbackURL = "https://raw.githubusercontent.com/reisen7/rabbit-panel/gh-pages/latest.json"
	updateLogPath             = "/tmp/rabbit-panel-update.log"
	updateScriptPath          = "/tmp/rabbit-panel-update.sh"
	updateBinaryDir           = "/tmp/rabbit-panel-update"
	updateStatePath           = "/tmp/rabbit-panel-update.state"
	updateResultPath          = "/tmp/rabbit-panel-update.result"

	// When set to true, the update check (manifest fetch) is skipped entirely.
	updateCheckDisabledEnv = "RABBIT_UPDATE_CHECK_DISABLED"
)

type BuildInfo struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildTime string `json:"build_time"`
}

type DeployConfig struct {
	Mode           string `json:"mode"`
	Image          string `json:"image"`
	ImageTag       string `json:"image_tag"`
	HostProjectDir string `json:"host_project_dir"`
	ComposeFile    string `json:"compose_file"`
	ServiceName    string `json:"service_name"`
}

type UpdateManifest struct {
	Version      string            `json:"version"`
	VersionNoV   string            `json:"version_number"`
	ReleaseURL   string            `json:"release_url"`
	ReleaseName  string            `json:"release_name"`
	ReleaseNotes string            `json:"release_notes"`
	PublishedAt  string            `json:"published_at"`
	DockerImage  string            `json:"docker_image"`
	DockerTag    string            `json:"docker_tag"`
	DockerTags   []string          `json:"docker_tags"`
	Assets       map[string]string `json:"assets"`
	SHA256       map[string]string `json:"sha256"`
}

type UpdateCheckResult struct {
	CurrentVersion   string `json:"current_version"`
	CurrentCommit    string `json:"current_commit"`
	CurrentBuildTime string `json:"current_build_time"`
	LatestVersion    string `json:"latest_version"`
	HasUpdate        bool   `json:"has_update"`
	DeployMode       string `json:"deploy_mode"`
	Image            string `json:"image"`
	ImageTag         string `json:"image_tag"`
	CanUpdate        bool   `json:"can_update"`
	IgnoredVersion   string `json:"ignored_version"`
	Ignored          bool   `json:"ignored"`
	ReleaseURL       string `json:"release_url"`
	ReleaseNotes     string `json:"release_notes"`
	Message          string `json:"message"`
	LastCheckTime    string `json:"last_check_time"`
	LastUpdateTime   string `json:"last_update_time"`
	LastUpdateStatus string `json:"last_update_status"`
	LastUpdateError  string `json:"last_update_error"`
}

type UpdateTaskStatus struct {
	Status          string   `json:"status"`
	Stage           string   `json:"stage"`
	Progress        int      `json:"progress"`
	ProgressKnown   bool     `json:"progress_known"`
	LastUpdateTime  string   `json:"last_update_time"`
	LastError       string   `json:"last_error"`
	LogLines        []string `json:"log_lines"`
	Prepared        bool     `json:"prepared"`
	PreparedVersion string   `json:"prepared_version"`
}

type runtimeUpdateState struct {
	Status             string
	Stage              string
	Progress           int
	ProgressKnown      bool
	LastUpdateTime     string
	LastError          string
	Prepared           bool
	PreparedVersion    string
	PreparedBinaryPath string
	OwnerPID           int
	WorkerPID          int
}

type UpdateSettings struct {
	IgnoredVersion     string `json:"ignored_version"`
	LastCheckTime      string `json:"last_check_time"`
	LastUpdateTime     string `json:"last_update_time"`
	LastUpdateStatus   string `json:"last_update_status"`
	LastUpdateError    string `json:"last_update_error"`
	PreparedBinaryPath string `json:"prepared_binary_path"`
	PreparedVersion    string `json:"prepared_version"`
}

type externalUpdateResult struct {
	Status             string
	LastUpdateTime     string
	LastError          string
	ClearPrepared      bool
	PreparedBinaryPath string
	PreparedVersion    string
}

type UpdateService struct {
	fileRepo       repository.IFileRepository
	buildInfo      BuildInfo
	httpClient     *http.Client
	downloadClient *http.Client
	mu             sync.Mutex
}

func NewUpdateService(fileRepo repository.IFileRepository, buildInfo BuildInfo) *UpdateService {
	return &UpdateService{
		fileRepo:  fileRepo,
		buildInfo: buildInfo,
		httpClient: &http.Client{
			Timeout: 20 * time.Second,
		},
		// Binary downloads can take much longer than manifest checks.
		// We rely on request contexts to bound download duration.
		downloadClient: &http.Client{},
	}
}

func (s *UpdateService) GetBuildInfo() BuildInfo {
	return s.buildInfo
}

func (s *UpdateService) DetectDeployConfig() DeployConfig {
	mode := strings.TrimSpace(os.Getenv("RABBIT_DEPLOY_MODE"))
	if mode == "" {
		mode = detectContainerMode()
	}
	if mode != "docker" && mode != "binary" {
		mode = "binary"
	}

	return DeployConfig{
		Mode:           mode,
		Image:          getEnv("RABBIT_IMAGE", "reisen7/rabbit-panel"),
		ImageTag:       getEnv("RABBIT_IMAGE_TAG", "latest"),
		HostProjectDir: getEnv("RABBIT_HOST_PROJECT_DIR", "/root/rabbit-panel"),
		ComposeFile:    getEnv("RABBIT_COMPOSE_FILE", "docker-compose.deploy.yml"),
		ServiceName:    getEnv("RABBIT_SERVICE_NAME", "rabbit-panel"),
	}
}

func (s *UpdateService) Check(ctx context.Context) (*UpdateCheckResult, error) {
	if err := s.reconcileUpdateArtifacts(); err != nil {
		return nil, err
	}
	settings, err := s.loadSettings()
	if err != nil {
		return nil, err
	}

	if updateCheckDisabled() {
		deploy := s.DetectDeployConfig()
		return &UpdateCheckResult{
			CurrentVersion:   s.buildInfo.Version,
			CurrentCommit:    s.buildInfo.Commit,
			CurrentBuildTime: s.buildInfo.BuildTime,
			LatestVersion:    "",
			HasUpdate:        false,
			DeployMode:       deploy.Mode,
			Image:            deploy.Image,
			ImageTag:         deploy.ImageTag,
			CanUpdate:        false,
			Message:          "update check is disabled",
			LastCheckTime:    settings.LastCheckTime,
			LastUpdateTime:   settings.LastUpdateTime,
			LastUpdateStatus: settings.LastUpdateStatus,
			LastUpdateError:  settings.LastUpdateError,
		}, nil
	}

	manifest, err := s.fetchManifest(ctx)
	if err != nil {
		return nil, err
	}

	deploy := s.DetectDeployConfig()
	currentVersion := normalizeVersion(s.buildInfo.Version)
	latestVersion := normalizeVersion(manifest.Version)
	if deploy.Mode == "docker" {
		if updated, err := s.reconcileDockerUpdateStatus(settings, currentVersion, latestVersion); err != nil {
			return nil, err
		} else if updated {
			settings, err = s.loadSettings()
			if err != nil {
				return nil, err
			}
		}
	}

	hasUpdate := false
	if currentVersion != "" && currentVersion != "dev" && latestVersion != "" {
		cmp, cmpErr := compareVersions(currentVersion, latestVersion)
		hasUpdate = cmpErr == nil && cmp < 0
	}

	ignored := settings.IgnoredVersion != "" && normalizeVersion(settings.IgnoredVersion) == latestVersion
	if ignored || currentVersion == "dev" {
		hasUpdate = false
	}

	settings.LastCheckTime = time.Now().Format(time.RFC3339)
	if err := s.saveSettings(settings); err != nil {
		log.Printf("save update settings failed: %v", err)
	}

	return &UpdateCheckResult{
		CurrentVersion:   s.buildInfo.Version,
		CurrentCommit:    s.buildInfo.Commit,
		CurrentBuildTime: s.buildInfo.BuildTime,
		LatestVersion:    manifest.Version,
		HasUpdate:        hasUpdate,
		DeployMode:       deploy.Mode,
		Image:            deploy.Image,
		ImageTag:         deploy.ImageTag,
		CanUpdate:        canUpdate(deploy),
		IgnoredVersion:   settings.IgnoredVersion,
		Ignored:          ignored,
		ReleaseURL:       manifest.ReleaseURL,
		ReleaseNotes:     manifest.ReleaseNotes,
		Message:          buildUpdateMessage(s.buildInfo.Version, manifest.Version, hasUpdate, ignored),
		LastCheckTime:    settings.LastCheckTime,
		LastUpdateTime:   settings.LastUpdateTime,
		LastUpdateStatus: settings.LastUpdateStatus,
		LastUpdateError:  settings.LastUpdateError,
	}, nil
}

func (s *UpdateService) IgnoreVersion(version string) error {
	settings, err := s.loadSettings()
	if err != nil {
		return err
	}
	settings.IgnoredVersion = strings.TrimSpace(version)
	return s.saveSettings(settings)
}

func (s *UpdateService) ClearIgnoredVersion() error {
	settings, err := s.loadSettings()
	if err != nil {
		return err
	}
	settings.IgnoredVersion = ""
	return s.saveSettings(settings)
}

func (s *UpdateService) ClearUpdateState() error {
	if err := s.reconcileExternalUpdateResult(); err != nil {
		return err
	}
	settings, err := s.loadSettings()
	if err != nil {
		return err
	}
	if state, ok := readRuntimeUpdateState(); ok && updateTaskStillActive(state) {
		return errors.New("cannot clear update state while an update task is still running")
	}

	_ = os.RemoveAll(updateBinaryDir)
	_ = os.Remove(updateLogPath)
	_ = os.Remove(updateStatePath)
	_ = os.Remove(updateResultPath)
	settings.LastUpdateTime = ""
	settings.LastUpdateStatus = ""
	settings.LastUpdateError = ""
	settings.PreparedBinaryPath = ""
	settings.PreparedVersion = ""
	return s.saveSettings(settings)
}

func (s *UpdateService) StartUpdate(ctx context.Context) error {
	return s.PrepareUpdate(ctx)
}

func (s *UpdateService) PrepareUpdate(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.reconcileUpdateArtifacts(); err != nil {
		return err
	}

	deploy := s.DetectDeployConfig()
	if deploy.Mode == "docker" && !canUpdate(deploy) {
		return errors.New("current Docker deployment uses a fixed image tag; switch to latest or update manually")
	}
	if deploy.Mode != "docker" && deploy.Mode != "binary" {
		return errors.New("unsupported deploy mode for update")
	}
	activeState, active := readRuntimeUpdateState()
	if active && (activeState.Status == "running" || activeState.Status == "applying") {
		return errors.New("an update task is already running")
	}
	manifest, err := s.fetchManifest(ctx)
	if err != nil {
		return err
	}

	if deploy.Mode == "binary" {
		return s.prepareBinaryUpdate(ctx, manifest, deploy)
	}
	return s.startDockerUpdate(deploy)
}

func (s *UpdateService) ApplyPreparedUpdate() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.reconcileUpdateArtifacts(); err != nil {
		return err
	}

	deploy := s.DetectDeployConfig()
	if deploy.Mode != "binary" {
		return errors.New("apply update is only supported for binary deployment")
	}
	if err := ensureSystemctlAvailable(); err != nil {
		return err
	}
	deploy.ServiceName = normalizeSystemdServiceName(deploy.ServiceName)

	settings, err := s.loadSettings()
	if err != nil {
		return err
	}
	preparedPath := strings.TrimSpace(settings.PreparedBinaryPath)
	if preparedPath == "" {
		return errors.New("no prepared update package available")
	}
	if _, err := os.Stat(preparedPath); err != nil {
		settings.PreparedBinaryPath = ""
		settings.PreparedVersion = ""
		settings.LastUpdateStatus = "failed"
		settings.LastUpdateError = "prepared update package is missing; please download it again"
		settings.LastUpdateTime = time.Now().Format(time.RFC3339)
		_ = s.saveSettings(settings)
		clearRuntimeUpdateState()
		return errors.New("prepared update package is missing; please download it again")
	}

	return s.applyBinaryUpdate(preparedPath, deploy, settings.PreparedVersion)
}

func (s *UpdateService) GetTaskStatus() (*UpdateTaskStatus, error) {
	if err := s.reconcileUpdateArtifacts(); err != nil {
		return nil, err
	}
	settings, err := s.loadSettings()
	if err != nil {
		return nil, err
	}

	if runtimeState, ok := readRuntimeUpdateState(); ok {
		return &UpdateTaskStatus{
			Status:          runtimeState.Status,
			Stage:           runtimeState.Stage,
			Progress:        runtimeState.Progress,
			ProgressKnown:   runtimeState.ProgressKnown,
			LastUpdateTime:  runtimeState.LastUpdateTime,
			LastError:       runtimeState.LastError,
			LogLines:        readLastLogLines(updateLogPath, 80),
			Prepared:        runtimeState.Prepared || strings.TrimSpace(settings.PreparedBinaryPath) != "",
			PreparedVersion: runtimeState.PreparedVersion,
		}, nil
	}

	logLines := readLastLogLines(updateLogPath, 80)
	stage, progress, progressKnown := deriveStage(logLines, settings.LastUpdateStatus)

	return &UpdateTaskStatus{
		Status:          settings.LastUpdateStatus,
		Stage:           stage,
		Progress:        progress,
		ProgressKnown:   progressKnown,
		LastUpdateTime:  settings.LastUpdateTime,
		LastError:       settings.LastUpdateError,
		LogLines:        logLines,
		Prepared:        strings.TrimSpace(settings.PreparedBinaryPath) != "",
		PreparedVersion: settings.PreparedVersion,
	}, nil
}

func (s *UpdateService) loadSettings() (*UpdateSettings, error) {
	record, err := s.fileRepo.LoadUpdateSettings()
	if err != nil {
		return nil, err
	}
	return &UpdateSettings{
		IgnoredVersion:     record.IgnoredVersion,
		LastCheckTime:      record.LastCheckTime,
		LastUpdateTime:     record.LastUpdateTime,
		LastUpdateStatus:   record.LastUpdateStatus,
		LastUpdateError:    record.LastUpdateError,
		PreparedBinaryPath: record.PreparedBinaryPath,
		PreparedVersion:    record.PreparedVersion,
	}, nil
}

func (s *UpdateService) saveSettings(settings *UpdateSettings) error {
	return s.fileRepo.SaveUpdateSettings(&repository.UpdateSettingsRecord{
		IgnoredVersion:     settings.IgnoredVersion,
		LastCheckTime:      settings.LastCheckTime,
		LastUpdateTime:     settings.LastUpdateTime,
		LastUpdateStatus:   settings.LastUpdateStatus,
		LastUpdateError:    settings.LastUpdateError,
		PreparedBinaryPath: settings.PreparedBinaryPath,
		PreparedVersion:    settings.PreparedVersion,
	})
}

func (s *UpdateService) updateStatus(status, errMsg string) {
	settings, err := s.loadSettings()
	if err != nil {
		log.Printf("load update settings failed: %v", err)
		return
	}
	settings.LastUpdateStatus = status
	settings.LastUpdateError = errMsg
	settings.LastUpdateTime = time.Now().Format(time.RFC3339)
	if err := s.saveSettings(settings); err != nil {
		log.Printf("save update status failed: %v", err)
	}
}

func (s *UpdateService) fetchManifest(ctx context.Context) (*UpdateManifest, error) {
	primaryURL := getEnv("RABBIT_UPDATE_MANIFEST_URL", updateManifestURL)
	fallbackURL := getEnv("RABBIT_UPDATE_MANIFEST_FALLBACK_URL", updateManifestFallbackURL)

	urls := []string{primaryURL}
	if fallbackURL != "" && fallbackURL != primaryURL {
		urls = append(urls, fallbackURL)
	}

	var errs []string
	for _, manifestURL := range urls {
		manifest, err := s.fetchManifestFromURL(ctx, manifestURL)
		if err == nil {
			return manifest, nil
		}
		errs = append(errs, err.Error())
	}

	return nil, fmt.Errorf("failed to fetch update manifest: %s", strings.Join(errs, "; "))
}

func (s *UpdateService) fetchManifestFromURL(ctx context.Context, manifestURL string) (*UpdateManifest, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, manifestURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "rabbit-panel")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request manifest from %s failed: %w", manifestURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("request manifest from %s failed: %s %s", manifestURL, resp.Status, strings.TrimSpace(string(body)))
	}

	var manifest UpdateManifest
	if err := json.NewDecoder(resp.Body).Decode(&manifest); err != nil {
		return nil, fmt.Errorf("decode manifest from %s failed: %w", manifestURL, err)
	}
	if manifest.Version == "" {
		return nil, fmt.Errorf("manifest from %s missing version", manifestURL)
	}
	return &manifest, nil
}

func (s *UpdateService) prepareBinaryUpdate(ctx context.Context, manifest *UpdateManifest, deploy DeployConfig) error {
	assetURL, assetName, archKey, err := findBinaryAsset(manifest.Assets)
	if err != nil {
		return err
	}
	expectedSHA := strings.TrimSpace(manifest.SHA256[archKey])

	if err := os.MkdirAll(updateBinaryDir, 0755); err != nil {
		return err
	}
	downloadPath := filepath.Join(updateBinaryDir, assetName)
	settings, err := s.loadSettings()
	if err != nil {
		return err
	}
	if previousPath := strings.TrimSpace(settings.PreparedBinaryPath); previousPath != "" && previousPath != downloadPath {
		_ = os.Remove(previousPath)
	}
	_ = os.Remove(updateResultPath)
	settings.PreparedBinaryPath = ""
	settings.PreparedVersion = ""
	settings.LastUpdateStatus = "running"
	settings.LastUpdateError = ""
	settings.LastUpdateTime = time.Now().Format(time.RFC3339)
	if err := s.saveSettings(settings); err != nil {
		return err
	}
	writeRuntimeUpdateState(runtimeUpdateState{
		Status:         "running",
		Stage:          "downloading",
		Progress:       0,
		ProgressKnown:  false,
		LastUpdateTime: settings.LastUpdateTime,
		OwnerPID:       os.Getpid(),
	})

	go func() {
		downloadCtx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()

		if err := s.downloadBinaryWithProgress(downloadCtx, assetURL, downloadPath, expectedSHA); err != nil {
			s.updateStatus("failed", err.Error())
			clearRuntimeUpdateState()
			return
		}

		nextSettings, err := s.loadSettings()
		if err != nil {
			s.updateStatus("failed", err.Error())
			return
		}
		nextSettings.PreparedBinaryPath = downloadPath
		nextSettings.PreparedVersion = manifest.Version
		nextSettings.LastUpdateStatus = "downloaded"
		nextSettings.LastUpdateError = ""
		nextSettings.LastUpdateTime = time.Now().Format(time.RFC3339)
		if err := s.saveSettings(nextSettings); err != nil {
			s.updateStatus("failed", err.Error())
			clearRuntimeUpdateState()
			return
		}
		writeRuntimeUpdateState(runtimeUpdateState{
			Status:             "downloaded",
			Stage:              "downloaded",
			Progress:           100,
			ProgressKnown:      true,
			LastUpdateTime:     nextSettings.LastUpdateTime,
			Prepared:           true,
			PreparedVersion:    nextSettings.PreparedVersion,
			PreparedBinaryPath: nextSettings.PreparedBinaryPath,
			OwnerPID:           os.Getpid(),
		})
	}()

	return nil
}

func (s *UpdateService) startDockerUpdate(deploy DeployConfig) error {
	script := fmt.Sprintf(`#!/bin/sh
set -eu
LOG_FILE=%s
mkdir -p "$(dirname "$LOG_FILE")"
: >"$LOG_FILE"
exec >>"$LOG_FILE" 2>&1
echo "[%s] docker update started"
PROJECT_DIR=%s
COMPOSE_FILE=%s
echo "project dir: $PROJECT_DIR"
echo "compose file: $COMPOSE_FILE"
if command -v docker >/dev/null 2>&1; then
  echo "using local docker cli"
  cd "$PROJECT_DIR"
  echo "docker compose pull"
  docker compose -f "$COMPOSE_FILE" pull
  echo "docker compose up -d"
  docker compose -f "$COMPOSE_FILE" up -d
else
  echo "using docker:cli helper container"
  docker run --rm -v /var/run/docker.sock:/var/run/docker.sock -v "$PROJECT_DIR":"$PROJECT_DIR" -w "$PROJECT_DIR" docker:cli sh -c "docker compose -f \"$COMPOSE_FILE\" pull && docker compose -f \"$COMPOSE_FILE\" up -d"
fi
echo "docker update completed"
`, shellQuote(updateLogPath), time.Now().Format(time.RFC3339), shellQuote(deploy.HostProjectDir), shellQuote(deploy.ComposeFile))

	if err := os.WriteFile(updateScriptPath, []byte(script), 0700); err != nil {
		return err
	}

	s.updateStatus("running", "")
	writeRuntimeUpdateState(runtimeUpdateState{
		Status:         "running",
		Stage:          "recreating",
		Progress:       0,
		ProgressKnown:  false,
		LastUpdateTime: time.Now().Format(time.RFC3339),
		OwnerPID:       os.Getpid(),
	})
	cmd := exec.Command("nohup", "sh", updateScriptPath)
	if err := cmd.Start(); err != nil {
		s.updateStatus("failed", err.Error())
		clearRuntimeUpdateState()
		return err
	}
	go s.watchProcess(cmd)
	return nil
}

func (s *UpdateService) applyBinaryUpdate(preparedPath string, deploy DeployConfig, preparedVersion string) error {
	execPath, err := os.Executable()
	if err != nil {
		return err
	}
	execPath, err = filepath.Abs(execPath)
	if err != nil {
		return err
	}

	if err := os.Remove(updateResultPath); err != nil && !os.IsNotExist(err) {
		return err
	}

	script := fmt.Sprintf(`#!/bin/sh
set -eu
LOG_FILE=%s
STATE_FILE=%s
RESULT_FILE=%s
mkdir -p "$(dirname "$LOG_FILE")"
: >"$LOG_FILE"
exec >>"$LOG_FILE" 2>&1
now() {
  date --iso-8601=seconds 2>/dev/null || date "+%%Y-%%m-%%dT%%H:%%M:%%S%%z"
}
sanitize() {
  printf "%%s" "$1" | tr '\n' ' ' | tr '\r' ' '
}
CURRENT_STAGE="initializing"
write_state() {
  status="$1"
  stage="$2"
  progress="$3"
  error_message="${4:-}"
  cat >"$STATE_FILE" <<EOF
status=$status
stage=$stage
progress=$progress
progress_known=true
last_update_time=$(now)
last_error=$(sanitize "$error_message")
prepared=true
prepared_version=%s
prepared_binary_path=%s
worker_pid=$$
EOF
}
write_result() {
  status="$1"
  error_message="${2:-}"
  clear_prepared="${3:-false}"
  cat >"$RESULT_FILE" <<EOF
status=$status
last_update_time=$(now)
last_error=$(sanitize "$error_message")
clear_prepared=$clear_prepared
prepared_binary_path=%s
prepared_version=%s
EOF
}
fail() {
  message="${1:-update apply failed}"
  write_result failed "$message" false
  rm -f "$STATE_FILE"
  echo "$message"
  exit 1
}
trap 'code=$?; if [ "$code" -ne 0 ] && [ ! -f "$RESULT_FILE" ]; then write_result failed "$CURRENT_STAGE" false; rm -f "$STATE_FILE"; echo "$CURRENT_STAGE"; fi' EXIT
echo "[%s] applying downloaded update"
CURRENT_BIN=%s
BACKUP_BIN="${CURRENT_BIN}.bak"
TMP_BIN="${CURRENT_BIN}.new"
NEW_BIN=%s
if [ ! -f "$NEW_BIN" ]; then
  fail "prepared binary not found: $NEW_BIN"
fi
write_state applying applying 100
echo "current binary: $CURRENT_BIN"
echo "prepared binary: $NEW_BIN"
CURRENT_STAGE="creating backup"
echo "creating backup: $BACKUP_BIN"
cp "$CURRENT_BIN" "$BACKUP_BIN"
echo "backup created"
CURRENT_STAGE="stopping service: %s"
write_state applying stopping 100
echo "stopping service: %s"
systemctl stop %s
echo "service stopped"
CURRENT_STAGE="replacing binary"
echo "preparing replacement binary"
cp "$NEW_BIN" "$TMP_BIN"
chmod +x "$TMP_BIN"
mv -f "$TMP_BIN" "$CURRENT_BIN"
echo "binary replaced"
CURRENT_STAGE="starting service: %s"
write_state applying restarting 100
echo "starting service: %s"
systemctl reset-failed %s || true
if ! systemctl start %s; then
  echo "service start failed, rolling back"
  cp "$BACKUP_BIN" "$CURRENT_BIN"
  chmod +x "$CURRENT_BIN"
  systemctl reset-failed %s || true
  systemctl start %s || true
  fail "service start failed after replacement, rollback attempted"
fi
echo "service started"
CURRENT_STAGE="completed"
write_result success "" true
rm -f "$STATE_FILE" "$NEW_BIN" "$LOG_FILE"
echo "update apply completed"
`, shellQuote(updateLogPath), shellQuote(updateStatePath), shellQuote(updateResultPath), preparedVersion, preparedPath, preparedPath, preparedVersion, time.Now().Format(time.RFC3339), shellQuote(execPath), shellQuote(preparedPath), shellQuote(deploy.ServiceName), shellQuote(deploy.ServiceName), shellQuote(deploy.ServiceName), shellQuote(deploy.ServiceName), shellQuote(deploy.ServiceName), shellQuote(deploy.ServiceName), shellQuote(deploy.ServiceName), shellQuote(deploy.ServiceName), shellQuote(deploy.ServiceName))

	if err := os.WriteFile(updateScriptPath, []byte(script), 0700); err != nil {
		return err
	}

	s.updateStatus("applying", "")
	writeRuntimeUpdateState(runtimeUpdateState{
		Status:             "applying",
		Stage:              "applying",
		Progress:           100,
		ProgressKnown:      true,
		LastUpdateTime:     time.Now().Format(time.RFC3339),
		Prepared:           true,
		PreparedVersion:    preparedVersion,
		PreparedBinaryPath: preparedPath,
		OwnerPID:           os.Getpid(),
	})
	if err := startDetachedUpdateScript(updateScriptPath); err != nil {
		s.updateStatus("failed", err.Error())
		clearRuntimeUpdateState()
		return err
	}
	return nil
}

func (s *UpdateService) downloadBinaryWithProgress(ctx context.Context, downloadURL, outputPath, expectedSHA string) error {
	if err := os.MkdirAll(filepath.Dir(updateLogPath), 0755); err != nil {
		return err
	}
	logFile, err := os.OpenFile(updateLogPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer logFile.Close()

	writeLog := func(format string, args ...interface{}) {
		fmt.Fprintf(logFile, format+"\n", args...)
	}
	cleanupOutput := true
	defer func() {
		if cleanupOutput {
			_ = os.Remove(outputPath)
		}
	}()

	writeLog("[%s] binary update started", time.Now().Format(time.RFC3339))
	writeLog("download url: %s", downloadURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "rabbit-panel")

	resp, err := s.downloadClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("download failed: %s %s", resp.Status, strings.TrimSpace(string(body)))
	}

	totalSize := resp.ContentLength
	writeLog("content length: %d", totalSize)
	writeLog("PROGRESS:0")
	writeRuntimeUpdateState(runtimeUpdateState{
		Status:         "running",
		Stage:          "downloading",
		Progress:       0,
		ProgressKnown:  totalSize > 0,
		LastUpdateTime: time.Now().Format(time.RFC3339),
		OwnerPID:       os.Getpid(),
	})

	out, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer out.Close()

	hasher := newSHA256Writer()
	reader := io.TeeReader(resp.Body, hasher)
	buf := make([]byte, 64*1024)
	var downloaded int64
	lastPercent := -1

	for {
		n, readErr := reader.Read(buf)
		if n > 0 {
			if _, err := out.Write(buf[:n]); err != nil {
				return err
			}
			downloaded += int64(n)
			if totalSize > 0 {
				percent := int(downloaded * 100 / totalSize)
				if percent > 100 {
					percent = 100
				}
				if percent != lastPercent {
					writeLog("PROGRESS:%d", percent)
					writeRuntimeUpdateState(runtimeUpdateState{
						Status:         "running",
						Stage:          "downloading",
						Progress:       percent,
						ProgressKnown:  true,
						LastUpdateTime: time.Now().Format(time.RFC3339),
						OwnerPID:       os.Getpid(),
					})
					lastPercent = percent
				}
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return readErr
		}
	}

	writeLog("PROGRESS:100")
	writeRuntimeUpdateState(runtimeUpdateState{
		Status:         "running",
		Stage:          "verifying",
		Progress:       100,
		ProgressKnown:  true,
		LastUpdateTime: time.Now().Format(time.RFC3339),
		OwnerPID:       os.Getpid(),
	})
	if err := out.Chmod(0755); err != nil {
		return err
	}
	writeLog("download completed")
	writeLog("binary chmod +x completed")

	if strings.TrimSpace(expectedSHA) != "" {
		writeLog("verifying sha256")
		actualSHA := hasher.Sum()
		if actualSHA != expectedSHA {
			return fmt.Errorf("sha256 mismatch: expected %s got %s", expectedSHA, actualSHA)
		}
		writeLog("sha256 verified")
	}

	cleanupOutput = false
	return nil
}

func (s *UpdateService) watchProcess(cmd *exec.Cmd) {
	if err := cmd.Wait(); err != nil {
		s.updateStatus("failed", err.Error())
		clearRuntimeUpdateState()
		return
	}
	s.updateStatus("success", "")
	_ = os.Remove(updateLogPath)
	clearRuntimeUpdateState()
}

func detectContainerMode() string {
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return "docker"
	}
	if data, err := os.ReadFile("/proc/1/cgroup"); err == nil {
		content := strings.ToLower(string(data))
		for _, keyword := range []string{"docker", "containerd", "kubepods", "podman", "lxc"} {
			if strings.Contains(content, keyword) {
				return "docker"
			}
		}
	}
	return "binary"
}

func canUpdate(deploy DeployConfig) bool {
	if deploy.Mode == "binary" {
		return runtime.GOOS == "linux"
	}
	if deploy.Mode == "docker" && strings.EqualFold(strings.TrimSpace(deploy.ImageTag), "latest") {
		return true
	}
	return false
}

func buildUpdateMessage(currentVersion, latestVersion string, hasUpdate, ignored bool) string {
	if normalizeVersion(currentVersion) == "dev" {
		return fmt.Sprintf("Current version is dev, latest release is %s", latestVersion)
	}
	if ignored {
		return fmt.Sprintf("Latest version %s is ignored", latestVersion)
	}
	if hasUpdate {
		return fmt.Sprintf("Found new version %s", latestVersion)
	}
	return "Already on the latest version"
}

func normalizeVersion(version string) string {
	version = strings.TrimSpace(version)
	return strings.TrimPrefix(version, "v")
}

func compareVersions(current, latest string) (int, error) {
	curParts, err := parseVersion(normalizeVersion(current))
	if err != nil {
		return 0, err
	}
	latestParts, err := parseVersion(normalizeVersion(latest))
	if err != nil {
		return 0, err
	}
	for i := 0; i < 3; i++ {
		if curParts[i] < latestParts[i] {
			return -1, nil
		}
		if curParts[i] > latestParts[i] {
			return 1, nil
		}
	}
	return 0, nil
}

func parseVersion(version string) ([3]int, error) {
	var result [3]int
	parts := strings.Split(version, ".")
	for i := 0; i < len(parts) && i < 3; i++ {
		part := parts[i]
		for idx, ch := range part {
			if ch < '0' || ch > '9' {
				part = part[:idx]
				break
			}
		}
		if part == "" {
			return result, fmt.Errorf("invalid version: %s", version)
		}
		value, err := strconv.Atoi(part)
		if err != nil {
			return result, err
		}
		result[i] = value
	}
	return result, nil
}

func findBinaryAsset(assets map[string]string) (string, string, string, error) {
	arch, err := currentReleaseArch()
	if err != nil {
		return "", "", "", err
	}
	downloadURL, ok := assets[arch]
	downloadURL = sanitizeManifestValue(downloadURL)
	if !ok || downloadURL == "" {
		return "", "", "", fmt.Errorf("no release asset found for %s", arch)
	}
	return downloadURL, filepath.Base(downloadURL), arch, nil
}

func currentReleaseArch() (string, error) {
	if runtime.GOOS != "linux" {
		return "", fmt.Errorf("binary auto update only supports linux, current os is %s", runtime.GOOS)
	}
	switch runtime.GOARCH {
	case "amd64":
		return "linux-amd64", nil
	case "arm64":
		return "linux-arm64", nil
	case "arm":
		return "linux-armv7", nil
	default:
		return "", fmt.Errorf("unsupported architecture: %s", runtime.GOARCH)
	}
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func getEnv(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func updateCheckDisabled() bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv(updateCheckDisabledEnv)))
	return value == "true" || value == "1" || value == "yes" || value == "on"
}

func ensureSystemctlAvailable() error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("binary auto update only supports linux, current os is %s", runtime.GOOS)
	}
	if _, err := exec.LookPath("systemctl"); err != nil {
		return errors.New("systemctl not found; binary auto update requires running Rabbit Panel as a systemd service")
	}
	return nil
}

func normalizeSystemdServiceName(serviceName string) string {
	serviceName = strings.TrimSpace(serviceName)
	if serviceName == "" {
		return "rabbit-panel.service"
	}
	if strings.Contains(serviceName, ".") {
		return serviceName
	}
	return serviceName + ".service"
}

func readLastLogLines(path string, limit int) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return []string{}
	}
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	filtered := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			filtered = append(filtered, line)
		}
	}
	if len(filtered) <= limit {
		return filtered
	}
	return filtered[len(filtered)-limit:]
}

func deriveStage(lines []string, status string) (string, int, bool) {
	lines = extractCurrentRunLines(lines)
	stage := "pending"
	progress := 0
	progressKnown := false
	joined := strings.ToLower(strings.Join(lines, "\n"))
	if p, ok := extractProgress(lines); ok {
		progress = p
		progressKnown = true
		stage = "downloading"
	}

	switch {
	case stage == "pending" && (strings.Contains(joined, "binary update started") || strings.Contains(joined, "docker update started")):
		stage = "started"
	}
	switch {
	case progressKnown || strings.Contains(joined, "download url:"):
		stage = "downloading"
	case strings.Contains(joined, "wget -o") || strings.Contains(joined, "curl ") || strings.Contains(joined, "fallback to wget"):
		stage = "downloading"
	}
	switch {
	case strings.Contains(joined, "sha256 verified"):
		stage = "verifying"
	}
	switch {
	case strings.Contains(joined, "service stopped"):
		stage = "stopping"
	case strings.Contains(joined, "docker compose -f"):
		stage = "recreating"
	}
	switch {
	case strings.Contains(joined, "binary replaced"):
		stage = "replacing"
	}
	switch {
	case strings.Contains(joined, "service started"):
		stage = "restarting"
	}

	if status == "success" {
		return "completed", 100, true
	}
	if status == "downloaded" {
		return "downloaded", 100, true
	}
	if status == "applying" {
		if stage == "pending" {
			stage = "applying"
		}
		return stage, 100, true
	}
	if status == "failed" {
		if !progressKnown {
			progress = 0
		}
		return "failed", progress, progressKnown
	}
	if status == "running" && !progressKnown {
		if stage == "pending" {
			stage = "running"
		}
		return stage, 0, false
	}
	return stage, progress, progressKnown
}

func extractCurrentRunLines(lines []string) []string {
	lastStart := -1
	for i, line := range lines {
		if strings.Contains(line, "binary update started") || strings.Contains(line, "docker update started") {
			lastStart = i
		}
	}
	if lastStart >= 0 {
		return lines[lastStart:]
	}
	return lines
}

func extractProgress(lines []string) (int, bool) {
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(line, "PROGRESS:") {
			continue
		}
		value := strings.TrimSpace(strings.TrimPrefix(line, "PROGRESS:"))
		n, err := strconv.Atoi(value)
		if err != nil {
			continue
		}
		if n < 0 {
			n = 0
		}
		if n > 100 {
			n = 100
		}
		return n, true
	}
	return 0, false
}

func writeRuntimeUpdateState(state runtimeUpdateState) {
	content := fmt.Sprintf(
		"status=%s\nstage=%s\nprogress=%d\nprogress_known=%t\nlast_update_time=%s\nlast_error=%s\nprepared=%t\nprepared_version=%s\nprepared_binary_path=%s\nowner_pid=%d\nworker_pid=%d\n",
		state.Status,
		state.Stage,
		state.Progress,
		state.ProgressKnown,
		state.LastUpdateTime,
		state.LastError,
		state.Prepared,
		state.PreparedVersion,
		state.PreparedBinaryPath,
		state.OwnerPID,
		state.WorkerPID,
	)
	_ = atomicWriteFile(updateStatePath, []byte(content), 0644)
}

func readRuntimeUpdateState() (runtimeUpdateState, bool) {
	data, err := os.ReadFile(updateStatePath)
	if err != nil {
		return runtimeUpdateState{}, false
	}
	state := runtimeUpdateState{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || !strings.Contains(line, "=") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		key, value := parts[0], parts[1]
		switch key {
		case "status":
			state.Status = value
		case "stage":
			state.Stage = value
		case "progress":
			if n, err := strconv.Atoi(value); err == nil {
				state.Progress = n
			}
		case "progress_known":
			state.ProgressKnown = value == "true"
		case "last_update_time":
			state.LastUpdateTime = value
		case "last_error":
			state.LastError = value
		case "prepared":
			state.Prepared = value == "true"
		case "prepared_version":
			state.PreparedVersion = value
		case "prepared_binary_path":
			state.PreparedBinaryPath = value
		case "owner_pid":
			if n, err := strconv.Atoi(value); err == nil {
				state.OwnerPID = n
			}
		case "worker_pid":
			if n, err := strconv.Atoi(value); err == nil {
				state.WorkerPID = n
			}
		}
	}
	if state.Status == "" {
		return runtimeUpdateState{}, false
	}
	return state, true
}

func clearRuntimeUpdateState() {
	_ = os.Remove(updateStatePath)
}

func (s *UpdateService) reconcileUpdateArtifacts() error {
	if err := s.reconcileExternalUpdateResult(); err != nil {
		return err
	}
	settings, err := s.loadSettings()
	if err != nil {
		return err
	}
	if err := s.reconcilePreparedBinaryState(settings); err != nil {
		return err
	}
	return s.reconcileRuntimeUpdateState(settings)
}

func (s *UpdateService) reconcileExternalUpdateResult() error {
	result, ok := readExternalUpdateResult()
	if !ok {
		return nil
	}

	settings, err := s.loadSettings()
	if err != nil {
		return err
	}
	settings.LastUpdateStatus = strings.TrimSpace(result.Status)
	settings.LastUpdateError = strings.TrimSpace(result.LastError)
	settings.LastUpdateTime = strings.TrimSpace(result.LastUpdateTime)
	if settings.LastUpdateTime == "" {
		settings.LastUpdateTime = time.Now().Format(time.RFC3339)
	}
	if result.ClearPrepared {
		settings.PreparedBinaryPath = ""
		settings.PreparedVersion = ""
	} else {
		if path := strings.TrimSpace(result.PreparedBinaryPath); path != "" {
			settings.PreparedBinaryPath = path
		}
		if version := strings.TrimSpace(result.PreparedVersion); version != "" {
			settings.PreparedVersion = version
		}
	}
	if err := s.saveSettings(settings); err != nil {
		return err
	}

	if result.Status == "success" {
		_ = os.Remove(updateLogPath)
	}
	clearRuntimeUpdateState()
	_ = os.Remove(updateResultPath)
	return nil
}

func (s *UpdateService) reconcilePreparedBinaryState(settings *UpdateSettings) error {
	preparedPath := strings.TrimSpace(settings.PreparedBinaryPath)
	if preparedPath == "" {
		return nil
	}
	if _, err := os.Stat(preparedPath); err == nil {
		return nil
	}
	settings.PreparedBinaryPath = ""
	settings.PreparedVersion = ""
	if settings.LastUpdateStatus == "downloaded" || settings.LastUpdateStatus == "applying" {
		settings.LastUpdateStatus = "failed"
		settings.LastUpdateError = "prepared update package is missing; please download it again"
		settings.LastUpdateTime = time.Now().Format(time.RFC3339)
	}
	clearRuntimeUpdateState()
	return s.saveSettings(settings)
}

func (s *UpdateService) reconcileRuntimeUpdateState(settings *UpdateSettings) error {
	state, ok := readRuntimeUpdateState()
	if !ok {
		if settings.LastUpdateStatus == "running" || settings.LastUpdateStatus == "applying" {
			if s.DetectDeployConfig().Mode == "docker" && settings.LastUpdateStatus == "running" {
				return nil
			}
			settings.LastUpdateStatus = "failed"
			if strings.TrimSpace(settings.LastUpdateError) == "" {
				settings.LastUpdateError = "previous update task was interrupted"
			}
			settings.LastUpdateTime = time.Now().Format(time.RFC3339)
			return s.saveSettings(settings)
		}
		return nil
	}

	deploy := s.DetectDeployConfig()
	switch state.Status {
	case "running":
		if deploy.Mode == "docker" {
			if timestampOlderThan(state.LastUpdateTime, 10*time.Minute) {
				return s.markInterruptedUpdate(settings, "previous docker update task did not complete")
			}
			return nil
		}
		if state.OwnerPID == 0 || state.OwnerPID != os.Getpid() {
			return s.markInterruptedUpdate(settings, "previous download task was interrupted")
		}
	case "applying":
		if state.WorkerPID == 0 {
			if state.OwnerPID == os.Getpid() && !timestampOlderThan(state.LastUpdateTime, 30*time.Second) {
				return nil
			}
			return s.markInterruptedUpdate(settings, "previous apply task was interrupted")
		}
		if !processExists(state.WorkerPID) {
			return s.markInterruptedUpdate(settings, "previous apply task was interrupted")
		}
	case "downloaded":
		if preparedPath := strings.TrimSpace(state.PreparedBinaryPath); preparedPath != "" {
			if _, err := os.Stat(preparedPath); err != nil {
				return s.markMissingPreparedUpdate(settings)
			}
		}
	}

	return nil
}

func (s *UpdateService) markInterruptedUpdate(settings *UpdateSettings, message string) error {
	settings.LastUpdateStatus = "failed"
	settings.LastUpdateError = message
	settings.LastUpdateTime = time.Now().Format(time.RFC3339)
	clearRuntimeUpdateState()
	return s.saveSettings(settings)
}

func (s *UpdateService) markMissingPreparedUpdate(settings *UpdateSettings) error {
	settings.PreparedBinaryPath = ""
	settings.PreparedVersion = ""
	settings.LastUpdateStatus = "failed"
	settings.LastUpdateError = "prepared update package is missing; please download it again"
	settings.LastUpdateTime = time.Now().Format(time.RFC3339)
	clearRuntimeUpdateState()
	return s.saveSettings(settings)
}

func startDetachedUpdateScript(scriptPath string) error {
	if _, err := exec.LookPath("systemd-run"); err != nil {
		return errors.New("systemd-run is required for binary update apply")
	}

	unitName := fmt.Sprintf("rabbit-panel-updater-%d", time.Now().UnixNano())
	cmd := exec.Command("systemd-run", "--unit", unitName, "/bin/sh", scriptPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			message = err.Error()
		}
		return fmt.Errorf("failed to start updater helper: %s", message)
	}
	return nil
}

func readExternalUpdateResult() (externalUpdateResult, bool) {
	data, err := os.ReadFile(updateResultPath)
	if err != nil {
		return externalUpdateResult{}, false
	}
	result := externalUpdateResult{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || !strings.Contains(line, "=") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		key, value := parts[0], parts[1]
		switch key {
		case "status":
			result.Status = value
		case "last_update_time":
			result.LastUpdateTime = value
		case "last_error":
			result.LastError = value
		case "clear_prepared":
			result.ClearPrepared = value == "true"
		case "prepared_binary_path":
			result.PreparedBinaryPath = value
		case "prepared_version":
			result.PreparedVersion = value
		}
	}
	if result.Status == "" {
		return externalUpdateResult{}, false
	}
	return result, true
}

func updateTaskStillActive(state runtimeUpdateState) bool {
	switch state.Status {
	case "running":
		return state.OwnerPID != 0 && state.OwnerPID == os.Getpid()
	case "applying":
		if state.WorkerPID != 0 {
			return processExists(state.WorkerPID)
		}
		return state.OwnerPID != 0 && state.OwnerPID == os.Getpid() && !timestampOlderThan(state.LastUpdateTime, 30*time.Second)
	default:
		return false
	}
}

func processExists(pid int) bool {
	if pid <= 0 {
		return false
	}
	if runtime.GOOS != "linux" {
		return false
	}
	_, err := os.Stat(filepath.Join("/proc", strconv.Itoa(pid)))
	return err == nil
}

func atomicWriteFile(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	tmpFile, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()
	defer func() {
		_ = os.Remove(tmpPath)
	}()
	if _, err := tmpFile.Write(data); err != nil {
		_ = tmpFile.Close()
		return err
	}
	if err := tmpFile.Chmod(mode); err != nil {
		_ = tmpFile.Close()
		return err
	}
	if err := tmpFile.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

func sanitizeManifestValue(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, `"'`)
	return value
}

func (s *UpdateService) reconcileDockerUpdateStatus(settings *UpdateSettings, currentVersion, latestVersion string) (bool, error) {
	if settings.LastUpdateStatus != "running" {
		return false, nil
	}

	if currentVersion != "" && currentVersion != "dev" && latestVersion != "" {
		cmp, err := compareVersions(currentVersion, latestVersion)
		if err == nil && cmp >= 0 {
			settings.LastUpdateStatus = "success"
			settings.LastUpdateError = ""
			settings.LastUpdateTime = time.Now().Format(time.RFC3339)
			if err := s.saveSettings(settings); err != nil {
				return false, err
			}
			_ = os.Remove(updateLogPath)
			clearRuntimeUpdateState()
			return true, nil
		}
	}

	if timestampOlderThan(settings.LastUpdateTime, 10*time.Minute) {
		settings.LastUpdateStatus = "failed"
		if strings.TrimSpace(settings.LastUpdateError) == "" {
			settings.LastUpdateError = "previous docker update task did not complete"
		}
		settings.LastUpdateTime = time.Now().Format(time.RFC3339)
		if err := s.saveSettings(settings); err != nil {
			return false, err
		}
		clearRuntimeUpdateState()
		return true, nil
	}

	return false, nil
}

func timestampOlderThan(value string, duration time.Duration) bool {
	if strings.TrimSpace(value) == "" {
		return true
	}
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return true
	}
	return time.Since(t) > duration
}

type sha256Writer struct {
	hash hashWriter
}

type hashWriter interface {
	Write(p []byte) (n int, err error)
	Sum(b []byte) []byte
}

func newSHA256Writer() *sha256Writer {
	return &sha256Writer{hash: sha256.New()}
}

func (w *sha256Writer) Write(p []byte) (n int, err error) {
	return w.hash.Write(p)
}

func (w *sha256Writer) Sum() string {
	return fmt.Sprintf("%x", w.hash.Sum(nil))
}
