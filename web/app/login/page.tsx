'use client'

import { useEffect, useState } from 'react'
import { useRouter } from 'next/navigation'
import { useAuth } from '@/lib/auth-context'

export default function LoginPage() {
  const { user, loading } = useAuth()
  const router = useRouter()
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [isLoggingIn, setIsLoggingIn] = useState(false)

  useEffect(() => {
    if (!loading && user) {
      router.push('/dashboard')
    }
  }, [user, loading, router])

  const handleLogin = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setIsLoggingIn(true)

    try {
      const response = await fetch('/api/v1/auth/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ username, password }),
      })

      if (response.ok) {
        // Reload to get user from context
        window.location.href = '/dashboard'
      } else {
        const data = await response.json().catch(() => ({}))
        setError(data.message || 'Invalid credentials')
      }
    } catch (err) {
      setError('Failed to login. Please try again.')
      console.error(err)
    } finally {
      setIsLoggingIn(false)
    }
  }

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

        <form onSubmit={handleLogin} className="space-y-4 mb-8">
          {error && (
            <div className="p-3 bg-red-100 text-red-700 rounded-lg text-sm">
              {error}
            </div>
          )}
          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
              Username
            </label>
            <input
              type="text"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              className="w-full px-4 py-2 rounded-lg border border-gray-300 dark:border-gray-600 dark:bg-gray-700 dark:text-white focus:ring-2 focus:ring-purple-500 outline-none"
              placeholder="Enter your username"
              required
            />
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
              Password
            </label>
            <input
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              className="w-full px-4 py-2 rounded-lg border border-gray-300 dark:border-gray-600 dark:bg-gray-700 dark:text-white focus:ring-2 focus:ring-purple-500 outline-none"
              placeholder="Enter your password"
              required
            />
          </div>
          <button
            type="submit"
            disabled={isLoggingIn}
            className="w-full px-6 py-3 bg-indigo-600 hover:bg-indigo-700 text-white rounded-lg transition-colors font-semibold disabled:opacity-50"
          >
            {isLoggingIn ? 'Logging in...' : 'Login'}
          </button>
        </form>

        <div className="relative mb-8">
          <div className="absolute inset-0 flex items-center">
            <div className="w-full border-t border-gray-300 dark:border-gray-600"></div>
          </div>
          <div className="relative flex justify-center text-sm">
            <span className="px-2 bg-white dark:bg-gray-800 text-gray-500">Or use Dev Mode</span>
          </div>
        </div>

        <div className="mb-4">
          <h2 className="text-lg font-semibold text-gray-900 dark:text-white mb-2">Development Mode</h2>
          <p className="text-sm text-gray-600 dark:text-gray-400">
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
