'use client'

import { useAuth } from '@/lib/auth-context'
import { api } from '@/lib/api'
import { Target, Credential } from '@/types'
import { useRouter } from 'next/navigation'
import { useEffect, useState } from 'react'
import dynamic from 'next/dynamic'

const Terminal = dynamic(() => import('@/components/terminal'), { ssr: false })
const RdpViewer = dynamic(() => import('@/components/rdp-viewer'), { ssr: false })

export default function DashboardPage() {
  const { user, loading, logout } = useAuth()
  const router = useRouter()
  const [targets, setTargets] = useState<Target[]>([])
  const [loadingTargets, setLoadingTargets] = useState(true)
  const [selectedTarget, setSelectedTarget] = useState<Target | null>(null)
  const [credentials, setCredentials] = useState<Credential[]>([])
  const [selectedCredential, setSelectedCredential] = useState<Credential | null>(null)
  const [showCredentialModal, setShowCredentialModal] = useState(false)
  const [activeConnection, setActiveConnection] = useState<{ target: Target; credential: Credential } | null>(null)

  useEffect(() => {
    if (!loading && !user) {
      router.push('/login')
    }
  }, [user, loading, router])

  useEffect(() => {
    if (user) {
      loadTargets()
    }
  }, [user])

  const loadTargets = async () => {
    try {
      setLoadingTargets(true)
      const response = await api.listTargets()
      setTargets(response.targets || [])
    } catch (error) {
      console.error('Failed to load targets:', error)
    } finally {
      setLoadingTargets(false)
    }
  }

  const handleTargetClick = async (target: Target) => {
    setSelectedTarget(target)
    try {
      const response = await api.listCredentials(target.id)
      setCredentials(response.credentials || [])
      setShowCredentialModal(true)
    } catch (error) {
      console.error('Failed to load credentials:', error)
    }
  }

  const handleConnect = () => {
    if (selectedTarget && selectedCredential) {
      setActiveConnection({ target: selectedTarget, credential: selectedCredential })
      setShowCredentialModal(false)
    }
  }

  const handleDisconnect = () => {
    setActiveConnection(null)
    setSelectedTarget(null)
    setSelectedCredential(null)
  }

  const handleLogout = async () => {
    await logout()
    router.push('/login')
  }

  if (loading || !user) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <p>Loading...</p>
      </div>
    )
  }

  if (activeConnection) {
    const wsUrl = api.getWebSocketUrl(
      activeConnection.target.protocol,
      activeConnection.target.id,
      activeConnection.credential.id
    )

    return (
      <div className="h-screen">
        {activeConnection.target.protocol === 'ssh' ? (
          <Terminal wsUrl={wsUrl} onClose={handleDisconnect} />
        ) : (
          <RdpViewer wsUrl={wsUrl} onClose={handleDisconnect} />
        )}
      </div>
    )
  }

  return (
    <div className="min-h-screen bg-gray-50">
      <nav className="bg-white shadow-sm">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
          <div className="flex justify-between h-16">
            <div className="flex items-center">
              <h1 className="text-2xl font-bold text-gray-900">OpenPAM</h1>
            </div>
            <div className="flex items-center space-x-4">
              <span className="text-sm text-gray-700">{user.display_name}</span>
              <a
                href="/admin"
                className="text-sm text-blue-600 hover:text-blue-800"
              >
                Admin
              </a>
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

      <main className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
        <div className="mb-6">
          <h2 className="text-xl font-semibold text-gray-900">Available Targets</h2>
          <p className="text-sm text-gray-600 mt-1">Select a target to connect</p>
        </div>

        {loadingTargets ? (
          <div className="text-center py-12">
            <p className="text-gray-500">Loading targets...</p>
          </div>
        ) : targets.length === 0 ? (
          <div className="text-center py-12">
            <p className="text-gray-500">No targets available</p>
          </div>
        ) : (
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
            {targets.map((target) => (
              <div
                key={target.id}
                onClick={() => handleTargetClick(target)}
                className="bg-white p-6 rounded-lg shadow hover:shadow-md cursor-pointer transition-shadow"
              >
                <div className="flex items-center justify-between mb-2">
                  <h3 className="text-lg font-semibold text-gray-900">{target.name}</h3>
                  <span className={`px-2 py-1 text-xs font-semibold rounded ${target.protocol === 'ssh'
                      ? 'bg-green-100 text-green-800'
                      : 'bg-blue-100 text-blue-800'
                    }`}>
                    {target.protocol.toUpperCase()}
                  </span>
                </div>
                <p className="text-sm text-gray-600 mb-2">{target.hostname}:{target.port}</p>
                {target.description && (
                  <p className="text-sm text-gray-500">{target.description}</p>
                )}
              </div>
            ))}
          </div>
        )}
      </main>

      {showCredentialModal && selectedTarget && (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center p-4">
          <div className="bg-white rounded-lg max-w-md w-full p-6">
            <h3 className="text-lg font-semibold mb-4">Select Credential</h3>
            <p className="text-sm text-gray-600 mb-4">
              Connect to {selectedTarget.name} with:
            </p>
            {credentials.length === 0 ? (
              <p className="text-sm text-gray-500 py-4">No credentials available</p>
            ) : (
              <div className="space-y-2 mb-6">
                {credentials.map((cred) => (
                  <label
                    key={cred.id}
                    className="flex items-center p-3 border rounded cursor-pointer hover:bg-gray-50"
                  >
                    <input
                      type="radio"
                      name="credential"
                      value={cred.id}
                      checked={selectedCredential?.id === cred.id}
                      onChange={() => setSelectedCredential(cred)}
                      className="mr-3"
                    />
                    <div>
                      <p className="font-medium">{cred.username}</p>
                      {cred.description && (
                        <p className="text-sm text-gray-500">{cred.description}</p>
                      )}
                    </div>
                  </label>
                ))}
              </div>
            )}
            <div className="flex space-x-3">
              <button
                onClick={() => setShowCredentialModal(false)}
                className="flex-1 px-4 py-2 border border-gray-300 rounded-md text-gray-700 hover:bg-gray-50"
              >
                Cancel
              </button>
              <button
                onClick={handleConnect}
                disabled={!selectedCredential}
                className="flex-1 px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 disabled:bg-gray-300 disabled:cursor-not-allowed"
              >
                Connect
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
