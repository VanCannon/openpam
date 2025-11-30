'use client'

import { useAuth } from '@/lib/auth-context'
import { useRouter } from 'next/navigation'
import { useEffect } from 'react'
import Link from 'next/link'
import Header from '@/components/header'

export default function AdminPage() {
  const { user, loading } = useAuth()
  const router = useRouter()

  useEffect(() => {
    if (!loading && !user) {
      router.push('/login')
    }
  }, [user, loading, router])

  if (loading || !user) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <p>Loading...</p>
      </div>
    )
  }

  const menuItems = [
    {
      title: 'Users',
      description: 'Manage users and assign roles',
      href: '/admin/users',
      icon: '👥',
    },
    {
      title: 'Schedule Requests',
      description: 'Approve or reject access requests',
      href: '/admin/requests',
      icon: '📅',
    },
    {
      title: 'Zones',
      description: 'Manage hub and satellite zones',
      href: '/admin/zones',
      icon: '🌐',
    },
    {
      title: 'Targets',
      description: 'Manage SSH and RDP targets',
      href: '/admin/targets',
      icon: '🎯',
    },
    {
      title: 'Credentials',
      description: 'Manage target credentials',
      href: '/admin/credentials',
      icon: '🔑',
    },
    {
      title: 'Audit Logs',
      description: 'View connection audit logs',
      href: '/admin/audit',
      icon: '📋',
    },
    {
      title: 'AD Sync',
      description: 'Synchronize with Active Directory',
      href: '/admin/identity',
      icon: '🔄',
    },
    {
      title: 'Managed Accounts',
      description: 'View accounts imported from AD',
      href: '/admin/managed-accounts',
      icon: '🏢',
    },
  ]

  return (
    <div className="min-h-screen bg-gray-50">
      <Header />

      <main className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
        <div className="mb-8">
          <h1 className="text-3xl font-bold text-gray-900">Administration</h1>
          <p className="text-gray-600 mt-2">Manage your PAM infrastructure</p>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
          {menuItems.map((item) => (
            <Link
              key={item.href}
              href={item.href}
              className="bg-white p-6 rounded-lg shadow hover:shadow-md transition-shadow"
            >
              <div className="flex items-start">
                <span className="text-4xl mr-4">{item.icon}</span>
                <div>
                  <h2 className="text-xl font-semibold text-gray-900 mb-2">
                    {item.title}
                  </h2>
                  <p className="text-gray-600">{item.description}</p>
                </div>
              </div>
            </Link>
          ))}
        </div>
      </main>
    </div>
  )
}
