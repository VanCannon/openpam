import { User, Zone, Target, Credential, AuditLog, SystemAuditLog, ListResponse } from '@/types'

const API_URL = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080'

class ApiClient {
  private baseUrl: string
  private token: string | null = null

  constructor(baseUrl: string) {
    this.baseUrl = baseUrl
    if (typeof window !== 'undefined') {
      this.token = localStorage.getItem('openpam_token')
    }
  }

  setToken(token: string | null) {
    this.token = token
    if (typeof window !== 'undefined') {
      if (token) {
        localStorage.setItem('openpam_token', token)
      } else {
        localStorage.removeItem('openpam_token')
      }
    }
  }

  private async request<T>(
    path: string,
    options: RequestInit = {}
  ): Promise<T> {
    // Ensure we are in a browser environment (skip during SSR)
    if (typeof window === 'undefined') {
      return Promise.reject(new Error('API calls can only be made from the browser'))
    }

    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
      'Cache-Control': 'no-store',
      ...options.headers as Record<string, string>,
    }

    if (this.token) {
      headers['Authorization'] = `Bearer ${this.token}`
    }

    try {
      const response = await fetch(`${this.baseUrl}${path}`, {
        ...options,
        headers,
        cache: 'no-store',      // Disable browser caching
        credentials: 'include', // Include cookies for CORS
        mode: 'cors',           // Explicit CORS mode for Next.js 16
      })

      if (!response.ok) {
        const error = await response.text()
        throw new Error(error || `HTTP ${response.status}`)
      }

      if (response.status === 204) {
        return null as unknown as T
      }

      return response.json()
    } catch (error) {
      // Don't log 401 errors as they are often expected (e.g. checkAuth)
      const isUnauthorized = error instanceof Error &&
        (error.message.includes('401') || error.message.includes('Unauthorized'))

      if (!isUnauthorized) {
        console.error('API request failed:', {
          path,
          baseUrl: this.baseUrl,
          error: error instanceof Error ? error.message : String(error),
        })
      }
      throw error
    }
  }

  // Authentication
  async login() {
    window.location.href = `${this.baseUrl}/api/v1/auth/login`
  }

  async logout() {
    await this.request('/api/v1/auth/logout', { method: 'POST' })
    this.setToken(null)
  }

  async getCurrentUser(): Promise<User> {
    return this.request<User>('/api/v1/auth/me')
  }

  // Users
  async listUsers(): Promise<ListResponse<User>> {
    return this.request<ListResponse<User>>('/api/v1/users')
  }

  async getUser(id: string): Promise<User> {
    return this.request<User>(`/api/v1/users/${id}`)
  }

  // Zones
  async listZones(): Promise<ListResponse<Zone>> {
    return this.request<ListResponse<Zone>>('/api/v1/zones')
  }

  async getZone(id: string): Promise<Zone> {
    return this.request<Zone>(`/api/v1/zones?id=${id}`)
  }

  async createZone(zone: Partial<Zone>): Promise<Zone> {
    return this.request<Zone>('/api/v1/zones', {
      method: 'POST',
      body: JSON.stringify(zone),
    })
  }

  async updateZone(id: string, zone: Partial<Zone>): Promise<Zone> {
    return this.request<Zone>(`/api/v1/zones?id=${id}`, {
      method: 'PUT',
      body: JSON.stringify(zone),
    })
  }

  async deleteZone(id: string): Promise<void> {
    return this.request<void>(`/api/v1/zones?id=${id}`, {
      method: 'DELETE',
    })
  }

  // Targets
  async listTargets(params?: { zone_id?: string; page?: number; page_size?: number }): Promise<ListResponse<Target>> {
    const query = new URLSearchParams()
    if (params?.zone_id) query.set('zone_id', params.zone_id)
    if (params?.page) query.set('page', params.page.toString())
    if (params?.page_size) query.set('page_size', params.page_size.toString())

    const queryString = query.toString()
    return this.request<ListResponse<Target>>(`/api/v1/targets${queryString ? '?' + queryString : ''}`)
  }

  async getTarget(id: string): Promise<Target> {
    return this.request<Target>(`/api/v1/targets?id=${id}`)
  }

  async createTarget(target: Partial<Target>): Promise<Target> {
    return this.request<Target>('/api/v1/targets/create', {
      method: 'POST',
      body: JSON.stringify(target),
    })
  }

  async updateTarget(id: string, target: Partial<Target>): Promise<Target> {
    return this.request<Target>(`/api/v1/targets?id=${id}`, {
      method: 'PUT',
      body: JSON.stringify(target),
    })
  }

  async deleteTarget(id: string): Promise<void> {
    return this.request<void>(`/api/v1/targets?id=${id}`, {
      method: 'DELETE',
    })
  }

  // Credentials
  async listCredentials(targetId: string): Promise<{ credentials: Credential[]; count: number }> {
    return this.request<{ credentials: Credential[]; count: number }>(
      `/api/v1/credentials?target_id=${targetId}`
    )
  }

  async createCredential(credential: Partial<Credential>): Promise<Credential> {
    return this.request<Credential>('/api/v1/credentials/create', {
      method: 'POST',
      body: JSON.stringify(credential),
    })
  }

  async updateCredential(id: string, credential: Partial<Credential>): Promise<Credential> {
    return this.request<Credential>(`/api/v1/credentials/update?id=${id}`, {
      method: 'PUT',
      body: JSON.stringify(credential),
    })
  }

  async deleteCredential(id: string): Promise<void> {
    return this.request<void>(`/api/v1/credentials/delete?id=${id}`, {
      method: 'DELETE',
    })
  }

  // Audit Logs
  async listAuditLogs(params?: { user_id?: string; target_id?: string }): Promise<ListResponse<AuditLog>> {
    const query = new URLSearchParams()
    if (params?.user_id) query.set('user_id', params.user_id)
    if (params?.target_id) query.set('target_id', params.target_id)

    const queryString = query.toString()
    return this.request<ListResponse<AuditLog>>(`/api/v1/audit-logs${queryString ? '?' + queryString : ''}`)
  }

  async getAuditLog(id: string): Promise<AuditLog> {
    return this.request<AuditLog>(`/api/v1/audit-logs/${id}`)
  }

  async getActiveSessions(): Promise<{ sessions: AuditLog[] }> {
    return this.request<{ sessions: AuditLog[] }>('/api/v1/audit-logs/active')
  }

  async getRecording(sessionId: string): Promise<string> {
    const headers: Record<string, string> = {}
    if (this.token) {
      headers['Authorization'] = `Bearer ${this.token}`
    }

    const response = await fetch(`${this.baseUrl}/api/v1/audit-logs/recording?session_id=${sessionId}`, {
      headers,
      credentials: 'include',
      mode: 'cors',
    })

    if (!response.ok) {
      const error = await response.text()
      throw new Error(error || `HTTP ${response.status}`)
    }

    return response.text()
  }

  // System Audit Logs
  async listSystemAuditLogs(params?: { event_type?: string; user_id?: string; limit?: number; offset?: number }): Promise<ListResponse<SystemAuditLog>> {
    const query = new URLSearchParams()
    if (params?.event_type) query.set('event_type', params.event_type)
    if (params?.user_id) query.set('user_id', params.user_id)
    if (params?.limit) query.set('limit', params.limit.toString())
    if (params?.offset) query.set('offset', params.offset.toString())

    const queryString = query.toString()
    return this.request<ListResponse<SystemAuditLog>>(`/api/v1/system-audit-logs${queryString ? '?' + queryString : ''}`)
  }

  async getSystemAuditLog(id: string): Promise<SystemAuditLog> {
    return this.request<SystemAuditLog>(`/api/v1/system-audit-logs/${id}`)
  }

  // WebSocket URL for connections
  getWebSocketUrl(protocol: string, targetId: string, credentialId: string): string {
    const wsUrl = process.env.NEXT_PUBLIC_WS_URL || 'ws://localhost:8080'
    // Defensive fix for potential undefined in credentialId
    const cleanCredId = credentialId.replace('?undefined', '')
    let url = `${wsUrl}/api/ws/connect/${protocol}/${targetId}?credential_id=${cleanCredId}`

    // Append auth token if available (required for WebSockets as they don't send headers)
    if (this.token) {
      url += `&token=${this.token}`
    }

    console.log('Generated WebSocket URL:', url)
    return url
  }
}

export const api = new ApiClient(API_URL)
