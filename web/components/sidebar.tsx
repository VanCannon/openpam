'use client'

import { useAuth } from '@/lib/auth-context'
import { useRouter, usePathname } from 'next/navigation'
import Link from 'next/link'
import Image from 'next/image'
import { useState, useEffect } from 'react'
import { useTheme } from 'next-themes'
import {
    LayoutDashboard,
    History,
    Shield,
    Users,
    Settings,
    LogOut,
    ChevronDown,
    ChevronRight,
    Server,
    Key,
    FileText,
    RefreshCw,
    Globe,
    Monitor,
    Sun,
    Moon
} from 'lucide-react'

export default function Sidebar() {
    const { user, logout } = useAuth()
    const router = useRouter()
    const pathname = usePathname()
    const [isAdminOpen, setIsAdminOpen] = useState(true)
    const { theme, setTheme } = useTheme()
    const [mounted, setMounted] = useState(false)

    useEffect(() => {
        setMounted(true)
    }, [])

    const handleLogout = async () => {
        await logout()
        router.push('/login')
    }

    if (!user) return null

    const isActive = (path: string) => pathname === path || pathname.startsWith(path + '/')

    const adminLinks = [
        { href: '/admin/sessions', label: 'All Sessions', icon: LayoutDashboard },
        { href: '/admin/users', label: 'Users', icon: Users },
        { href: '/admin/groups', label: 'Groups', icon: Users },
        { href: '/admin/requests', label: 'Requests', icon: FileText },
        { href: '/admin/zones', label: 'Zones', icon: Globe },
        { href: '/admin/targets', label: 'Targets', icon: Server },
        { href: '/admin/credentials', label: 'Credentials', icon: Key },
        { href: '/admin/audit', label: 'Audit Logs', icon: Shield },
        { href: '/admin/identity', label: 'AD Sync', icon: RefreshCw },
        { href: '/admin/managed-accounts', label: 'Managed Accounts', icon: Monitor },
    ]

    return (
        <div className="h-screen w-64 bg-background text-foreground flex flex-col border-r border-border fixed left-0 top-0 overflow-y-auto z-50 transition-colors duration-300">
            {/* Logo */}
            <div className="p-6 flex flex-col items-center justify-center space-y-2">
                <div className="relative w-24 h-24">
                    <Image
                        src="/logo_icon_v2.png"
                        alt="OpenPAM Logo"
                        fill
                        className="object-contain"
                        priority
                    />
                </div>
                <span className="text-2xl font-bold tracking-widest text-foreground">OpenPAM</span>
            </div>

            {/* Navigation */}
            <nav className="flex-1 px-4 space-y-2 mt-4">
                <Link
                    href="/sessions"
                    className={`flex items-center space-x-3 px-4 py-3 rounded-lg transition-colors ${isActive('/sessions')
                        ? 'bg-gradient-primary text-white shadow-lg shadow-indigo-500/20'
                        : 'text-muted-foreground hover:bg-secondary hover:text-foreground'
                        }`}
                >
                    <LayoutDashboard size={20} />
                    <span className="font-medium">Sessions</span>
                </Link>

                <Link
                    href="/my-sessions"
                    className={`flex items-center space-x-3 px-4 py-3 rounded-lg transition-colors ${isActive('/my-sessions')
                        ? 'bg-gradient-primary text-white shadow-lg shadow-indigo-500/20'
                        : 'text-muted-foreground hover:bg-secondary hover:text-foreground'
                        }`}
                >
                    <History size={20} />
                    <span className="font-medium">History</span>
                </Link>

                {(user.role.toLowerCase() === 'auditor' || user.role.toLowerCase() === 'admin') && (
                    <Link
                        href="/auditor"
                        className={`flex items-center space-x-3 px-4 py-3 rounded-lg transition-colors ${isActive('/auditor')
                            ? 'bg-gradient-primary text-white shadow-lg shadow-indigo-500/20'
                            : 'text-muted-foreground hover:bg-secondary hover:text-foreground'
                            }`}
                    >
                        <Shield size={20} />
                        <span className="font-medium">Session Audit</span>
                    </Link>
                )}

                {user.role.toLowerCase() === 'admin' && (
                    <div className="pt-4">
                        <button
                            onClick={() => setIsAdminOpen(!isAdminOpen)}
                            className="w-full flex items-center justify-between px-4 py-2 text-muted-foreground hover:text-foreground transition-colors"
                        >
                            <div className="flex items-center space-x-3">
                                <Settings size={20} />
                                <span className="font-medium">Admin</span>
                            </div>
                            {isAdminOpen ? <ChevronDown size={16} /> : <ChevronRight size={16} />}
                        </button>

                        {isAdminOpen && (
                            <div className="mt-2 space-y-1 pl-4">
                                {adminLinks.map((link) => (
                                    <Link
                                        key={link.href}
                                        href={link.href}
                                        className={`flex items-center space-x-3 px-4 py-2 rounded-lg text-sm transition-colors ${isActive(link.href)
                                            ? 'bg-gradient-primary text-white shadow-lg shadow-indigo-500/20'
                                            : 'text-muted-foreground hover:text-foreground hover:bg-secondary/50'
                                            }`}
                                    >
                                        <link.icon size={16} />
                                        <span>{link.label}</span>
                                    </Link>
                                ))}
                            </div>
                        )}
                    </div>
                )}
            </nav>

            {/* User Profile & Logout */}
            <div className="p-4 border-t border-border">
                <div className="flex items-center justify-between mb-4 px-2">
                    <div className="flex flex-col">
                        <span className="text-sm font-medium text-foreground">{user.display_name}</span>
                        <span className="text-xs text-muted-foreground capitalize">{user.role}</span>
                    </div>
                    {mounted && (
                        <button
                            onClick={() => setTheme(theme === 'dark' ? 'light' : 'dark')}
                            className="p-2 rounded-full hover:bg-secondary text-muted-foreground hover:text-foreground transition-colors"
                            aria-label="Toggle theme"
                        >
                            {theme === 'dark' ? <Sun size={18} /> : <Moon size={18} />}
                        </button>
                    )}
                </div>
                <button
                    onClick={handleLogout}
                    className="w-full flex items-center justify-center space-x-2 px-4 py-2 rounded-lg border border-border text-muted-foreground hover:text-foreground hover:bg-secondary transition-colors"
                >
                    <LogOut size={16} />
                    <span>Logout</span>
                </button>
            </div>
        </div>
    )
}
