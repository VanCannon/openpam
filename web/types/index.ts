export interface User {
  id: string
  email: string
  display_name: string
  created_at: string
  updated_at: string
}

export interface Zone {
  id: string
  name: string
  type: 'hub' | 'satellite'
  description?: string
  created_at: string
  updated_at: string
}

export interface Target {
  id: string
  zone_id: string
  name: string
  hostname: string
  protocol: 'ssh' | 'rdp'
  port: number
  description?: string
  enabled: boolean
  created_at: string
  updated_at: string
}

export interface Credential {
  id: string
  target_id: string
  username: string
  description?: string
}

export interface AuditLog {
  id: string
  user_id: string
  target_id: string
  protocol: string
  status: 'active' | 'completed' | 'failed'
  started_at: string
  ended_at?: string
  bytes_sent?: number
  bytes_received?: number
  error_message?: string
}

export interface ApiResponse<T> {
  data?: T
  error?: string
  message?: string
}

export interface ListResponse<T> {
  items: T[]
  total: number
  page?: number
  page_size?: number
}
