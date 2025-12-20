'use client'

import { useEffect, useState } from 'react'
import { useAuth } from '@/lib/auth-context'
import { useRouter } from 'next/navigation'
import { Target } from '@/types'
import { Schedule } from '@/types/schedule'

export default function ScheduleRequestsPage() {
    const { user, loading } = useAuth()
    const router = useRouter()
    const [targets, setTargets] = useState<Target[]>([])
    const [schedules, setSchedules] = useState<Schedule[]>([])
    const [loadingSchedules, setLoadingSchedules] = useState(true)
    const [selectedSchedule, setSelectedSchedule] = useState<Schedule | null>(null)
    const [showCreateModal, setShowCreateModal] = useState(false)
    const [users, setUsers] = useState<any[]>([])
    const [selectedUserId, setSelectedUserId] = useState('')
    const [selectedTargetId, setSelectedTargetId] = useState('')
    const [createStartTime, setCreateStartTime] = useState('')
    const [createEndTime, setCreateEndTime] = useState('')
    const [scheduleType, setScheduleType] = useState('scheduled')
    const [accountType, setAccountType] = useState('static')
    const [creating, setCreating] = useState(false)
    const [showApproveModal, setShowApproveModal] = useState(false)
    const [showRejectModal, setShowRejectModal] = useState(false)
    const [modifyStartTime, setModifyStartTime] = useState('')
    const [modifyEndTime, setModifyEndTime] = useState('')
    const [rejectionReason, setRejectionReason] = useState('')
    const [processing, setProcessing] = useState(false)
    const [filter, setFilter] = useState<'pending' | 'all'>('pending')

    useEffect(() => {
        if (!loading && (!user || user.role.toLowerCase() !== 'admin')) {
            router.push('/sessions')
        }
    }, [user, loading, router])

    useEffect(() => {
        if (user?.role.toLowerCase() === 'admin') {
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

    const handleOpenCreateModal = async () => {
        try {
            const response = await fetch('/api/v1/users', { credentials: 'include' })
            if (response.ok) {
                const data = await response.json()
                setUsers(data.users || [])
                setShowCreateModal(true)
            }
        } catch (error) {
            console.error('Failed to fetch users:', error)
            alert('Failed to load users')
        }
    }

    const handleCreateSchedule = async (e: React.FormEvent) => {
        e.preventDefault()
        if (!selectedUserId || !selectedTargetId) return
        if (scheduleType === 'scheduled' && (!createStartTime || !createEndTime)) return

        try {
            setCreating(true)
            const response = await fetch('/api/v1/schedules', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                credentials: 'include',
                body: JSON.stringify({
                    user_id: selectedUserId,
                    target_id: selectedTargetId,
                    start_time: scheduleType === 'standing' ? new Date().toISOString() : new Date(createStartTime).toISOString(),
                    end_time: scheduleType === 'standing' ? new Date(new Date().setFullYear(new Date().getFullYear() + 100)).toISOString() : new Date(createEndTime).toISOString(),
                    timezone: Intl.DateTimeFormat().resolvedOptions().timeZone,
                    type: scheduleType,
                    account_type: accountType
                })
            })

            if (response.ok) {
                setShowCreateModal(false)
                setSelectedUserId('')
                setSelectedTargetId('')
                setCreateStartTime('')
                setCreateEndTime('')
                fetchSchedules()
            } else {
                const error = await response.json()
                alert(error.message || 'Failed to create schedule')
            }
        } catch (error) {
            console.error('Failed to create schedule:', error)
            alert('Failed to create schedule')
        } finally {
            setCreating(false)
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

    const formatDateTime = (dateString: string, timezone?: string) => {
        if (!dateString) return 'N/A'
        try {
            return new Date(dateString).toLocaleString(undefined, {
                timeZone: timezone || undefined,
                year: 'numeric',
                month: 'numeric',
                day: 'numeric',
                hour: 'numeric',
                minute: 'numeric',
            }) + (timezone ? ` (${timezone})` : '')
        } catch (e) {
            return new Date(dateString).toLocaleString()
        }
    }

    if (loading || user?.role.toLowerCase() !== 'admin') {
        return null
    }

    return (
        <div className="min-h-screen bg-background ">
            <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
                {/* Header */}
                <div className="mb-8 flex justify-between items-center">
                    <div>
                        <h1 className="text-3xl font-bold text-foreground ">Schedule Requests</h1>
                        <p className="mt-2 text-muted-foreground ">Review and manage session requests</p>
                    </div>
                    <div className="flex space-x-2">
                        <button
                            onClick={handleOpenCreateModal}
                            className="px-4 py-2 bg-green-600 text-white rounded-lg hover:bg-green-700 transition-colors"
                        >
                            Create Request
                        </button>
                        <button
                            onClick={() => setFilter('pending')}
                            className={`px-4 py-2 rounded-lg transition-colors ${filter === 'pending'
                                ? 'bg-indigo-600 text-white'
                                : 'bg-secondary  text-secondary-foreground hover:bg-secondary/80'
                                }`}
                        >
                            Pending ({schedules.filter(s => s.approval_status === 'pending').length})
                        </button>
                        <button
                            onClick={() => setFilter('all')}
                            className={`px-4 py-2 rounded-lg transition-colors ${filter === 'all'
                                ? 'bg-indigo-600 text-white'
                                : 'bg-secondary  text-secondary-foreground hover:bg-secondary/80'
                                }`}
                        >
                            All Schedules
                        </button>
                    </div>
                </div>

                {/* Schedules Table */}
                <div className="bg-card  shadow-md rounded-lg overflow-hidden">
                    <table className="min-w-full divide-y divide-border ">
                        <thead className="bg-background ">
                            <tr>
                                <th className="px-6 py-3 text-left text-xs font-medium text-muted-foreground  uppercase tracking-wider">
                                    Target
                                </th>
                                <th className="px-6 py-3 text-left text-xs font-medium text-muted-foreground  uppercase tracking-wider">
                                    Type
                                </th>
                                <th className="px-6 py-3 text-left text-xs font-medium text-muted-foreground  uppercase tracking-wider">
                                    Requested Times
                                </th>
                                <th className="px-6 py-3 text-left text-xs font-medium text-muted-foreground  uppercase tracking-wider">
                                    Status
                                </th>
                                <th className="px-6 py-3 text-left text-xs font-medium text-muted-foreground  uppercase tracking-wider">
                                    Created
                                </th>
                                <th className="px-6 py-3 text-right text-xs font-medium text-muted-foreground  uppercase tracking-wider">
                                    Actions
                                </th>
                            </tr>
                        </thead>
                        <tbody className="bg-card  divide-y divide-border ">
                            {loadingSchedules ? (
                                <tr>
                                    <td colSpan={6} className="px-6 py-4 text-center text-muted-foreground ">
                                        Loading schedules...
                                    </td>
                                </tr>
                            ) : schedules.length === 0 ? (
                                <tr>
                                    <td colSpan={6} className="px-6 py-4 text-center text-muted-foreground ">
                                        No {filter === 'pending' ? 'pending ' : ''}schedules found
                                    </td>
                                </tr>
                            ) : (
                                schedules.map((schedule) => {
                                    const target = targets.find(t => t.id === schedule.target_id)
                                    return (
                                        <tr key={schedule.id}>
                                            <td className="px-6 py-4 whitespace-nowrap">
                                                <div className="text-sm font-medium text-foreground ">
                                                    {target?.name || schedule.target_id}
                                                </div>
                                                <div className="text-sm text-muted-foreground ">
                                                    {target?.protocol?.toUpperCase()} - {target?.hostname}
                                                </div>
                                            </td>
                                            <td className="px-6 py-4 whitespace-nowrap">
                                                <div className="text-sm text-foreground  capitalize">
                                                    {schedule.type}
                                                </div>
                                                <div className="text-xs text-muted-foreground  capitalize">
                                                    {schedule.account_type?.replace('_', ' ')}
                                                </div>
                                            </td>
                                            <td className="px-6 py-4">
                                                <div className="text-sm text-foreground ">
                                                    <div>{formatDateTime(schedule.start_time, schedule.timezone)}</div>
                                                    <div className="text-muted-foreground ">to</div>
                                                    <div>{formatDateTime(schedule.end_time, schedule.timezone)}</div>
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
                                            <td className="px-6 py-4 whitespace-nowrap text-sm text-muted-foreground ">
                                                {formatDateTime(schedule.created_at)}
                                            </td>
                                            <td className="px-6 py-4 whitespace-nowrap text-right text-sm font-medium space-x-2">
                                                {schedule.approval_status === 'pending' && (
                                                    <>
                                                        <button
                                                            onClick={() => {
                                                                setSelectedSchedule(schedule)
                                                                // Convert UTC to local time for the input
                                                                const toLocalISO = (dateStr: string) => {
                                                                    const date = new Date(dateStr)
                                                                    const offset = date.getTimezoneOffset() * 60000
                                                                    const localDate = new Date(date.getTime() - offset)
                                                                    return localDate.toISOString().slice(0, 16)
                                                                }
                                                                setModifyStartTime(toLocalISO(schedule.start_time))
                                                                setModifyEndTime(toLocalISO(schedule.end_time))
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

            {/* Create Request Modal */}
            {showCreateModal && (
                <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
                    <div className="bg-card  rounded-lg p-6 max-w-md w-full mx-4 overflow-y-auto max-h-[90vh]">
                        <h2 className="text-xl font-bold text-foreground  mb-4">Create Schedule Request</h2>
                        <form onSubmit={handleCreateSchedule} className="space-y-4">
                            <div>
                                <label className="block text-sm font-medium text-foreground  mb-1">User</label>
                                <select
                                    value={selectedUserId}
                                    onChange={(e) => setSelectedUserId(e.target.value)}
                                    required
                                    className="w-full px-3 py-2 border border-input  rounded-lg bg-card  text-foreground "
                                >
                                    <option value="">Select a user...</option>
                                    {users.map((u) => (
                                        <option key={u.id} value={u.id}>
                                            {u.display_name} ({u.email})
                                        </option>
                                    ))}
                                </select>
                            </div>

                            <div>
                                <label className="block text-sm font-medium text-foreground  mb-1">Target</label>
                                <select
                                    value={selectedTargetId}
                                    onChange={(e) => setSelectedTargetId(e.target.value)}
                                    required
                                    className="w-full px-3 py-2 border border-input  rounded-lg bg-card  text-foreground "
                                >
                                    <option value="">Select a target...</option>
                                    {targets.map((target) => (
                                        <option key={target.id} value={target.id}>
                                            {target.name} ({target.protocol.toUpperCase()})
                                        </option>
                                    ))}
                                </select>
                            </div>

                            <div>
                                <label className="block text-sm font-medium text-foreground  mb-1">Access Type</label>
                                <select
                                    value={scheduleType}
                                    onChange={(e) => setScheduleType(e.target.value)}
                                    className="w-full px-3 py-2 border border-input  rounded-lg bg-card  text-foreground "
                                >
                                    <option value="scheduled">Scheduled Session</option>
                                    <option value="standing">Standing Access</option>
                                </select>
                            </div>

                            <div>
                                <label className="block text-sm font-medium text-foreground  mb-1">Account Type</label>
                                <select
                                    value={accountType}
                                    onChange={(e) => setAccountType(e.target.value)}
                                    className="w-full px-3 py-2 border border-input  rounded-lg bg-card  text-foreground "
                                >
                                    <option value="static">Static Credential</option>
                                    <option value="ephemeral">Ephemeral Account</option>
                                    <option value="user_promotion">AD User Promotion</option>
                                </select>
                            </div>

                            {scheduleType === 'scheduled' && (
                                <>
                                    <div>
                                        <label className="block text-sm font-medium text-foreground  mb-1">Start Time</label>
                                        <input
                                            type="datetime-local"
                                            value={createStartTime}
                                            onChange={(e) => setCreateStartTime(e.target.value)}
                                            required={scheduleType === 'scheduled'}
                                            className="w-full px-3 py-2 border border-input  rounded-lg bg-card  text-foreground "
                                        />
                                    </div>

                                    <div>
                                        <label className="block text-sm font-medium text-foreground  mb-1">End Time</label>
                                        <input
                                            type="datetime-local"
                                            value={createEndTime}
                                            onChange={(e) => setCreateEndTime(e.target.value)}
                                            required={scheduleType === 'scheduled'}
                                            className="w-full px-3 py-2 border border-input  rounded-lg bg-card  text-foreground "
                                        />
                                    </div>
                                </>
                            )}

                            <div className="flex justify-end space-x-3 mt-6">
                                <button
                                    type="button"
                                    onClick={() => setShowCreateModal(false)}
                                    className="px-4 py-2 text-foreground  hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg"
                                >
                                    Cancel
                                </button>
                                <button
                                    type="submit"
                                    disabled={creating}
                                    className="px-4 py-2 bg-green-600 text-white rounded-lg hover:bg-green-700 disabled:opacity-50"
                                >
                                    {creating ? 'Creating...' : 'Create Request'}
                                </button>
                            </div>
                        </form>
                    </div>
                </div>
            )}

            {/* Approve Modal */}
            {showApproveModal && selectedSchedule && (
                <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
                    <div className="bg-card  rounded-lg p-6 max-w-md w-full mx-4">
                        <h2 className="text-xl font-bold text-foreground  mb-4">
                            Approve Schedule Request
                        </h2>
                        <p className="text-muted-foreground  mb-4">
                            You can approve as-is or modify the times
                        </p>
                        <div className="space-y-4">
                            <div>
                                <label className="block text-sm font-medium text-foreground  mb-1">
                                    Start Time
                                </label>
                                <input
                                    type="datetime-local"
                                    value={modifyStartTime}
                                    onChange={(e) => setModifyStartTime(e.target.value)}
                                    className="w-full px-3 py-2 border border-input  rounded-lg bg-card  text-foreground "
                                />
                            </div>
                            <div>
                                <label className="block text-sm font-medium text-foreground  mb-1">
                                    End Time
                                </label>
                                <input
                                    type="datetime-local"
                                    value={modifyEndTime}
                                    onChange={(e) => setModifyEndTime(e.target.value)}
                                    className="w-full px-3 py-2 border border-input  rounded-lg bg-card  text-foreground "
                                />
                            </div>
                        </div>
                        <div className="flex justify-end space-x-3 mt-6">
                            <button
                                onClick={() => setShowApproveModal(false)}
                                className="px-4 py-2 text-foreground  hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg"
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
                    <div className="bg-card  rounded-lg p-6 max-w-md w-full mx-4">
                        <h2 className="text-xl font-bold text-foreground  mb-4">
                            Reject Schedule Request
                        </h2>
                        <div className="mb-4">
                            <label className="block text-sm font-medium text-foreground  mb-1">
                                Reason for Rejection
                            </label>
                            <textarea
                                value={rejectionReason}
                                onChange={(e) => setRejectionReason(e.target.value)}
                                placeholder="Please provide a reason..."
                                rows={4}
                                className="w-full px-3 py-2 border border-input  rounded-lg bg-card  text-foreground "
                            />
                        </div>
                        <div className="flex justify-end space-x-3">
                            <button
                                onClick={() => setShowRejectModal(false)}
                                className="px-4 py-2 text-foreground  hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg"
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
