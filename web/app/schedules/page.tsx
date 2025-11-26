'use client'

import { useEffect, useState } from 'react'
import { useAuth } from '@/lib/auth-context'
import { useRouter } from 'next/navigation'
import { Target } from '@/types'
import { Schedule } from '@/types/schedule'

export default function SchedulesPage() {
    const { user, loading } = useAuth()
    const router = useRouter()
    const [targets, setTargets] = useState<Target[]>([])
    const [schedules, setSchedules] = useState<Schedule[]>([])
    const [loadingTargets, setLoadingTargets] = useState(true)
    const [loadingSchedules, setLoadingSchedules] = useState(true)
    const [showRequestModal, setShowRequestModal] = useState(false)
    const [selectedTarget, setSelectedTarget] = useState<string>('')
    const [startTime, setStartTime] = useState('')
    const [endTime, setEndTime] = useState('')
    const [requesting, setRequesting] = useState(false)

    useEffect(() => {
        if (!loading && !user) {
            router.push('/login')
        }
    }, [user, loading, router])

    useEffect(() => {
        if (user) {
            fetchTargets()
            fetchSchedules()
        }
    }, [user])

    const fetchTargets = async () => {
        try {
            setLoadingTargets(true)
            const response = await fetch('/api/v1/targets', {
                credentials: 'include'
            })
            if (response.ok) {
                const data = await response.json()
                setTargets(data.targets || [])
            }
        } catch (error) {
            console.error('Failed to fetch targets:', error)
        } finally {
            setLoadingTargets(false)
        }
    }

    const fetchSchedules = async () => {
        try {
            setLoadingSchedules(true)
            const response = await fetch('/api/v1/schedules', {
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

    const handleRequestSchedule = async (e: React.FormEvent) => {
        e.preventDefault()
        if (!selectedTarget || !startTime || !endTime || !user) return

        try {
            setRequesting(true)
            const response = await fetch('/api/v1/schedules/request', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                credentials: 'include',
                body: JSON.stringify({
                    user_id: user.id,
                    target_id: selectedTarget,
                    start_time: new Date(startTime).toISOString(),
                    end_time: new Date(endTime).toISOString(),
                    timezone: Intl.DateTimeFormat().resolvedOptions().timeZone
                })
            })

            if (response.ok) {
                setShowRequestModal(false)
                setSelectedTarget('')
                setStartTime('')
                setEndTime('')
                fetchSchedules()
            } else {
                const error = await response.json()
                alert(error.message || 'Failed to request schedule')
            }
        } catch (error) {
            console.error('Failed to request schedule:', error)
            alert('Failed to request schedule')
        } finally {
            setRequesting(false)
        }
    }

    const getStatusBadge = (schedule: Schedule) => {
        if (schedule.approval_status === 'rejected') {
            return <span className="px-2 py-1 text-xs font-semibold rounded-full bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200">Rejected</span>
        }
        if (schedule.approval_status === 'pending') {
            return <span className="px-2 py-1 text-xs font-semibold rounded-full bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200">Pending Approval</span>
        }
        if (schedule.status === 'active') {
            return <span className="px-2 py-1 text-xs font-semibold rounded-full bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200">Active</span>
        }
        if (schedule.status === 'expired') {
            return <span className="px-2 py-1 text-xs font-semibold rounded-full bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-200">Expired</span>
        }
        return <span className="px-2 py-1 text-xs font-semibold rounded-full bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200">Approved</span>
    }

    if (loading || !user) {
        return null
    }

    return (
        <div className="min-h-screen bg-gray-50 dark:bg-gray-900">
            <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
                {/* Header */}
                <div className="mb-8 flex justify-between items-center">
                    <div>
                        <h1 className="text-3xl font-bold text-gray-900 dark:text-white">My Schedules</h1>
                        <p className="mt-2 text-gray-600 dark:text-gray-400">Request and manage your scheduled access</p>
                    </div>
                    <button
                        onClick={() => setShowRequestModal(true)}
                        className="px-4 py-2 bg-indigo-600 text-white rounded-lg hover:bg-indigo-700 transition-colors"
                    >
                        Request Access
                    </button>
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
                                    Start Time
                                </th>
                                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">
                                    End Time
                                </th>
                                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">
                                    Status
                                </th>
                                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">
                                    Notes
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
                                        No schedules found. Request access to get started.
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
                                            <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-900 dark:text-white">
                                                {new Date(schedule.start_time).toLocaleString()}
                                            </td>
                                            <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-900 dark:text-white">
                                                {new Date(schedule.end_time).toLocaleString()}
                                            </td>
                                            <td className="px-6 py-4 whitespace-nowrap">
                                                {getStatusBadge(schedule)}
                                            </td>
                                            <td className="px-6 py-4 text-sm text-gray-500 dark:text-gray-400">
                                                {schedule.rejection_reason || '-'}
                                            </td>
                                        </tr>
                                    )
                                })
                            )}
                        </tbody>
                    </table>
                </div>
            </div>

            {/* Request Schedule Modal */}
            {showRequestModal && (
                <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
                    <div className="bg-white dark:bg-gray-800 rounded-lg p-6 max-w-md w-full mx-4">
                        <h2 className="text-xl font-bold text-gray-900 dark:text-white mb-4">
                            Request Scheduled Access
                        </h2>
                        <form onSubmit={handleRequestSchedule} className="space-y-4">
                            <div>
                                <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                                    Target
                                </label>
                                <select
                                    value={selectedTarget}
                                    onChange={(e) => setSelectedTarget(e.target.value)}
                                    required
                                    className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
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
                                <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                                    Start Time
                                </label>
                                <input
                                    type="datetime-local"
                                    value={startTime}
                                    onChange={(e) => setStartTime(e.target.value)}
                                    required
                                    className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
                                />
                            </div>
                            <div>
                                <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                                    End Time
                                </label>
                                <input
                                    type="datetime-local"
                                    value={endTime}
                                    onChange={(e) => setEndTime(e.target.value)}
                                    required
                                    className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
                                />
                            </div>
                            <div className="flex justify-end space-x-3">
                                <button
                                    type="button"
                                    onClick={() => setShowRequestModal(false)}
                                    className="px-4 py-2 text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg"
                                >
                                    Cancel
                                </button>
                                <button
                                    type="submit"
                                    disabled={requesting}
                                    className="px-4 py-2 bg-indigo-600 text-white rounded-lg hover:bg-indigo-700 disabled:opacity-50"
                                >
                                    {requesting ? 'Requesting...' : 'Submit Request'}
                                </button>
                            </div>
                        </form>
                    </div>
                </div>
            )}
        </div>
    )
}
