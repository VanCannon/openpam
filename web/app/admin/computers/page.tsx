'use client'

import { useEffect, useState } from 'react'
import { useAuth } from '@/lib/auth-context'
import { useRouter } from 'next/navigation'
import Header from '@/components/header'

interface Computer {
    id: string
    name: string
    dns_host_name: string
    operating_system: string
    operating_system_version: string
    created_at: string
}

export default function ComputersPage() {
    const { user, loading } = useAuth()
    const router = useRouter()
    const [computers, setComputers] = useState<Computer[]>([])
    const [loadingComputers, setLoadingComputers] = useState(true)

    useEffect(() => {
        if (!loading && (!user || user.role !== 'admin')) {
            router.push('/dashboard')
        }
    }, [user, loading, router])

    useEffect(() => {
        if (user?.role === 'admin') {
            fetchComputers()
        }
    }, [user])

    const fetchComputers = async () => {
        try {
            setLoadingComputers(true)
            const response = await fetch('/api/v1/computers', {
                credentials: 'include'
            })
            if (response.ok) {
                const data = await response.json()
                setComputers(data.computers || [])
            }
        } catch (error) {
            console.error('Failed to fetch computers:', error)
        } finally {
            setLoadingComputers(false)
        }
    }

    if (loading || user?.role !== 'admin') {
        return null
    }

    return (
        <div className="min-h-screen bg-gray-50 dark:bg-gray-900">
            <Header />
            <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
                {/* Header */}
                <div className="mb-8">
                    <h1 className="text-3xl font-bold text-gray-900 dark:text-white">Computers</h1>
                    <p className="mt-2 text-gray-600 dark:text-gray-400">View synced computers from Active Directory</p>
                </div>

                {/* Computers Table */}
                <div className="bg-white dark:bg-gray-800 shadow-md rounded-lg overflow-hidden">
                    <table className="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
                        <thead className="bg-gray-50 dark:bg-gray-700">
                            <tr>
                                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">
                                    Name
                                </th>
                                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">
                                    DNS Host Name
                                </th>
                                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">
                                    Operating System
                                </th>
                                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">
                                    Version
                                </th>
                                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">
                                    Synced At
                                </th>
                            </tr>
                        </thead>
                        <tbody className="bg-white dark:bg-gray-800 divide-y divide-gray-200 dark:divide-gray-700">
                            {loadingComputers ? (
                                <tr>
                                    <td colSpan={5} className="px-6 py-4 text-center text-gray-500 dark:text-gray-400">
                                        Loading computers...
                                    </td>
                                </tr>
                            ) : computers.length === 0 ? (
                                <tr>
                                    <td colSpan={5} className="px-6 py-4 text-center text-gray-500 dark:text-gray-400">
                                        No computers found
                                    </td>
                                </tr>
                            ) : (
                                computers.map((c) => (
                                    <tr key={c.id}>
                                        <td className="px-6 py-4 whitespace-nowrap">
                                            <div className="text-sm font-medium text-gray-900 dark:text-white">
                                                {c.name}
                                            </div>
                                        </td>
                                        <td className="px-6 py-4 whitespace-nowrap">
                                            <div className="text-sm text-gray-500 dark:text-gray-400">
                                                {c.dns_host_name || '-'}
                                            </div>
                                        </td>
                                        <td className="px-6 py-4 whitespace-nowrap">
                                            <div className="text-sm text-gray-900 dark:text-white">
                                                {c.operating_system || '-'}
                                            </div>
                                        </td>
                                        <td className="px-6 py-4 whitespace-nowrap">
                                            <div className="text-sm text-gray-500 dark:text-gray-400">
                                                {c.operating_system_version || '-'}
                                            </div>
                                        </td>
                                        <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500 dark:text-gray-400">
                                            {c.created_at ? new Date(c.created_at).toLocaleString() : '-'}
                                        </td>
                                    </tr>
                                ))
                            )}
                        </tbody>
                    </table>
                </div>
            </div>
        </div>
    )
}
