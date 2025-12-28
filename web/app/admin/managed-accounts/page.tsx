'use client'

import { useState, useEffect } from 'react'
import { useRouter } from 'next/navigation'
import { useAuth } from '@/lib/auth-context'
import { Button } from '@/components/ui/button'
import { api } from '@/lib/api'

import { ManagedAccount } from '@/types'

export default function ManagedAccountsPage() {
    const { user, loading } = useAuth()
    const router = useRouter()
    const [accounts, setAccounts] = useState<ManagedAccount[]>([])
    const [loadingUsers, setLoadingUsers] = useState(true)

    useEffect(() => {
        if (!loading && (!user || user.role.toLowerCase() !== 'admin')) {
            router.push('/sessions')
        }
    }, [user, loading, router])

    const fetchAccounts = () => {
        setLoadingUsers(true)
        api.listManagedAccounts()
            .then(res => {
                setAccounts(res.accounts || [])
            })
            .catch(err => console.error('Failed to fetch users:', err))
            .finally(() => setLoadingUsers(false))
    }

    useEffect(() => {
        if (user?.role.toLowerCase() === 'admin') {
            fetchAccounts()
        }
    }, [user])

    const handleDelete = async (id: string) => {
        if (!confirm('Are you sure you want to delete this managed account?')) return
        try {
            await api.deleteManagedAccount(id)
            fetchAccounts()
        } catch (err) {
            console.error('Failed to delete account:', err)
            alert('Failed to delete account')
        }
    }

    return (
        <div className="min-h-screen bg-background">

            <main className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
                <div className="mb-8 flex justify-between items-center">
                    <div>
                        <h1 className="text-3xl font-bold text-foreground">Managed Accounts</h1>
                        <p className="text-muted-foreground mt-2">View and manage accounts imported from Active Directory</p>
                    </div>
                    <Button onClick={fetchAccounts} variant="outline">Refresh List</Button>
                </div>

                <div className="bg-card shadow rounded-lg overflow-hidden">
                    <div className="overflow-x-auto">
                        <table className="min-w-full divide-y divide-border">
                            <thead className="bg-background">
                                <tr>
                                    <th className="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase tracking-wider">Display Name</th>
                                    <th className="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase tracking-wider">Email / UPN</th>
                                    <th className="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase tracking-wider">Role</th>
                                    <th className="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase tracking-wider">Status</th>
                                    <th className="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase tracking-wider">Created At</th>
                                    <th className="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase tracking-wider">Last Login</th>
                                    <th className="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase tracking-wider">Actions</th>
                                </tr>
                            </thead>
                            <tbody className="bg-card divide-y divide-border">
                                {loadingUsers ? (
                                    <tr>
                                        <td colSpan={7} className="px-6 py-4 text-center text-sm text-muted-foreground">Loading...</td>
                                    </tr>
                                ) : accounts.length === 0 ? (
                                    <tr>
                                        <td colSpan={7} className="px-6 py-4 text-center text-sm text-muted-foreground">No managed accounts found. Import users from the AD Sync page.</td>
                                    </tr>
                                ) : (
                                    accounts.map((account) => (
                                        <tr key={account.id}>
                                            <td className="px-6 py-4 whitespace-nowrap text-sm font-medium text-foreground">{account.display_name}</td>
                                            <td className="px-6 py-4 whitespace-nowrap text-sm text-muted-foreground">{account.email || account.user_principal_name || '-'}</td>
                                            <td className="px-6 py-4 whitespace-nowrap text-sm text-muted-foreground">
                                                <span className="px-2 inline-flex text-xs leading-5 font-semibold rounded-full bg-blue-100 text-blue-800">
                                                    {account.source || 'AD'}
                                                </span>
                                            </td>
                                            <td className="px-6 py-4 whitespace-nowrap text-sm text-muted-foreground">
                                                <span className={`px-2 inline-flex text-xs leading-5 font-semibold rounded-full ${account.status === 'active' ? 'bg-green-100 text-green-800' : 'bg-red-100 text-red-800'
                                                    }`}>
                                                    {account.status}
                                                </span>
                                            </td>
                                            <td className="px-6 py-4 whitespace-nowrap text-sm text-muted-foreground">{account.created_at ? new Date(account.created_at).toLocaleString() : '-'}</td>
                                            <td className="px-6 py-4 whitespace-nowrap text-sm text-muted-foreground">
                                                -
                                            </td>
                                            <td className="px-6 py-4 whitespace-nowrap text-sm text-muted-foreground">
                                                <button
                                                    onClick={() => handleDelete(account.id)}
                                                    className="px-3 py-1 bg-red-100 text-red-800 rounded-md hover:bg-red-200 text-xs font-medium"
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
            </main>
        </div>
    )
}
