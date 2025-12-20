'use client'

import { useAuth } from '@/lib/auth-context'
import { api } from '@/lib/api'
import { Target, User } from '@/types'
import { Schedule } from '@/types/schedule'
import { useRouter } from 'next/navigation'
import { useEffect, useState } from 'react'

export default function AdminSessionsPage() {
    const { user, loading } = useAuth()
    const router = useRouter()
    const [schedules, setSchedules] = useState<Schedule[]>([])
    const [targets, setTargets] = useState<Target[]>([])
    const [users, setUsers] = useState<User[]>([])
    const [loadingData, setLoadingData] = useState(true)

    // Filters
    const [filterUser, setFilterUser] = useState('')
    const [filterTarget, setFilterTarget] = useState('')
    const [filterType, setFilterType] = useState('')
    const [filterStatus, setFilterStatus] = useState('')

    // Modal state
    const [isDeleteModalOpen, setIsDeleteModalOpen] = useState(false)
    const [sessionToDelete, setSessionToDelete] = useState<string | null>(null)

    useEffect(() => {
        if (!loading) {
            if (!user) {
                router.push('/login')
            } else if (user.role !== 'admin') {
                router.push('/')
            } else {
                fetchData()
            }
        }
    }, [user, loading, router])

    // Refetch when filters change
    useEffect(() => {
        if (user?.role === 'admin') {
            fetchSchedules()
        }
    }, [filterUser, filterTarget, filterType, filterStatus])

    const fetchData = async () => {
        try {
            setLoadingData(true)
            const [targetsResponse, usersResponse] = await Promise.all([
                api.listTargets(),
                api.listUsers()
            ])
            setTargets(targetsResponse.targets || [])
            setUsers(usersResponse.users || [])

            // Initial fetch of schedules
            await fetchSchedules()
        } catch (error) {
            console.error('Failed to load data:', error)
        } finally {
            setLoadingData(false)
        }
    }

    const fetchSchedules = async () => {
        try {
            const params: any = {}
            if (filterUser) params.user_id = filterUser
            if (filterTarget) params.target_id = filterTarget
            if (filterType) params.type = filterType
            if (filterStatus) params.status = filterStatus

            const response = await api.listSchedules(params)
            setSchedules(response.schedules || [])
        } catch (error) {
            console.error('Failed to fetch schedules:', error)
        }
    }

    const clearFilters = () => {
        setFilterUser('')
        setFilterTarget('')
        setFilterType('')
        setFilterStatus('')
    }

    const handleDeleteClick = (id: string) => {
        setSessionToDelete(id)
        setIsDeleteModalOpen(true)
    }

    const confirmDelete = async () => {
        if (!sessionToDelete) return

        try {
            await api.deleteSchedule(sessionToDelete)
            // Refresh list
            fetchSchedules()
            setIsDeleteModalOpen(false)
            setSessionToDelete(null)
        } catch (error) {
            console.error('Failed to delete schedule:', error)
            alert('Failed to delete schedule')
        }
    }

    const cancelDelete = () => {
        setIsDeleteModalOpen(false)
        setSessionToDelete(null)
    }

    if (loading || !user) {
        return (
            <div className="flex min-h-screen items-center justify-center">
                <p>Loading...</p>
            </div>
        )
    }

    return (
        <div className="min-h-screen bg-background ">

            <main className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
                <div className="flex justify-between items-center mb-8">
                    <h1 className="text-2xl font-bold text-foreground ">All Sessions</h1>
                </div>

                {/* Filters */}
                <div className="bg-card  p-4 rounded-lg shadow mb-8">
                    <div className="grid grid-cols-1 md:grid-cols-5 gap-4 items-end">
                        <div>
                            <label className="block text-sm font-medium text-foreground  mb-1">User</label>
                            <select
                                value={filterUser}
                                onChange={(e) => setFilterUser(e.target.value)}
                                className="w-full px-3 py-2 border border-input  rounded-lg bg-card  text-foreground "
                            >
                                <option value="">All Users</option>
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
                                value={filterTarget}
                                onChange={(e) => setFilterTarget(e.target.value)}
                                className="w-full px-3 py-2 border border-input  rounded-lg bg-card  text-foreground "
                            >
                                <option value="">All Targets</option>
                                {targets.map((t) => (
                                    <option key={t.id} value={t.id}>
                                        {t.name} ({t.hostname})
                                    </option>
                                ))}
                            </select>
                        </div>

                        <div>
                            <label className="block text-sm font-medium text-foreground  mb-1">Type</label>
                            <select
                                value={filterType}
                                onChange={(e) => setFilterType(e.target.value)}
                                className="w-full px-3 py-2 border border-input  rounded-lg bg-card  text-foreground "
                            >
                                <option value="">All Types</option>
                                <option value="scheduled">Scheduled</option>
                                <option value="standing">Standing Access</option>
                            </select>
                        </div>

                        <div>
                            <label className="block text-sm font-medium text-foreground  mb-1">Status</label>
                            <select
                                value={filterStatus}
                                onChange={(e) => setFilterStatus(e.target.value)}
                                className="w-full px-3 py-2 border border-input  rounded-lg bg-card  text-foreground "
                            >
                                <option value="">All Statuses</option>
                                <option value="active">Active</option>
                                <option value="pending">Pending</option>
                                <option value="expired">Expired</option>
                                <option value="cancelled">Cancelled</option>
                            </select>
                        </div>

                        <div>
                            <button
                                onClick={clearFilters}
                                className="w-full px-4 py-2 bg-secondary text-secondary-foreground hover:bg-secondary/80 rounded-lg transition-colors"
                            >
                                Clear Filters
                            </button>
                        </div>
                    </div>
                </div>

                {/* Results Table */}
                <div className="bg-card  shadow rounded-lg overflow-hidden">
                    <div className="overflow-x-auto">
                        <table className="min-w-full divide-y divide-border ">
                            <thead className="bg-background ">
                                <tr>
                                    <th className="px-6 py-3 text-left text-xs font-medium text-muted-foreground  uppercase tracking-wider">User</th>
                                    <th className="px-6 py-3 text-left text-xs font-medium text-muted-foreground  uppercase tracking-wider">Target</th>
                                    <th className="px-6 py-3 text-left text-xs font-medium text-muted-foreground  uppercase tracking-wider">Type</th>
                                    <th className="px-6 py-3 text-left text-xs font-medium text-muted-foreground  uppercase tracking-wider">Time / Duration</th>
                                    <th className="px-6 py-3 text-left text-xs font-medium text-muted-foreground  uppercase tracking-wider">Status</th>
                                    <th className="px-6 py-3 text-left text-xs font-medium text-muted-foreground  uppercase tracking-wider">Approval</th>
                                    <th className="px-6 py-3 text-left text-xs font-medium text-muted-foreground  uppercase tracking-wider">Actions</th>
                                </tr>
                            </thead>
                            <tbody className="bg-card  divide-y divide-border ">
                                {schedules.length === 0 ? (
                                    <tr>
                                        <td colSpan={7} className="px-6 py-4 text-center text-muted-foreground ">
                                            No sessions found matching filters.
                                        </td>
                                    </tr>
                                ) : (
                                    schedules.map((session) => {
                                        const target = targets.find(t => t.id === session.target_id)
                                        const sessionUser = users.find(u => u.id === session.user_id)
                                        const duration = (new Date(session.end_time).getTime() - new Date(session.start_time).getTime()) / (1000 * 60) // minutes

                                        return (
                                            <tr key={session.id}>
                                                <td className="px-6 py-4 whitespace-nowrap">
                                                    <div className="text-sm font-medium text-foreground ">
                                                        {sessionUser?.display_name || 'Unknown User'}
                                                    </div>
                                                    <div className="text-sm text-muted-foreground ">
                                                        {sessionUser?.email}
                                                    </div>
                                                </td>
                                                <td className="px-6 py-4 whitespace-nowrap">
                                                    <div className="text-sm text-foreground ">
                                                        {target?.name || 'Unknown Target'}
                                                    </div>
                                                    <div className="text-sm text-muted-foreground ">
                                                        {target?.hostname}
                                                    </div>
                                                </td>
                                                <td className="px-6 py-4 whitespace-nowrap text-sm text-muted-foreground ">
                                                    {session.type === 'standing' ? (
                                                        <span className="px-2 py-1 text-xs font-semibold rounded-full bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200">
                                                            Standing Access
                                                        </span>
                                                    ) : (
                                                        <span className="px-2 py-1 text-xs font-semibold rounded-full bg-purple-100 text-purple-800 dark:bg-purple-900 dark:text-purple-200">
                                                            Scheduled
                                                        </span>
                                                    )}
                                                </td>
                                                <td className="px-6 py-4 whitespace-nowrap text-sm text-muted-foreground ">
                                                    {session.type === 'standing' ? (
                                                        'Always Available'
                                                    ) : (
                                                        <div>
                                                            <div>{new Date(session.start_time).toLocaleString()}</div>
                                                            <div className="text-xs text-gray-400">{duration} mins</div>
                                                        </div>
                                                    )}
                                                </td>
                                                <td className="px-6 py-4 whitespace-nowrap">
                                                    <span className={`px-2 py-1 text-xs font-semibold rounded-full ${session.status === 'active'
                                                        ? 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200'
                                                        : session.status === 'expired'
                                                            ? 'bg-gray-100 text-gray-800  '
                                                            : 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200'
                                                        }`}>
                                                        {session.status.charAt(0).toUpperCase() + session.status.slice(1)}
                                                    </span>
                                                </td>
                                                <td className="px-6 py-4 whitespace-nowrap">
                                                    <span className={`px-2 py-1 text-xs font-semibold rounded-full ${session.approval_status === 'approved'
                                                        ? 'bg-green-100 text-green-800'
                                                        : session.approval_status === 'rejected'
                                                            ? 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200'
                                                            : 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200'
                                                        }`}>
                                                        {session.approval_status.charAt(0).toUpperCase() + session.approval_status.slice(1)}
                                                    </span>
                                                </td>
                                                <td className="px-6 py-4 whitespace-nowrap text-sm text-muted-foreground ">
                                                    <button
                                                        onClick={() => handleDeleteClick(session.id)}
                                                        className="text-red-600 hover:text-red-900 dark:text-red-400 dark:hover:text-red-300"
                                                        title="Delete Session"
                                                    >
                                                        <svg xmlns="http://www.w3.org/2000/svg" className="h-5 w-5" viewBox="0 0 20 20" fill="currentColor">
                                                            <path fillRule="evenodd" d="M9 2a1 1 0 00-.894.553L7.382 4H4a1 1 0 000 2v10a2 2 0 002 2h8a2 2 0 002-2V6a1 1 0 100-2h-3.382l-.724-1.447A1 1 0 0011 2H9zM7 8a1 1 0 012 0v6a1 1 0 11-2 0V8zm5-1a1 1 0 00-1 1v6a1 1 0 102 0V8a1 1 0 00-1-1z" clipRule="evenodd" />
                                                        </svg>
                                                    </button>
                                                </td>
                                            </tr>
                                        )
                                    })
                                )}
                            </tbody>
                        </table>
                    </div>
                </div>

                {/* Delete Confirmation Modal */}
                {isDeleteModalOpen && (
                    <div className="fixed inset-0 z-10 overflow-y-auto">
                        <div className="flex items-end justify-center min-h-screen pt-4 px-4 pb-20 text-center sm:block sm:p-0">
                            <div className="fixed inset-0 transition-opacity" aria-hidden="true">
                                <div className="absolute inset-0 bg-black opacity-75"></div>
                            </div>

                            <span className="hidden sm:inline-block sm:align-middle sm:h-screen" aria-hidden="true">&#8203;</span>

                            <div className="inline-block align-bottom bg-card  rounded-lg text-left overflow-hidden shadow-xl transform transition-all sm:my-8 sm:align-middle sm:max-w-lg sm:w-full">
                                <div className="bg-card  px-4 pt-5 pb-4 sm:p-6 sm:pb-4">
                                    <div className="sm:flex sm:items-start">
                                        <div className="mx-auto flex-shrink-0 flex items-center justify-center h-12 w-12 rounded-full bg-red-100 sm:mx-0 sm:h-10 sm:w-10">
                                            <svg className="h-6 w-6 text-red-600" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke="currentColor" aria-hidden="true">
                                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
                                            </svg>
                                        </div>
                                        <div className="mt-3 text-center sm:mt-0 sm:ml-4 sm:text-left">
                                            <h3 className="text-lg leading-6 font-medium text-foreground " id="modal-title">
                                                Delete Session
                                            </h3>
                                            <div className="mt-2">
                                                <p className="text-sm text-muted-foreground ">
                                                    Are you sure you want to delete this session? This action cannot be undone.
                                                </p>
                                            </div>
                                        </div>
                                    </div>
                                </div>
                                <div className="bg-background  px-4 py-3 sm:px-6 sm:flex sm:flex-row-reverse">
                                    <button
                                        type="button"
                                        className="w-full inline-flex justify-center rounded-md border border-transparent shadow-sm px-4 py-2 bg-red-600 text-base font-medium text-white hover:bg-red-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-red-500 sm:ml-3 sm:w-auto sm:text-sm"
                                        onClick={confirmDelete}
                                    >
                                        Delete
                                    </button>
                                    <button
                                        type="button"
                                        className="mt-3 w-full inline-flex justify-center rounded-md border border-input shadow-sm px-4 py-2 bg-card text-base font-medium text-foreground hover:bg-background focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-indigo-500 sm:mt-0 sm:ml-3 sm:w-auto sm:text-sm"
                                        onClick={cancelDelete}
                                    >
                                        Cancel
                                    </button>
                                </div>
                            </div>
                        </div>
                    </div>
                )}
            </main>
        </div>
    )
}
