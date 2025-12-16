'use client'

import { useAuth } from '@/lib/auth-context'
import Sidebar from './sidebar'
import { usePathname } from 'next/navigation'

export default function ClientLayout({ children }: { children: React.ReactNode }) {
    const { user } = useAuth()
    const pathname = usePathname()

    // Don't show sidebar on login page even if user is somehow populated (edge case)
    const isLoginPage = pathname === '/login'
    const showSidebar = user && !isLoginPage

    return (
        <div className="min-h-screen bg-background text-foreground transition-colors duration-300">
            {showSidebar && <Sidebar />}
            <main className={`min-h-screen transition-all duration-200 ${showSidebar ? 'ml-64' : ''}`}>
                {children}
            </main>
        </div>
    )
}
