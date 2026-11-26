// 镜像引用与字段校验。与后端 router/image_favorites.go 的正则保持一致：
// 这些字段会拼进用户复制执行的 docker pull 命令，必须拒绝一切 shell 注入字符。
const REPO_NAME_RE = /^[a-z0-9]+(?:[._-][a-z0-9]+)*(?:\/[a-z0-9]+(?:[._-][a-z0-9]+)*)*$/
const TAG_RE = /^[A-Za-z0-9_][A-Za-z0-9_.-]{0,127}$/
const HOST_RE = /^[A-Za-z0-9]([A-Za-z0-9.-]*[A-Za-z0-9])?(:[0-9]{1,5})?$/

export function isValidRepoName(name: unknown): name is string {
  return typeof name === 'string' && name.length > 0 && name.length <= 200 && REPO_NAME_RE.test(name)
}

export function isValidTag(tag: unknown): tag is string {
  return typeof tag === 'string' && TAG_RE.test(tag)
}

// 空串合法（字段可留空）；非空必须是 hostname[:port]
export function isValidHost(host: unknown): host is string {
  return typeof host === 'string' && (host === '' || (host.length <= 100 && HOST_RE.test(host)))
}

export type PullSource = 'mirror' | 'origin'

export interface SearchItem {
  id: number
  name: string
  namespace: string
  registry: string
  repository_name: string
  pull_domain: string
  logo_url: string
  description: string
  pull_count: number
  star_count: number
  is_official: boolean
}

function toNonNegativeInt(value: unknown): number {
  const n = typeof value === 'number' ? value : Number(value)
  if (!Number.isFinite(n) || n < 0) return 0
  return Math.floor(n)
}

function toText(value: unknown): string {
  return typeof value === 'string' ? value : ''
}

function sanitizeLogoUrl(value: unknown): string {
  if (typeof value !== 'string' || value.length > 300 || /\s/.test(value)) return ''
  return value.startsWith('https://') || value.startsWith('http://') ? value : ''
}

// sanitizeSearchItem 校验一条搜索结果；repository_name/pull_domain/registry
// 非法的条目直接返回 null 丢弃，logo_url 非法则静默置空。
export function sanitizeSearchItem(raw: unknown): SearchItem | null {
  if (!raw || typeof raw !== 'object') return null
  const obj = raw as Record<string, unknown>
  const repositoryName = toText(obj.repository_name)
  const registry = toText(obj.registry)
  const pullDomain = toText(obj.pull_domain)
  if (!isValidRepoName(repositoryName) || !isValidHost(registry) || !isValidHost(pullDomain)) {
    return null
  }
  return {
    id: toNonNegativeInt(obj.id),
    name: toText(obj.name),
    namespace: toText(obj.namespace),
    registry,
    repository_name: repositoryName,
    pull_domain: pullDomain,
    logo_url: sanitizeLogoUrl(obj.logo_url),
    description: toText(obj.description),
    pull_count: toNonNegativeInt(obj.pull_count),
    star_count: toNonNegativeInt(obj.star_count),
    is_official: obj.is_official === true,
  }
}

// 构建 docker pull 所需的最小字段；搜索结果与收藏记录共用
export interface ImageRefSource {
  repository_name: string
  pull_domain: string
  registry: string
}

export function resolveTag(tag: string | null | undefined): string {
  const trimmed = (tag ?? '').trim()
  return isValidTag(trimmed) ? trimmed : 'latest'
}

// buildImageRef 生成完整镜像引用。mirror 优先用加速域名，没有则回退官方源；
// docker.io/空 registry 的官方源引用去掉多余的 library/ 前缀。
export function buildImageRef(item: ImageRefSource, tag: string, source: PullSource): string {
  const resolvedTag = resolveTag(tag)
  if (source === 'mirror' && item.pull_domain) {
    return `${item.pull_domain}/${item.repository_name}:${resolvedTag}`
  }
  if (!item.registry || item.registry === 'docker.io') {
    return `${item.repository_name.replace(/^library\//, '')}:${resolvedTag}`
  }
  return `${item.registry}/${item.repository_name}:${resolvedTag}`
}

export function buildPullCommand(item: ImageRefSource, tag: string, source: PullSource): string {
  return `docker pull ${buildImageRef(item, tag, source)}`
}
