import request from '@/utils/request'
import type { FileInfo } from '@/types'

/**
 * Saved SSH credential (password never leaves the server)
 */
export interface SshCredential {
  id: number
  node_id: string
  host: string
  port: number
  username: string
  created_at: string
}

export interface SshCredentialSaveRequest {
  node_id: string
  host: string
  port: number
  username: string
  password: string
}

/** SSH/SFTP 连接参数：credential_id 走已保存凭证（密码不出服务端），否则明文参数 */
export interface SshConnParams {
  credential_id?: number
  host?: string
  port?: number
  username?: string
  password?: string
}

/**
 * SSH API — username/password terminal access to managed nodes
 */
export const sshApi = {
  /**
   * List saved SSH credentials for a node (no passwords)
   */
  async listCredentials(nodeId: string): Promise<SshCredential[]> {
    const response = await request.get<SshCredential[]>('/ssh/credentials', {
      params: { node_id: nodeId },
    })
    return response.data ?? []
  },

  /**
   * Save a username/password SSH credential for a node
   */
  async saveCredential(data: SshCredentialSaveRequest): Promise<SshCredential> {
    const response = await request.post<SshCredential>('/ssh/credentials', data)
    return response.data
  },

  /**
   * Delete a saved SSH credential
   */
  async deleteCredential(id: number): Promise<void> {
    await request.delete('/ssh/credentials', { params: { id } })
  },

  /**
   * WebSocket URL for the SSH terminal session.
   * After connecting, the client sends a JSON `connect` message first.
   */
  getTerminalWsUrl(): string {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    return `${protocol}//${window.location.host}/api/ssh/terminal`
  },

  // ---- SFTP 文件管理（连接参数与终端同一套：credential_id 或明文 host/port/username/password） ----

  async sftpList(conn: SshConnParams, path: string): Promise<FileInfo[]> {
    const response = await request.post<FileInfo[]>('/ssh/sftp/list', { ...conn, path })
    return response.data ?? []
  },

  async sftpRead(conn: SshConnParams, path: string): Promise<string> {
    const response = await request.post<{ content: string }>('/ssh/sftp/read', { ...conn, path })
    return response.data.content
  },

  async sftpWrite(conn: SshConnParams, path: string, content: string): Promise<void> {
    await request.post('/ssh/sftp/write', { ...conn, path, content })
  },

  async sftpMkdir(conn: SshConnParams, path: string): Promise<void> {
    await request.post('/ssh/sftp/mkdir', { ...conn, path })
  },

  async sftpDelete(conn: SshConnParams, path: string): Promise<void> {
    await request.post('/ssh/sftp/delete', { ...conn, path })
  },

  async sftpRename(conn: SshConnParams, path: string, newPath: string): Promise<void> {
    await request.post('/ssh/sftp/rename', { ...conn, path, new_path: newPath })
  },

  async sftpUpload(conn: SshConnParams, dir: string, name: string, contentBase64: string): Promise<void> {
    await request.post('/ssh/sftp/upload', { ...conn, path: dir, name, content: contentBase64 })
  },

  /** 下载远端文件为 Blob，由调用方触发保存 */
  async sftpDownload(conn: SshConnParams, path: string): Promise<Blob> {
    const response = await request.post('/ssh/sftp/download', { ...conn, path }, { responseType: 'blob' })
    return response.data as Blob
  },
}

export default sshApi
