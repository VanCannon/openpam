'use client'

import { useEffect } from 'react'
import { useRouter, useSearchParams } from 'next/navigation'
import { useAuth } from '@/lib/auth-context'

export default function AuthCallbackPage() {
  const router = useRouter()
  const searchParams = useSearchParams()
  const { setToken } = useAuth()

  useEffect(() => {
    const handleCallback = async () => {
      // Try to get token from query parameter
      const token = searchParams.get('token')

      // Also check for token in cookie (backend sets session_token cookie)
      const cookies = document.cookie.split(';').reduce((acc, cookie) => {
        const [key, value] = cookie.trim().split('=')
        acc[key] = value
        return acc
      }, {} as Record<string, string>)

      const cookieToken = cookies['session_token'] || cookies['openpam_token']

      const finalToken = token || cookieToken

      if (finalToken) {
        // Store token and trigger auth check
        setToken(finalToken)

        // Small delay to let auth context update
        setTimeout(() => {
          router.push('/dashboard')
        }, 100)
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
        <h2 className="text-2xl font-semibold mb-4">Authenticating...</h2>
        <p className="text-gray-600">Please wait while we sign you in.</p>
      </div>
    </div>
  )
}
