'use client'

import { useAuth } from '@/lib/auth-context'
import { api } from '@/lib/api'
import { SystemAuditLog } from '@/types'
import { useRouter } from 'next/navigation'
import { useEffect, useState } from 'react'
import Header from '@/components/header'

export default function AuditLogsPage() {
  const { user, loading } = useAuth()
  const router = useRouter()
  const [auditLogs, setAuditLogs] = useState<SystemAuditLog[]>([])
  const [loadingLogs, setLoadingLogs] = useState(true)
  const [filter, setFilter] = useState<string>('all')

  useEffect(() => {
    if (!loading && !user) {
      router.push('/login')
    }
  }, [user, loading, router])

  useEffect(() => {
    if (user) {
      loadAuditLogs()
    }
  }, [user, filter])

  const loadAuditLogs = async () => {
    try {
      setLoadingLogs(true)
      const params = filter !== 'all' ? { event_type: filter } : {}
      const response = await api.listSystemAuditLogs(params)
      setAuditLogs(response.logs || [])
    } catch (error) {
      console.error('Failed to load system audit logs:', error)
    } finally {
      setLoadingLogs(false)
    }
  }

  const formatDate = (dateString?: string) => {
    if (!dateString) return '-'
    return new Date(dateString).toLocaleString()
  }

  const getEventTypeBadge = (eventType: string) => {
    const typeMap: Record<string, { bg: string; text: string; label: string }> = {
      login_success: { bg: 'bg-green-100', text: 'text-green-800', label: 'Login Success' },
      login_failed: { bg: 'bg-red-100', text: 'text-red-800', label: 'Login Failed' },
      logout: { bg: 'bg-gray-100', text: 'text-gray-800', label: 'Logout' },
      user_created: { bg: 'bg-blue-100', text: 'text-blue-800', label: 'User Created' },
      user_updated: { bg: 'bg-yellow-100', text: 'text-yellow-800', label: 'User Updated' },
      user_deleted: { bg: 'bg-red-100', text: 'text-red-800', label: 'User Deleted' },
      target_created: { bg: 'bg-blue-100', text: 'text-blue-800', label: 'Target Created' },
      target_updated: { bg: 'bg-yellow-100', text: 'text-yellow-800', label: 'Target Updated' },
      target_deleted: { bg: 'bg-red-100', text: 'text-red-800', label: 'Target Deleted' },
      credential_created: { bg: 'bg-blue-100', text: 'text-blue-800', label: 'Credential Created' },
      credential_updated: { bg: 'bg-yellow-100', text: 'text-yellow-800', label: 'Credential Updated' },
      credential_deleted: { bg: 'bg-red-100', text: 'text-red-800', label: 'Credential Deleted' },
      session_started: { bg: 'bg-green-100', text: 'text-green-800', label: 'Session Started' },
      session_ended: { bg: 'bg-gray-100', text: 'text-gray-800', label: 'Session Ended' },
    }
    const config = typeMap[eventType] || { bg: 'bg-gray-100', text: 'text-gray-800', label: eventType }
    return (
      <span className={`px-2 py-1 text-xs font-semibold rounded ${config.bg} ${config.text}`}>
        {config.label}
      </span>
    )
  }

  const getStatusBadge = (status: string) => {
    const statusMap: Record<string, { bg: string; text: string }> = {
      success: { bg: 'bg-green-100', text: 'text-green-800' },
      failure: { bg: 'bg-red-100', text: 'text-red-800' },
      pending: { bg: 'bg-yellow-100', text: 'text-yellow-800' },
    }
    const config = statusMap[status] || { bg: 'bg-gray-100', text: 'text-gray-800' }
    return (
      <span className={`px-2 py-1 text-xs font-semibold rounded ${config.bg} ${config.text}`}>
        {status.toUpperCase()}
      </span>
    )
  }

  if (loading || !user) {
    return <div className="flex min-h-screen items-center justify-center"><p>Loading...</p></div>
  }

  return (
    <div className="min-h-screen bg-gray-50">
      <Header />

      <main className="w-full px-4 sm:px-6 lg:px-8 py-8">
        <div className="mb-6 flex justify-between items-center">
          <div>
            <h1 className="text-2xl font-bold text-gray-900">Audit Logs</h1>
            <p className="text-sm text-gray-600 mt-1">System events and activity logs</p>
          </div>

          <div className="flex gap-2">
            <select
              value={filter}
              onChange={(e) => setFilter(e.target.value)}
              className="px-4 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-indigo-500"
            >
              <option value="all">All Events</option>
              <option value="login_success">Login Success</option>
              <option value="login_failed">Login Failed</option>
              <option value="logout">Logout</option>
              <option value="user_created">User Created</option>
              <option value="user_updated">User Updated</option>
              <option value="user_deleted">User Deleted</option>
              <option value="target_created">Target Created</option>
              <option value="target_updated">Target Updated</option>
              <option value="target_deleted">Target Deleted</option>
              <option value="credential_created">Credential Created</option>
              <option value="credential_updated">Credential Updated</option>
              <option value="credential_deleted">Credential Deleted</option>
              <option value="session_started">Session Started</option>
              <option value="session_ended">Session Ended</option>
            </select>
          </div>
        </div>

        {loadingLogs ? (
          <div className="text-center py-12"><p className="text-gray-500">Loading...</p></div>
        ) : auditLogs.length === 0 ? (
          <div className="text-center py-12"><p className="text-gray-500">No audit logs found</p></div>
        ) : (
          <div className="bg-white shadow rounded-lg overflow-hidden">
            <div className="overflow-x-auto">
              <table className="min-w-full divide-y divide-gray-200">
                <thead className="bg-gray-50">
                  <tr>
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Timestamp</th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Event Type</th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">User</th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Action</th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Resource</th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Status</th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">IP Address</th>
                  </tr>
                </thead>
                <tbody className="bg-white divide-y divide-gray-200">
                  {auditLogs.map((log) => (
                    <tr key={log.id} className="hover:bg-gray-50">
                      <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-900">
                        {formatDate(log.timestamp)}
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap">
                        {getEventTypeBadge(log.event_type)}
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                        {log.user_id || '-'}
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                        {log.action}
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                        {log.resource_name || log.resource_type || '-'}
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap">
                        {getStatusBadge(log.status)}
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                        {log.ip_address || '-'}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        )}
      </main>
    </div>
  )
}
