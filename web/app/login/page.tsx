'use client'

import { useEffect } from 'react'
import { useRouter } from 'next/navigation'
import { useAuth } from '@/lib/auth-context'

export default function LoginPage() {
  const { user, loading } = useAuth()
  const router = useRouter()

  useEffect(() => {
    if (!loading && user) {
      router.push('/dashboard')
    }
  }, [user, loading, router])

  const handleDevLogin = (role: 'admin' | 'user' | 'auditor') => {
    const API_URL = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080'
    window.location.href = `${API_URL}/api/v1/auth/login?role=${role}`
  }

  if (loading) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <p>Loading...</p>
      </div>
    )
  }

  return (
    <div className="min-h-screen bg-gradient-to-br from-indigo-500 via-purple-500 to-pink-500 flex items-center justify-center p-4">
      <div className="bg-white dark:bg-gray-800 rounded-2xl shadow-2xl p-8 max-w-md w-full">
        <div className="text-center mb-8">
          <h1 className="text-4xl font-bold text-gray-900 dark:text-white mb-2">OpenPAM</h1>
          <p className="text-gray-600 dark:text-gray-400">Privileged Access Management</p>
        </div>

        <div className="mb-6">
          <h2 className="text-xl font-semibold text-gray-900 dark:text-white mb-4">Development Mode</h2>
          <p className="text-sm text-gray-600 dark:text-gray-400 mb-4">
            Select a role to login as:
          </p>
        </div>

        <div className="space-y-3">
          <button
            onClick={() => handleDevLogin('admin')}
            className="w-full px-6 py-4 bg-purple-600 hover:bg-purple-700 text-white rounded-lg transition-colors text-left"
          >
            <div className="flex items-center">
              <span className="text-2xl mr-3">👑</span>
              <div>
                <div className="font-semibold">Admin</div>
                <div className="text-sm opacity-90">admin@example.com</div>
                <div className="text-xs opacity-75 mt-1">Full system access</div>
              </div>
            </div>
          </button>

          <button
            onClick={() => handleDevLogin('user')}
            className="w-full px-6 py-4 bg-green-600 hover:bg-green-700 text-white rounded-lg transition-colors text-left"
          >
            <div className="flex items-center">
              <span className="text-2xl mr-3">👤</span>
              <div>
                <div className="font-semibold">User</div>
                <div className="text-sm opacity-90">dev@example.com</div>
                <div className="text-xs opacity-75 mt-1">Request and use sessions</div>
              </div>
            </div>
          </button>

          <button
            onClick={() => handleDevLogin('auditor')}
            className="w-full px-6 py-4 bg-blue-600 hover:bg-blue-700 text-white rounded-lg transition-colors text-left"
          >
            <div className="flex items-center">
              <span className="text-2xl mr-3">🔍</span>
              <div>
                <div className="font-semibold">Auditor</div>
                <div className="text-sm opacity-90">auditor@example.com</div>
                <div className="text-xs opacity-75 mt-1">View sessions and logs</div>
              </div>
            </div>
          </button>
        </div>

        <div className="mt-6 pt-6 border-t border-gray-200 dark:border-gray-700">
          <p className="text-xs text-gray-500 dark:text-gray-400 text-center">
            Development mode only - Production uses EntraID authentication
          </p>
        </div>
      </div>
    </div>
  )
}
