'use client'

import { useEffect, useState } from 'react'
import { useAuth } from '@/lib/auth-context'
import { useRouter } from 'next/navigation'
import { Group } from '@/types'
import { api } from '@/lib/api'
import Link from 'next/link'

export default function GroupsPage() {
    const { user, loading } = useAuth()
    const router = useRouter()
    const [groups, setGroups] = useState<Group[]>([])
    const [loadingGroups, setLoadingGroups] = useState(true)
    const [showCreateModal, setShowCreateModal] = useState(false)
    const [newGroupName, setNewGroupName] = useState('')
    const [newGroupDesc, setNewGroupDesc] = useState('')
    const [creating, setCreating] = useState(false)

    useEffect(() => {
        if (!loading && (!user || user.role.toLowerCase() !== 'admin')) {
            router.push('/sessions')
        }
    }, [user, loading, router])

    useEffect(() => {
        if (user?.role.toLowerCase() === 'admin') {
            fetchGroups()
        }
    }, [user])

    const fetchGroups = async () => {
        try {
            setLoadingGroups(true)
            const response = await api.listGroups()
            setGroups(response.groups || [])
        } catch (error) {
            console.error('Failed to fetch groups:', error)
        } finally {
            setLoadingGroups(false)
        }
    }

    const handleCreateGroup = async (e: React.FormEvent) => {
        e.preventDefault()
        if (!newGroupName.trim()) return

        try {
            setCreating(true)
            await api.createGroup({
                name: newGroupName,
                description: newGroupDesc
            })
            setShowCreateModal(false)
            setNewGroupName('')
            setNewGroupDesc('')
            fetchGroups()
        } catch (error) {
            console.error('Failed to create group:', error)
            alert('Failed to create group')
        } finally {
            setCreating(false)
        }
    }

    const handleDeleteGroup = async (id: string) => {
        if (!confirm('Are you sure you want to delete this group?')) return

        try {
            await api.deleteGroup(id)
            fetchGroups()
        } catch (error) {
            console.error('Failed to delete group:', error)
            alert('Failed to delete group')
        }
    }

    if (loading || user?.role.toLowerCase() !== 'admin') {
        return null
    }

    return (
        <div className="min-h-screen bg-background ">
            <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
                <div className="flex justify-between items-center mb-8">
                    <div>
                        <h1 className="text-3xl font-bold text-foreground ">Groups</h1>
                        <p className="mt-2 text-muted-foreground ">Manage user groups and memberships</p>
                    </div>
                    <button
                        onClick={() => setShowCreateModal(true)}
                        className="px-4 py-2 bg-indigo-600 text-white rounded-lg hover:bg-indigo-700 transition-colors"
                    >
                        Create Group
                    </button>
                </div>

                <div className="bg-card  shadow-md rounded-lg overflow-hidden">
                    <table className="min-w-full divide-y divide-border ">
                        <thead className="bg-background ">
                            <tr>
                                <th className="px-6 py-3 text-left text-xs font-medium text-muted-foreground  uppercase tracking-wider">Name</th>
                                <th className="px-6 py-3 text-left text-xs font-medium text-muted-foreground  uppercase tracking-wider">Description</th>
                                <th className="px-6 py-3 text-left text-xs font-medium text-muted-foreground  uppercase tracking-wider">Members</th>
                                <th className="px-6 py-3 text-left text-xs font-medium text-muted-foreground  uppercase tracking-wider">Created</th>
                                <th className="px-6 py-3 text-right text-xs font-medium text-muted-foreground  uppercase tracking-wider">Actions</th>
                            </tr>
                        </thead>
                        <tbody className="bg-card  divide-y divide-border ">
                            {loadingGroups ? (
                                <tr>
                                    <td colSpan={5} className="px-6 py-4 text-center text-muted-foreground ">Loading groups...</td>
                                </tr>
                            ) : groups.length === 0 ? (
                                <tr>
                                    <td colSpan={5} className="px-6 py-4 text-center text-muted-foreground ">No groups found</td>
                                </tr>
                            ) : (
                                groups.map((group) => (
                                    <tr key={group.id}>
                                        <td className="px-6 py-4 whitespace-nowrap">
                                            <div className="text-sm font-medium text-foreground ">{group.name}</div>
                                        </td>
                                        <td className="px-6 py-4 whitespace-nowrap">
                                            <div className="text-sm text-muted-foreground ">{group.description || '-'}</div>
                                        </td>
                                        <td className="px-6 py-4 whitespace-nowrap">
                                            <div className="text-sm text-muted-foreground ">{group.member_count || 0} members</div>
                                        </td>
                                        <td className="px-6 py-4 whitespace-nowrap text-sm text-muted-foreground ">
                                            {new Date(group.created_at).toLocaleDateString()}
                                        </td>
                                        <td className="px-6 py-4 whitespace-nowrap text-right text-sm font-medium space-x-3">
                                            <Link
                                                href={`/admin/groups/${group.id}`}
                                                className="text-indigo-600 hover:text-indigo-900 dark:text-indigo-400 dark:hover:text-indigo-300"
                                            >
                                                Manage Members
                                            </Link>
                                            <button
                                                onClick={() => handleDeleteGroup(group.id)}
                                                className="text-red-600 hover:text-red-900 dark:text-red-400 dark:hover:text-red-300"
                                            >
                                                Delete
                                            </button>
                                        </td>
                                    </tr>
                                ))
                            )}
                        </tbody>
                    </table>
                </div>
            </div>

            {/* Create Group Modal */}
            {showCreateModal && (
                <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
                    <div className="bg-card  rounded-lg p-6 max-w-md w-full mx-4">
                        <h2 className="text-xl font-bold text-foreground  mb-4">Create Group</h2>
                        <form onSubmit={handleCreateGroup} className="space-y-4">
                            <div>
                                <label className="block text-sm font-medium text-foreground  mb-1">Name</label>
                                <input
                                    type="text"
                                    value={newGroupName}
                                    onChange={(e) => setNewGroupName(e.target.value)}
                                    required
                                    className="w-full px-3 py-2 border border-input  rounded-lg bg-card  text-foreground "
                                    placeholder="e.g. Developers"
                                />
                            </div>
                            <div>
                                <label className="block text-sm font-medium text-foreground  mb-1">Description</label>
                                <textarea
                                    value={newGroupDesc}
                                    onChange={(e) => setNewGroupDesc(e.target.value)}
                                    className="w-full px-3 py-2 border border-input  rounded-lg bg-card  text-foreground "
                                    placeholder="Optional description"
                                    rows={3}
                                />
                            </div>
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
                                    className="px-4 py-2 bg-indigo-600 text-white rounded-lg hover:bg-indigo-700 disabled:opacity-50"
                                >
                                    {creating ? 'Creating...' : 'Create'}
                                </button>
                            </div>
                        </form>
                    </div>
                </div>
            )}
        </div>
    )
}
