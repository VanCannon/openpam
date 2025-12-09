'use client'

import { useAuth } from '@/lib/auth-context'
import { useRouter, usePathname } from 'next/navigation'
import Link from 'next/link'

export default function Header() {
    const { user, logout } = useAuth()
    const router = useRouter()
    const pathname = usePathname()

    const handleLogout = async () => {
        await logout()
        router.push('/login')
    }

    if (!user) return null

    const isActive = (path: string) => pathname === path

    return (
        <nav className="bg-white shadow-sm">
            <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
                <div className="flex justify-between h-16">
                    <div className="flex items-center">
                        <Link href="/dashboard" className="text-2xl font-bold text-gray-900">
                            OpenPAM
                        </Link>
                    </div>
                    <div className="flex items-center space-x-4">
                        <span className="text-sm text-gray-700">{user.display_name}</span>
                        <span className="text-xs text-gray-500 bg-gray-100 px-2 py-1 rounded capitalize">{user.role}</span>

                        <Link
                            href="/dashboard"
                            className={`text-sm ${isActive('/dashboard') ? 'text-gray-900 font-medium' : 'text-blue-600 hover:text-blue-800'}`}
                        >
                            Dashboard
                        </Link>

                        <Link
                            href="/schedules"
                            className={`text-sm ${isActive('/schedules') ? 'text-gray-900 font-medium' : 'text-blue-600 hover:text-blue-800'}`}
                        >
                            My Schedules
                        </Link>

                        <Link
                            href="/my-sessions"
                            className={`text-sm ${isActive('/my-sessions') ? 'text-gray-900 font-medium' : 'text-blue-600 hover:text-blue-800'}`}
                        >
                            My Sessions
                        </Link>

                        {user.role.toLowerCase() === 'admin' && (
                            <>
                                <Link
                                    href="/admin"
                                    className={`text-sm ${isActive('/admin') ? 'text-gray-900 font-medium' : 'text-blue-600 hover:text-blue-800'}`}
                                >
                                    Admin
                                </Link>
                                <Link
                                    href="/admin/requests"
                                    className={`text-sm ${isActive('/admin/requests') ? 'text-gray-900 font-medium' : 'text-blue-600 hover:text-blue-800'}`}
                                >
                                    Requests
                                </Link>
                            </>
                        )}

                        {(user.role.toLowerCase() === 'auditor' || user.role.toLowerCase() === 'admin') && (
                            <Link
                                href="/auditor"
                                className={`text-sm ${isActive('/auditor') ? 'text-gray-900 font-medium' : 'text-blue-600 hover:text-blue-800'}`}
                            >
                                Session Audit
                            </Link>
                        )}

                        <button
                            onClick={handleLogout}
                            className="text-sm text-gray-600 hover:text-gray-900"
                        >
                            Logout
                        </button>
                    </div>
                </div>
            </div>
        </nav>
    )
}
