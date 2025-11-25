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
  credential_id?: {
    UUID: string
    Valid: boolean
  }
  start_time: string
  end_time?: {
    Time: string
    Valid: boolean
  }
  bytes_sent?: number
  bytes_received?: number
  session_status: 'active' | 'completed' | 'failed' | 'terminated'
  client_ip?: string
  error_message?: string
  recording_path?: string
  protocol: string
  created_at: string
}

export interface ApiResponse<T> {
  data?: T
  error?: string
  message?: string
}

export interface ListResponse<T> {
  items?: T[]
  logs?: T[]
  sessions?: T[]
  total?: number
  count?: number
  page?: number
  page_size?: number
  limit?: number
  offset?: number
}
