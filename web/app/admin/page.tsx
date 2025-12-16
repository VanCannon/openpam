'use client'

import { useAuth } from '@/lib/auth-context'
import { useRouter } from 'next/navigation'
import { useEffect } from 'react'

export default function AdminPage() {
  const { user, loading } = useAuth()
  const router = useRouter()

  useEffect(() => {
    if (!loading) {
      if (!user) {
        router.push('/login')
      } else {
        router.push('/admin/users')
      }
    }
  }, [user, loading, router])

  return (
    <div className="flex min-h-screen items-center justify-center text-gray-400">
      <p>Redirecting...</p>
    </div>
  )
}
