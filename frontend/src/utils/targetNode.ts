const STORAGE_KEY = 'rabbit-target-node'

let targetNodeId = ''
try {
  targetNodeId = sessionStorage.getItem(STORAGE_KEY) || ''
} catch {
  targetNodeId = ''
}

export function getTargetNodeId(): string {
  return targetNodeId
}

export function setTargetNodeId(id: string): void {
  targetNodeId = id
  try {
    if (id) {
      sessionStorage.setItem(STORAGE_KEY, id)
    } else {
      sessionStorage.removeItem(STORAGE_KEY)
    }
  } catch {
    // ignore storage failures
  }
}

export function targetNodeHeaders(): Record<string, string> {
  return targetNodeId ? { 'X-Target-Node': targetNodeId } : {}
}

export function withTargetNode(url: string): string {
  if (!targetNodeId) return url
  const join = url.includes('?') ? '&' : '?'
  return `${url}${join}target_node=${encodeURIComponent(targetNodeId)}`
}
