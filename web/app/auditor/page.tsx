'use client'

import { useEffect, useState } from 'react'
import { useAuth } from '@/lib/auth-context'
import { useRouter } from 'next/navigation'
import { AuditLog } from '@/types'

export default function AuditorPage() {
    const { user, loading } = useAuth()
    const router = useRouter()
    const [sessions, setSessions] = useState<AuditLog[]>([])
    const [loadingSessions, setLoadingSessions] = useState(true)
    const [filter, setFilter] = useState<'all' | 'active' | 'completed'>('all')

    useEffect(() => {
        if (!loading && (!user || (user.role !== 'auditor' && user.role !== 'admin'))) {
            router.push('/dashboard')
        }
    }, [user, loading, router])

    useEffect(() => {
        if (user && (user.role === 'auditor' || user.role === 'admin')) {
            fetchSessions()
        }
    }, [user, filter])

    const fetchSessions = async () => {
        try {
            setLoadingSessions(true)
            const endpoint = filter === 'active'
                ? '/api/v1/audit-logs/active'
                : '/api/v1/audit-logs'
            const response = await fetch(endpoint, {
                credentials: 'include'
            })
            if (response.ok) {
                const data = await response.json()
                let fetchedSessions = data.logs || data.sessions || []

                if (filter === 'completed') {
                    fetchedSessions = fetchedSessions.filter((s: AuditLog) => s.session_status === 'completed')
                }

                setSessions(fetchedSessions)
            }
        } catch (error) {
            console.error('Failed to fetch sessions:', error)
        } finally {
            setLoadingSessions(false)
        }
    }

    const handleViewRecording = (recordingPath: string) => {
        // Navigate to recording playback
        window.open(`/admin/audit/${recordingPath}/play`, '_blank')
    }

    const getStatusBadge = (status: string) => {
        const statusMap: Record<string, { bg: string, text: string, label: string }> = {
            active: { bg: 'bg-green-100 dark:bg-green-900', text: 'text-green-800 dark:text-green-200', label: 'Active' },
            completed: { bg: 'bg-blue-100 dark:bg-blue-900', text: 'text-blue-800 dark:text-blue-200', label: 'Completed' },
            failed: { bg: 'bg-red-100 dark:bg-red-900', text: 'text-red-800 dark:text-red-200', label: 'Failed' },
            terminated: { bg: 'bg-yellow-100 dark:bg-yellow-900', text: 'text-yellow-800 dark:text-yellow-200', label: 'Terminated' }
        }
        const config = statusMap[status] || statusMap.completed
        return (
            <span className={`px-2 py-1 text-xs font-semibold rounded-full ${config.bg} ${config.text}`}>
                {config.label}
            </span>
        )
    }

    if (loading || (!user || (user.role !== 'auditor' && user.role !== 'admin'))) {
        return null
    }

    return (
        <div className="min-h-screen bg-gray-50 dark:bg-gray-900">
            <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
                {/* Header */}
                <div className="mb-8 flex justify-between items-center">
                    <div>
                        <h1 className="text-3xl font-bold text-gray-900 dark:text-white">Session Audit</h1>
                        <p className="mt-2 text-gray-600 dark:text-gray-400">Monitor and review all sessions</p>
                    </div>
                    <div className="flex space-x-2">
                        <button
                            onClick={() => setFilter('all')}
                            className={`px-4 py-2 rounded-lg transition-colors ${filter === 'all'
                                    ? 'bg-indigo-600 text-white'
                                    : 'bg-gray-200 dark:bg-gray-700 text-gray-700 dark:text-gray-300'
                                }`}
                        >
                            All Sessions
                        </button>
                        <button
                            onClick={() => setFilter('active')}
                            className={`px-4 py-2 rounded-lg transition-colors ${filter === 'active'
                                    ? 'bg-indigo-600 text-white'
                                    : 'bg-gray-200 dark:bg-gray-700 text-gray-700 dark:text-gray-300'
                                }`}
                        >
                            Active ({sessions.filter(s => s.session_status === 'active').length})
                        </button>
                        <button
                            onClick={() => setFilter('completed')}
                            className={`px-4 py-2 rounded-lg transition-colors ${filter === 'completed'
                                    ? 'bg-indigo-600 text-white'
                                    : 'bg-gray-200 dark:bg-gray-700 text-gray-700 dark:text-gray-300'
                                }`}
                        >
                            Completed
                        </button>
                    </div>
                </div>

                {/* Sessions Table */}
                <div className="bg-white dark:bg-gray-800 shadow-md rounded-lg overflow-hidden">
                    <table className="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
                        <thead className="bg-gray-50 dark:bg-gray-700">
                            <tr>
                                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">
                                    Session
                                </th>
                                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">
                                    Protocol
                                </th>
                                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">
                                    Start Time
                                </th>
                                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">
                                    Duration
                                </th>
                                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">
                                    Status
                                </th>
                                <th className="px-6 py-3 text-right text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">
                                    Actions
                                </th>
                            </tr>
                        </thead>
                        <tbody className="bg-white dark:bg-gray-800 divide-y divide-gray-200 dark:divide-gray-700">
                            {loadingSessions ? (
                                <tr>
                                    <td colSpan={6} className="px-6 py-4 text-center text-gray-500 dark:text-gray-400">
                                        Loading sessions...
                                    </td>
                                </tr>
                            ) : sessions.length === 0 ? (
                                <tr>
                                    <td colSpan={6} className="px-6 py-4 text-center text-gray-500 dark:text-gray-400">
                                        No sessions found
                                    </td>
                                </tr>
                            ) : (
                                sessions.map((session) => {
                                    const duration = session.end_time?.Valid && session.end_time.Time
                                        ? Math.round((new Date(session.end_time.Time).getTime() - new Date(session.start_time).getTime()) / 1000 / 60)
                                        : null

                                    return (
                                        <tr key={session.id}>
                                            <td className="px-6 py-4 whitespace-nowrap">
                                                <div className="text-sm font-medium text-gray-900 dark:text-white">
                                                    {session.user_id}
                                                </div>
                                                <div className="text-sm text-gray-500 dark:text-gray-400">
                                                    Target: {session.target_id}
                                                </div>
                                            </td>
                                            <td className="px-6 py-4 whitespace-nowrap">
                                                <span className="px-2 py-1 text-xs font-semibold rounded bg-gray-100 dark:bg-gray-700 text-gray-800 dark:text-gray-200">
                                                    {session.protocol?.toUpperCase() || 'SSH'}
                                                </span>
                                            </td>
                                            <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500 dark:text-gray-400">
                                                {new Date(session.start_time).toLocaleString()}
                                            </td>
                                            <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500 dark:text-gray-400">
                                                {duration !== null ? `${duration} min` : 'In progress'}
                                            </td>
                                            <td className="px-6 py-4 whitespace-nowrap">
                                                {getStatusBadge(session.session_status)}
                                            </td>
                                            <td className="px-6 py-4 whitespace-nowrap text-right text-sm font-medium">
                                                {session.recording_path && (
                                                    <button
                                                        onClick={() => handleViewRecording(session.id)}
                                                        className="text-indigo-600 hover:text-indigo-900 dark:text-indigo-400 dark:hover:text-indigo-300"
                                                    >
                                                        View Recording
                                                    </button>
                                                )}
                                            </td>
                                        </tr>
                                    )
                                })
                            )}
                        </tbody>
                    </table>
                </div>
            </div>
        </div>
    )
}
