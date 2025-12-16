'use client'

import { useEffect, useState } from 'react'
import { useAuth } from '@/lib/auth-context'
import { useRouter, useParams } from 'next/navigation'
import { Group, User } from '@/types'
import { api } from '@/lib/api'
import Link from 'next/link'

export default function GroupMembersPage() {
    const { user, loading } = useAuth()
    const router = useRouter()
    const params = useParams()
    const groupId = params.id as string

    const [group, setGroup] = useState<Group | null>(null)
    const [members, setMembers] = useState<User[]>([])
    const [loadingData, setLoadingData] = useState(true)

    // Add Member State
    const [showAddModal, setShowAddModal] = useState(false)
    const [allUsers, setAllUsers] = useState<User[]>([])
    const [searchTerm, setSearchTerm] = useState('')
    const [adding, setAdding] = useState(false)

    useEffect(() => {
        if (!loading && (!user || user.role.toLowerCase() !== 'admin')) {
            router.push('/sessions')
        }
    }, [user, loading, router])

    useEffect(() => {
        if (user?.role.toLowerCase() === 'admin' && groupId) {
            fetchData()
        }
    }, [user, groupId])

    const fetchData = async () => {
        try {
            setLoadingData(true)
            const [groupResp, membersResp] = await Promise.all([
                api.getGroup(groupId),
                api.listGroupMembers(groupId)
            ])
            setGroup(groupResp)
            setMembers(membersResp.users || [])
        } catch (error) {
            console.error('Failed to fetch group data:', error)
            alert('Failed to load group data')
            router.push('/admin/groups')
        } finally {
            setLoadingData(false)
        }
    }

    const handleOpenAddModal = async () => {
        try {
            const response = await api.listUsers()
            // Filter out existing members
            const existingMemberIds = new Set(members.map(m => m.id))
            const availableUsers = (response.users || []).filter(u => !existingMemberIds.has(u.id))
            setAllUsers(availableUsers)
            setShowAddModal(true)
        } catch (error) {
            console.error('Failed to fetch users:', error)
        }
    }

    const handleAddMember = async (userId: string) => {
        try {
            setAdding(true)
            await api.addGroupMember(groupId, userId)
            setShowAddModal(false)
            fetchData()
        } catch (error) {
            console.error('Failed to add member:', error)
            alert('Failed to add member')
        } finally {
            setAdding(false)
        }
    }

    const handleRemoveMember = async (userId: string) => {
        if (!confirm('Are you sure you want to remove this user from the group?')) return

        try {
            await api.removeGroupMember(groupId, userId)
            fetchData()
        } catch (error) {
            console.error('Failed to remove member:', error)
            alert('Failed to remove member')
        }
    }

    const filteredUsers = allUsers.filter(u =>
        u.display_name.toLowerCase().includes(searchTerm.toLowerCase()) ||
        u.email.toLowerCase().includes(searchTerm.toLowerCase())
    )

    if (loading || user?.role.toLowerCase() !== 'admin') {
        return null
    }

    if (loadingData) {
        return (
            <div className="min-h-screen bg-background  flex items-center justify-center">
                <p>Loading...</p>
            </div>
        )
    }

    return (
        <div className="min-h-screen bg-background ">
            <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
                <div className="mb-8">
                    <Link href="/admin/groups" className="text-indigo-600 hover:text-indigo-900 dark:text-indigo-400 dark:hover:text-indigo-300 mb-4 inline-block">
                        &larr; Back to Groups
                    </Link>
                    <div className="flex justify-between items-center">
                        <div>
                            <h1 className="text-3xl font-bold text-foreground ">{group?.name}</h1>
                            <p className="mt-2 text-muted-foreground ">{group?.description || 'No description'}</p>
                        </div>
                        <button
                            onClick={handleOpenAddModal}
                            className="px-4 py-2 bg-indigo-600 text-white rounded-lg hover:bg-indigo-700 transition-colors"
                        >
                            Add Member
                        </button>
                    </div>
                </div>

                <div className="bg-card  shadow-md rounded-lg overflow-hidden">
                    <div className="px-6 py-4 border-b border-border ">
                        <h2 className="text-lg font-semibold text-foreground ">Members ({members.length})</h2>
                    </div>
                    <table className="min-w-full divide-y divide-border ">
                        <thead className="bg-background ">
                            <tr>
                                <th className="px-6 py-3 text-left text-xs font-medium text-muted-foreground  uppercase tracking-wider">Name</th>
                                <th className="px-6 py-3 text-left text-xs font-medium text-muted-foreground  uppercase tracking-wider">Email</th>
                                <th className="px-6 py-3 text-left text-xs font-medium text-muted-foreground  uppercase tracking-wider">Role</th>
                                <th className="px-6 py-3 text-right text-xs font-medium text-muted-foreground  uppercase tracking-wider">Actions</th>
                            </tr>
                        </thead>
                        <tbody className="bg-card  divide-y divide-border ">
                            {members.length === 0 ? (
                                <tr>
                                    <td colSpan={4} className="px-6 py-4 text-center text-muted-foreground ">No members in this group</td>
                                </tr>
                            ) : (
                                members.map((member) => (
                                    <tr key={member.id}>
                                        <td className="px-6 py-4 whitespace-nowrap">
                                            <div className="text-sm font-medium text-foreground ">{member.display_name}</div>
                                        </td>
                                        <td className="px-6 py-4 whitespace-nowrap">
                                            <div className="text-sm text-muted-foreground ">{member.email}</div>
                                        </td>
                                        <td className="px-6 py-4 whitespace-nowrap">
                                            <span className="px-2 py-1 text-xs font-semibold rounded-full bg-gray-100 text-gray-800   capitalize">
                                                {member.role}
                                            </span>
                                        </td>
                                        <td className="px-6 py-4 whitespace-nowrap text-right text-sm font-medium">
                                            <button
                                                onClick={() => handleRemoveMember(member.id)}
                                                className="text-red-600 hover:text-red-900 dark:text-red-400 dark:hover:text-red-300"
                                            >
                                                Remove
                                            </button>
                                        </td>
                                    </tr>
                                ))
                            )}
                        </tbody>
                    </table>
                </div>
            </div>

            {/* Add Member Modal */}
            {showAddModal && (
                <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
                    <div className="bg-card  rounded-lg p-6 max-w-md w-full mx-4 h-[500px] flex flex-col">
                        <h2 className="text-xl font-bold text-foreground  mb-4">Add Member</h2>
                        <div className="mb-4">
                            <input
                                type="text"
                                placeholder="Search users..."
                                value={searchTerm}
                                onChange={(e) => setSearchTerm(e.target.value)}
                                className="w-full px-3 py-2 border border-input  rounded-lg bg-card  text-foreground "
                            />
                        </div>
                        <div className="flex-1 overflow-y-auto border border-border  rounded-lg mb-4">
                            {filteredUsers.length === 0 ? (
                                <div className="p-4 text-center text-muted-foreground">No users found</div>
                            ) : (
                                <div className="divide-y divide-border ">
                                    {filteredUsers.map(u => (
                                        <div key={u.id} className="p-3 flex justify-between items-center hover:bg-background dark:hover:bg-gray-700">
                                            <div>
                                                <div className="font-medium text-foreground ">{u.display_name}</div>
                                                <div className="text-sm text-muted-foreground ">{u.email}</div>
                                            </div>
                                            <button
                                                onClick={() => handleAddMember(u.id)}
                                                disabled={adding}
                                                className="px-3 py-1 bg-indigo-600 text-white text-sm rounded hover:bg-indigo-700 disabled:opacity-50"
                                            >
                                                Add
                                            </button>
                                        </div>
                                    ))}
                                </div>
                            )}
                        </div>
                        <div className="flex justify-end">
                            <button
                                onClick={() => setShowAddModal(false)}
                                className="px-4 py-2 text-foreground  hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg"
                            >
                                Close
                            </button>
                        </div>
                    </div>
                </div>
            )}
        </div>
    )
}
