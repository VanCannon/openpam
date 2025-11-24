'use client'

import { useAuth } from '@/lib/auth-context'
import { useRouter, useSearchParams } from 'next/navigation'
import { useEffect } from 'react'

export default function AuthCallbackPage() {
  const { setToken } = useAuth()
  const router = useRouter()
  const searchParams = useSearchParams()

  useEffect(() => {
    const token = searchParams.get('token')
    const error = searchParams.get('error')

    if (error) {
      console.error('Authentication error:', error)
      router.push('/login?error=' + encodeURIComponent(error))
      return
    }

    if (token) {
      setToken(token)
      router.push('/dashboard')
    } else {
      router.push('/login')
    }
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
