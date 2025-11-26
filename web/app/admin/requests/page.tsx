'use client'

import { useEffect, useState } from 'react'
import { useAuth } from '@/lib/auth-context'
import { useRouter } from 'next/navigation'
import { Target } from '@/types'
import { Schedule } from '@/types/schedule'
import Header from '@/components/header'

export default function ScheduleRequestsPage() {
    const { user, loading } = useAuth()
    const router = useRouter()
    const [targets, setTargets] = useState<Target[]>([])
    const [schedules, setSchedules] = useState<Schedule[]>([])
    const [loadingSchedules, setLoadingSchedules] = useState(true)
    const [selectedSchedule, setSelectedSchedule] = useState<Schedule | null>(null)
    const [showApproveModal, setShowApproveModal] = useState(false)
    const [showRejectModal, setShowRejectModal] = useState(false)
    const [modifyStartTime, setModifyStartTime] = useState('')
    const [modifyEndTime, setModifyEndTime] = useState('')
    const [rejectionReason, setRejectionReason] = useState('')
    const [processing, setProcessing] = useState(false)
    const [filter, setFilter] = useState<'pending' | 'all'>('pending')

    useEffect(() => {
        if (!loading && (!user || user.role !== 'admin')) {
            router.push('/dashboard')
        }
    }, [user, loading, router])

    useEffect(() => {
        if (user?.role === 'admin') {
            fetchTargets()
            fetchSchedules()
        }
    }, [user, filter])

    const fetchTargets = async () => {
        try {
            const response = await fetch('/api/v1/targets', {
                credentials: 'include'
            })
            if (response.ok) {
                const data = await response.json()
                setTargets(data.targets || [])
            }
        } catch (error) {
            console.error('Failed to fetch targets:', error)
        }
    }

    const fetchSchedules = async () => {
        try {
            setLoadingSchedules(true)
            const url = filter === 'pending'
                ? '/api/v1/schedules?approval_status=pending'
                : '/api/v1/schedules'
            const response = await fetch(url, {
                credentials: 'include'
            })
            if (response.ok) {
                const data = await response.json()
                setSchedules(data.schedules || [])
            }
        } catch (error) {
            console.error('Failed to fetch schedules:', error)
        } finally {
            setLoadingSchedules(false)
        }
    }

    const handleApprove = async () => {
        if (!selectedSchedule) return

        try {
            setProcessing(true)
            const body: any = { schedule_id: selectedSchedule.id }
            if (modifyStartTime) body.start_time = new Date(modifyStartTime).toISOString()
            if (modifyEndTime) body.end_time = new Date(modifyEndTime).toISOString()

            const response = await fetch('/api/v1/schedules/approve', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                credentials: 'include',
                body: JSON.stringify(body)
            })

            if (response.ok) {
                setShowApproveModal(false)
                setModifyStartTime('')
                setModifyEndTime('')
                fetchSchedules()
            } else {
                const error = await response.json()
                alert(error.message || 'Failed to approve schedule')
            }
        } catch (error) {
            console.error('Failed to approve schedule:', error)
            alert('Failed to approve schedule')
        } finally {
            setProcessing(false)
        }
    }

    const handleReject = async () => {
        if (!selectedSchedule || !rejectionReason.trim()) return

        try {
            setProcessing(true)
            const response = await fetch('/api/v1/schedules/reject', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                credentials: 'include',
                body: JSON.stringify({
                    schedule_id: selectedSchedule.id,
                    reason: rejectionReason
                })
            })

            if (response.ok) {
                setShowRejectModal(false)
                setRejectionReason('')
                fetchSchedules()
            } else {
                const error = await response.json()
                alert(error.message || 'Failed to reject schedule')
            }
        } catch (error) {
            console.error('Failed to reject schedule:', error)
            alert('Failed to reject schedule')
        } finally {
            setProcessing(false)
        }
    }

    const formatDateTime = (dateString: string) => {
        return new Date(dateString).toLocaleString()
    }

    if (loading || user?.role !== 'admin') {
        return null
    }

    return (
        <div className="min-h-screen bg-gray-50 dark:bg-gray-900">
            <Header />
            <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
                {/* Header */}
                <div className="mb-8 flex justify-between items-center">
                    <div>
                        <h1 className="text-3xl font-bold text-gray-900 dark:text-white">Schedule Requests</h1>
                        <p className="mt-2 text-gray-600 dark:text-gray-400">Review and manage session requests</p>
                    </div>
                    <div className="flex space-x-2">
                        <button
                            onClick={() => setFilter('pending')}
                            className={`px-4 py-2 rounded-lg transition-colors ${filter === 'pending'
                                ? 'bg-indigo-600 text-white'
                                : 'bg-gray-200 dark:bg-gray-700 text-gray-700 dark:text-gray-300'
                                }`}
                        >
                            Pending ({schedules.filter(s => s.approval_status === 'pending').length})
                        </button>
                        <button
                            onClick={() => setFilter('all')}
                            className={`px-4 py-2 rounded-lg transition-colors ${filter === 'all'
                                ? 'bg-indigo-600 text-white'
                                : 'bg-gray-200 dark:bg-gray-700 text-gray-700 dark:text-gray-300'
                                }`}
                        >
                            All Schedules
                        </button>
                    </div>
                </div>

                {/* Schedules Table */}
                <div className="bg-white dark:bg-gray-800 shadow-md rounded-lg overflow-hidden">
                    <table className="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
                        <thead className="bg-gray-50 dark:bg-gray-700">
                            <tr>
                                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">
                                    Target
                                </th>
                                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">
                                    Requested Times
                                </th>
                                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">
                                    Status
                                </th>
                                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">
                                    Created
                                </th>
                                <th className="px-6 py-3 text-right text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">
                                    Actions
                                </th>
                            </tr>
                        </thead>
                        <tbody className="bg-white dark:bg-gray-800 divide-y divide-gray-200 dark:divide-gray-700">
                            {loadingSchedules ? (
                                <tr>
                                    <td colSpan={5} className="px-6 py-4 text-center text-gray-500 dark:text-gray-400">
                                        Loading schedules...
                                    </td>
                                </tr>
                            ) : schedules.length === 0 ? (
                                <tr>
                                    <td colSpan={5} className="px-6 py-4 text-center text-gray-500 dark:text-gray-400">
                                        No {filter === 'pending' ? 'pending ' : ''}schedules found
                                    </td>
                                </tr>
                            ) : (
                                schedules.map((schedule) => {
                                    const target = targets.find(t => t.id === schedule.target_id)
                                    return (
                                        <tr key={schedule.id}>
                                            <td className="px-6 py-4 whitespace-nowrap">
                                                <div className="text-sm font-medium text-gray-900 dark:text-white">
                                                    {target?.name || schedule.target_id}
                                                </div>
                                                <div className="text-sm text-gray-500 dark:text-gray-400">
                                                    {target?.protocol?.toUpperCase()} - {target?.hostname}
                                                </div>
                                            </td>
                                            <td className="px-6 py-4">
                                                <div className="text-sm text-gray-900 dark:text-white">
                                                    <div>{formatDateTime(schedule.start_time)}</div>
                                                    <div className="text-gray-500 dark:text-gray-400">to</div>
                                                    <div>{formatDateTime(schedule.end_time)}</div>
                                                </div>
                                            </td>
                                            <td className="px-6 py-4 whitespace-nowrap">
                                                {schedule.approval_status === 'pending' && (
                                                    <span className="px-2 py-1 text-xs font-semibold rounded-full bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200">
                                                        Pending Approval
                                                    </span>
                                                )}
                                                {schedule.approval_status === 'approved' && (
                                                    <span className="px-2 py-1 text-xs font-semibold rounded-full bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200">
                                                        Approved
                                                    </span>
                                                )}
                                                {schedule.approval_status === 'rejected' && (
                                                    <span className="px-2 py-1 text-xs font-semibold rounded-full bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200">
                                                        Rejected
                                                    </span>
                                                )}
                                            </td>
                                            <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500 dark:text-gray-400">
                                                {formatDateTime(schedule.created_at)}
                                            </td>
                                            <td className="px-6 py-4 whitespace-nowrap text-right text-sm font-medium space-x-2">
                                                {schedule.approval_status === 'pending' && (
                                                    <>
                                                        <button
                                                            onClick={() => {
                                                                setSelectedSchedule(schedule)
                                                                setModifyStartTime(schedule.start_time.substring(0, 16))
                                                                setModifyEndTime(schedule.end_time.substring(0, 16))
                                                                setShowApproveModal(true)
                                                            }}
                                                            className="text-green-600 hover:text-green-900 dark:text-green-400 dark:hover:text-green-300"
                                                        >
                                                            Approve
                                                        </button>
                                                        <button
                                                            onClick={() => {
                                                                setSelectedSchedule(schedule)
                                                                setShowRejectModal(true)
                                                            }}
                                                            className="text-red-600 hover:text-red-900 dark:text-red-400 dark:hover:text-red-300"
                                                        >
                                                            Reject
                                                        </button>
                                                    </>
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

            {/* Approve Modal */}
            {showApproveModal && selectedSchedule && (
                <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
                    <div className="bg-white dark:bg-gray-800 rounded-lg p-6 max-w-md w-full mx-4">
                        <h2 className="text-xl font-bold text-gray-900 dark:text-white mb-4">
                            Approve Schedule Request
                        </h2>
                        <p className="text-gray-600 dark:text-gray-400 mb-4">
                            You can approve as-is or modify the times
                        </p>
                        <div className="space-y-4">
                            <div>
                                <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                                    Start Time
                                </label>
                                <input
                                    type="datetime-local"
                                    value={modifyStartTime}
                                    onChange={(e) => setModifyStartTime(e.target.value)}
                                    className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
                                />
                            </div>
                            <div>
                                <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                                    End Time
                                </label>
                                <input
                                    type="datetime-local"
                                    value={modifyEndTime}
                                    onChange={(e) => setModifyEndTime(e.target.value)}
                                    className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
                                />
                            </div>
                        </div>
                        <div className="flex justify-end space-x-3 mt-6">
                            <button
                                onClick={() => setShowApproveModal(false)}
                                className="px-4 py-2 text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg"
                            >
                                Cancel
                            </button>
                            <button
                                onClick={handleApprove}
                                disabled={processing}
                                className="px-4 py-2 bg-green-600 text-white rounded-lg hover:bg-green-700 disabled:opacity-50"
                            >
                                {processing ? 'Approving...' : 'Approve'}
                            </button>
                        </div>
                    </div>
                </div>
            )}

            {/* Reject Modal */}
            {showRejectModal && selectedSchedule && (
                <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
                    <div className="bg-white dark:bg-gray-800 rounded-lg p-6 max-w-md w-full mx-4">
                        <h2 className="text-xl font-bold text-gray-900 dark:text-white mb-4">
                            Reject Schedule Request
                        </h2>
                        <div className="mb-4">
                            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                                Reason for Rejection
                            </label>
                            <textarea
                                value={rejectionReason}
                                onChange={(e) => setRejectionReason(e.target.value)}
                                placeholder="Please provide a reason..."
                                rows={4}
                                className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
                            />
                        </div>
                        <div className="flex justify-end space-x-3">
                            <button
                                onClick={() => setShowRejectModal(false)}
                                className="px-4 py-2 text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg"
                            >
                                Cancel
                            </button>
                            <button
                                onClick={handleReject}
                                disabled={processing || !rejectionReason.trim()}
                                className="px-4 py-2 bg-red-600 text-white rounded-lg hover:bg-red-700 disabled:opacity-50"
                            >
                                {processing ? 'Rejecting...' : 'Reject'}
                            </button>
                        </div>
                    </div>
                </div>
            )}
        </div>
    )
}
