// Container Types
export interface ContainerInfo {
  id: string
  name: string
  image: string
  status: string
  ports: string
  memory: string
  created: string
  state: 'running' | 'exited' | 'paused' | 'created'
}

export interface ContainerConfig {
  id: string
  fullId: string
  name: string
  image: string
  imageId: string
  created: string
  started: string
  finished: string
  state: string
  running: boolean
  paused: boolean
  pid: number
  exitCode: number
  platform: string
  hostname: string
  domainname: string
  networkMode: string
  ports: Array<{ host: string; container: string; hostIP: string }>
  dns: string[]
  dnsSearch: string[]
  extraHosts: string[]
  macAddress: string
  ipAddress: string
  gateway: string
  volumes: Array<{ host: string; container: string; mode: string }>
  workingDir: string
  readOnly: boolean
  env: Array<{ key: string; value: string }>
  cmd: string[]
  entrypoint: string[]
  user: string
  tty: boolean
  stdin: boolean
  restart: string
  restartMaxRetry: number
  memory: number
  memorySwap: number
  memoryRes: number
  cpus: number
  cpuShares: number
  cpusetCpus: string
  cpusetMems: string
  cpuPeriod: number
  cpuQuota: number
  pidsLimit: number
  oomKillDisable: boolean
  privileged: boolean
  capAdd: string[]
  capDrop: string[]
  securityOpt: string[]
  labels: Record<string, string>
  logDriver: string
  logOptions: Record<string, string>
}

export interface ContainerStats {
  cpu_percent: number
  cpu_cores: number
  memory_usage: number
  memory_limit: number
  memory_percent: number
  has_memory_limit: boolean  // 是否设置了内存限制
  network_rx: number
  network_tx: number
  block_read: number
  block_write: number
  pids: number
}

export interface CreateContainerRequest {
  image: string
  name: string
  restart: string
  network: string
  ports: Array<{ host: string; container: string }>
  env: Array<{ key: string; value: string }>
  volumes: Array<{ host: string; container: string }>
}

export interface RecreateContainerRequest {
  container_id: string
  image: string
  name: string
  restart: string
  network: string
  ports: Array<{ host: string; container: string }>
  env: Array<{ key: string; value: string }>
  volumes: Array<{ host: string; container: string }>
}


// File Types
export interface FileInfo {
  name: string
  path: string
  size: number
  mode: string
  mod_time: string
  is_dir: boolean
  is_link?: boolean
}

// Image Types
export interface ImageInfo {
  id: string
  name: string
  tag: string
  size: string
  created: string
  in_use: boolean
  used_by: string[]
  used_by_count: number
}

// Network Types
export interface NetworkInfo {
  id: string
  name: string
  driver: string
  scope: string
  ipam: string
  containers: string[]
  container_count: number
  in_use: boolean
  internal: boolean
  created: string
}

export interface NetworkDetail extends NetworkInfo {
  gateway: string
  subnet: string
  connectedContainers: Array<{
    id: string
    name: string
    ipAddress: string
    macAddress: string
  }>
}

// Compose Types
export interface ComposeProject {
  name: string
  status: 'running' | 'partial' | 'stopped' | 'unknown'
  containers?: ComposeContainer[]
}

export interface ComposeContainer {
  id: string
  name: string
  service: string
  state: string
  status: string
  ports: string
}

// System Types
export interface SystemStats {
  cpu: number
  memory: number
  memoryUsed: number  // 已用内存 (KB)
  memoryTotal: number // 总内存 (KB)
  disk: number
  diskUsed: number    // 已用磁盘 (KB)
  diskTotal: number   // 总磁盘 (KB)
  time: string
}

// Node Types
export interface NodeInfo {
  id: string
  name: string
  address: string
  mode: 'master' | 'worker'
  status: 'online' | 'offline' | 'error'
  cpu: number
  memory: number
  disk: number
  containers: number
  last_seen: string
  labels: Record<string, string>
}

// Auth Types
export interface LoginRequest {
  username: string
  password: string
  captcha_id?: string
  captcha?: string
}

export interface CaptchaResponse {
  captcha_id: string
  image: string
}

export interface LoginResponse {
  token: string
  need_change_password: boolean
  message: string
}

export interface ChangePasswordRequest {
  old_password: string
  new_password: string
}

export interface UserInfo {
  username: string
  need_change_password: boolean
}

// Volume Types
export interface VolumeInfo {
  name: string
  driver: string
  mountpoint: string
  scope: string
  created: string
  labels: Record<string, string>
  usageData?: {
    size: number
    refCount: number
  }
  containers: string[]
  container_count: number
  in_use: boolean
}

export interface CreateVolumeRequest {
  name: string
  driver?: string
  driverOpts?: Record<string, string>
  labels?: Record<string, string>
}

export interface VolumePruneResult {
  volumesDeleted: string[]
  spaceReclaimed: number
}

// Registry Types
export interface RegistryInfo {
  id: string
  name: string
  url: string
  username?: string
  isDefault: boolean
  createdAt: string
}

export interface CreateRegistryRequest {
  name: string
  url: string
  username?: string
  password?: string
}

export interface RegistryTestResult {
  success: boolean
  message: string
  latency?: number
}

// Docker Config Types
export interface DockerConfig {
  registryMirrors: string[]
  insecureRegistries: string[]
  ipv6: boolean
  fixedCidrV6?: string
  logDriver: string
  logOpts: Record<string, string>
  iptables: boolean
  liveRestore: boolean
  cgroupDriver: 'cgroupfs' | 'systemd'
  dataRoot?: string
  storageDriver?: string
}

export interface DockerInfo {
  serverVersion: string
  apiVersion: string
  os: string
  arch: string
  kernelVersion: string
  operatingSystem: string
  containers: number
  containersRunning: number
  containersPaused: number
  containersStopped: number
  images: number
  driver: string
  dockerRootDir: string
  memoryLimit: boolean
  swapLimit: boolean
  cpuCfsPeriod: boolean
  cpuCfsQuota: boolean
}

// Theme and Language Types
export type Theme = 'light' | 'dark'
export type Language = 'zh-CN' | 'en-US'

// API Response Types
export interface ApiResponse<T> {
  data?: T
  error?: string
  status: number
}

export interface SystemUpdateInfo {
  current_version: string
  current_commit: string
  current_build_time: string
  latest_version: string
  has_update: boolean
  deploy_mode: 'docker' | 'binary' | string
  image: string
  image_tag: string
  can_update: boolean
  ignored_version: string
  ignored: boolean
  release_url: string
  release_notes: string
  message: string
  last_check_time: string
  last_update_time: string
  last_update_status: string
  last_update_error: string
}

export interface UpdateTaskStatus {
  status: string
  stage: string
  progress: number
  progress_known: boolean
  last_update_time: string
  last_error: string
  log_lines: string[]
}
