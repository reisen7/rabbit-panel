import request from '@/utils/request'
import { sanitizeSearchItem, type SearchItem } from '@/utils/imageRef'

// 1ms.run 镜像搜索接口允许跨域（Access-Control-Allow-Origin: *），
// 浏览器直接 fetch，不走带认证/节点头的 axios 实例。
const SEARCH_URL = 'https://1ms.run/api/v1/registry/search'

export interface RegistrySearchResult {
  items: SearchItem[]
  total: number
  page: number
}

export async function searchRegistry(
  query: string,
  page = 1,
  pageSize = 12,
  signal?: AbortSignal
): Promise<RegistrySearchResult> {
  const url = `${SEARCH_URL}?query=${encodeURIComponent(query)}&page=${page}&page_size=${pageSize}`
  const response = await fetch(url, { signal })
  if (!response.ok) {
    throw new Error(`HTTP ${response.status}`)
  }
  const body = await response.json()
  if (body?.code !== 0) {
    throw new Error(typeof body?.msg === 'string' && body.msg ? body.msg : 'search failed')
  }
  const list: unknown[] = Array.isArray(body?.data?.list) ? body.data.list : []
  const items = list
    .map(sanitizeSearchItem)
    .filter((item): item is SearchItem => item !== null)
  return {
    items,
    total: Number(body?.data?.total) || 0,
    page: Number(body?.data?.page) || page,
  }
}

export interface ImageFavoritePayload {
  repository_name: string
  registry: string
  pull_domain: string
  tag: string
  description: string
  logo_url: string
  pull_count: number
  star_count: number
  is_official: boolean
}

export interface ImageFavoriteRecord extends ImageFavoritePayload {
  id: number
  created_at: string
}

export const imageFavoriteApi = {
  async list(): Promise<ImageFavoriteRecord[]> {
    const response = await request.get<ImageFavoriteRecord[]>('/image-favorites')
    return Array.isArray(response.data) ? response.data : []
  },

  async add(payload: ImageFavoritePayload): Promise<ImageFavoriteRecord> {
    const response = await request.post<ImageFavoriteRecord>('/image-favorites', payload)
    return response.data
  },

  async remove(id: number): Promise<void> {
    await request.delete('/image-favorites', { params: { id } })
  },
}
