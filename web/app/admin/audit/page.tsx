'use client'

import { useAuth } from '@/lib/auth-context'
import { api } from '@/lib/api'
import { AuditLog } from '@/types'
import { useRouter } from 'next/navigation'
import { useEffect, useState } from 'react'
import Link from 'next/link'

export default function AuditPage() {
  const { user, loading } = useAuth()
  const router = useRouter()
  const [auditLogs, setAuditLogs] = useState<AuditLog[]>([])
  const [loadingLogs, setLoadingLogs] = useState(true)

  useEffect(() => {
    if (!loading && !user) {
      router.push('/login')
    }
  }, [user, loading, router])

  useEffect(() => {
    if (user) {
      loadAuditLogs()
    }
  }, [user])

  const loadAuditLogs = async () => {
    try {
      setLoadingLogs(true)
      const response = await api.listAuditLogs()
      setAuditLogs(response.items || [])
    } catch (error) {
      console.error('Failed to load audit logs:', error)
    } finally {
      setLoadingLogs(false)
    }
  }

  const formatDate = (dateString: string) => {
    return new Date(dateString).toLocaleString()
  }

  const formatBytes = (bytes?: number) => {
    if (!bytes) return '-'
    if (bytes < 1024) return `${bytes} B`
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(2)} KB`
    return `${(bytes / (1024 * 1024)).toFixed(2)} MB`
  }

  if (loading || !user) {
    return <div className="flex min-h-screen items-center justify-center"><p>Loading...</p></div>
  }

  return (
    <div className="min-h-screen bg-gray-50">
      <nav className="bg-white shadow-sm">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
          <div className="flex justify-between h-16">
            <div className="flex items-center">
              <Link href="/admin" className="text-2xl font-bold text-gray-900">OpenPAM</Link>
              <span className="ml-4 text-sm text-gray-500">/ Audit Logs</span>
            </div>
            <div className="flex items-center">
              <Link href="/admin" className="text-sm text-blue-600 hover:text-blue-800">Back to Admin</Link>
            </div>
          </div>
        </div>
      </nav>

      <main className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
        <div className="mb-6">
          <h1 className="text-2xl font-bold text-gray-900">Audit Logs</h1>
          <p className="text-sm text-gray-600 mt-1">View all connection audit logs</p>
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
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">User</th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Protocol</th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Started</th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Ended</th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Status</th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Bytes Sent</th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Bytes Received</th>
                  </tr>
                </thead>
                <tbody className="bg-white divide-y divide-gray-200">
                  {auditLogs.map((log) => (
                    <tr key={log.id}>
                      <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-900">{log.user_id}</td>
                      <td className="px-6 py-4 whitespace-nowrap">
                        <span className={`px-2 py-1 text-xs font-semibold rounded ${
                          log.protocol === 'ssh' ? 'bg-green-100 text-green-800' : 'bg-blue-100 text-blue-800'
                        }`}>
                          {log.protocol.toUpperCase()}
                        </span>
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                        {formatDate(log.started_at)}
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                        {log.ended_at ? formatDate(log.ended_at) : '-'}
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap">
                        <span className={`px-2 py-1 text-xs font-semibold rounded ${
                          log.status === 'completed' ? 'bg-green-100 text-green-800' :
                          log.status === 'active' ? 'bg-yellow-100 text-yellow-800' :
                          'bg-red-100 text-red-800'
                        }`}>
                          {log.status}
                        </span>
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                        {formatBytes(log.bytes_sent)}
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                        {formatBytes(log.bytes_received)}
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
