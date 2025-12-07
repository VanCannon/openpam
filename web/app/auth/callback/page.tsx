'use client'

import { useEffect, useRef, Suspense } from 'react'
import { useRouter, useSearchParams } from 'next/navigation'
import { useAuth } from '@/lib/auth-context'
import { api } from '@/lib/api'

function AuthCallbackContent() {
  const router = useRouter()
  const searchParams = useSearchParams()
  const { setToken } = useAuth()
  const hasRun = useRef(false)

  useEffect(() => {
    if (hasRun.current) return
    hasRun.current = true

    const handleCallback = async () => {
      // Try to get token from query parameter
      const token = searchParams.get('token')

      // Also check for token in cookie (backend sets openpam_token cookie)
      const cookies = document.cookie.split(';').reduce((acc, cookie) => {
        const [key, value] = cookie.trim().split('=')
        acc[key] = value
        return acc
      }, {} as Record<string, string>)

      const cookieToken = cookies['openpam_token']

      const finalToken = token || cookieToken

      if (finalToken) {
        // Store token in localStorage
        localStorage.setItem('openpam_token', finalToken)

        // Update API client token
        api.setToken(finalToken)

        // Update auth context
        await setToken(finalToken)

        // Redirect to dashboard
        router.push('/dashboard')
      } else {
        console.error('No token found in callback')
        router.push('/login')
      }
    }

    handleCallback()
  }, [searchParams, setToken, router])

  return (
    <div className="flex min-h-screen items-center justify-center">
      <div className="text-center">
        <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-indigo-600 mx-auto mb-4"></div>
        <p className="text-gray-600">Completing login...</p>
      </div>
    </div>
  )
}

export default function AuthCallbackPage() {
  return (
    <Suspense fallback={
      <div className="flex min-h-screen items-center justify-center">
        <div className="text-center">
          <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-indigo-600 mx-auto mb-4"></div>
          <p className="text-gray-600">Loading...</p>
        </div>
      </div>
    }>
      <AuthCallbackContent />
    </Suspense>
  )
}
