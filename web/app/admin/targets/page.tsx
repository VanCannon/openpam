'use client'

import { useAuth } from '@/lib/auth-context'
import { api } from '@/lib/api'
import { Target, Zone } from '@/types'
import { useRouter } from 'next/navigation'
import { useEffect, useState } from 'react'
import Link from 'next/link'
import Header from '@/components/header'

export default function TargetsPage() {
  const { user, loading } = useAuth()
  const router = useRouter()
  const [targets, setTargets] = useState<Target[]>([])
  const [zones, setZones] = useState<Zone[]>([])
  const [loadingTargets, setLoadingTargets] = useState(true)
  const [showModal, setShowModal] = useState(false)
  const [formData, setFormData] = useState({
    zone_id: '',
    name: '',
    hostname: '',
    protocol: 'ssh' as 'ssh' | 'rdp',
    port: 22,
    description: '',
  })

  useEffect(() => {
    if (!loading && (!user || user.role.toLowerCase() !== 'admin')) {
      router.push('/login')
    }
  }, [user, loading, router])

  useEffect(() => {
    if (user) {
      loadTargets()
      loadZones()
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

  const loadZones = async () => {
    try {
      const response = await api.listZones()
      setZones(response.zones || [])
    } catch (error) {
      console.error('Failed to load zones:', error)
    }
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    try {
      await api.createTarget(formData)
      setShowModal(false)
      setFormData({ zone_id: '', name: '', hostname: '', protocol: 'ssh', port: 22, description: '' })
      loadTargets()
    } catch (error) {
      console.error('Failed to create target:', error)
      alert('Failed to create target')
    }
  }

  const handleDelete = async (id: string) => {
    if (!confirm('Are you sure you want to delete this target?')) return

    try {
      await api.deleteTarget(id)
      loadTargets()
    } catch (error) {
      console.error('Failed to delete target:', error)
      alert('Failed to delete target')
    }
  }

  if (loading || user?.role.toLowerCase() !== 'admin') {
    return <div className="flex min-h-screen items-center justify-center"><p>Loading...</p></div>
  }

  return (
    <div className="min-h-screen bg-gray-50">
      <Header />

      <main className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
        <div className="flex justify-between items-center mb-6">
          <div>
            <h1 className="text-2xl font-bold text-gray-900">Targets</h1>
            <p className="text-sm text-gray-600 mt-1">Manage SSH and RDP targets</p>
          </div>
          <button
            onClick={() => setShowModal(true)}
            className="px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700"
          >
            Create Target
          </button>
        </div>

        {loadingTargets ? (
          <div className="text-center py-12"><p className="text-gray-500">Loading...</p></div>
        ) : targets.length === 0 ? (
          <div className="text-center py-12"><p className="text-gray-500">No targets found</p></div>
        ) : (
          <div className="bg-white shadow rounded-lg overflow-hidden">
            <table className="min-w-full divide-y divide-gray-200">
              <thead className="bg-gray-50">
                <tr>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Name</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Hostname</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Protocol</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Port</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Status</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Actions</th>
                </tr>
              </thead>
              <tbody className="bg-white divide-y divide-gray-200">
                {targets.map((target) => (
                  <tr key={target.id}>
                    <td className="px-6 py-4 whitespace-nowrap text-sm font-medium text-gray-900">{target.name}</td>
                    <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">{target.hostname}</td>
                    <td className="px-6 py-4 whitespace-nowrap">
                      <span className={`px-2 py-1 text-xs font-semibold rounded ${target.protocol === 'ssh' ? 'bg-green-100 text-green-800' : 'bg-blue-100 text-blue-800'
                        }`}>
                        {target.protocol.toUpperCase()}
                      </span>
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">{target.port}</td>
                    <td className="px-6 py-4 whitespace-nowrap">
                      <span className={`px-2 py-1 text-xs font-semibold rounded ${target.enabled ? 'bg-green-100 text-green-800' : 'bg-gray-100 text-gray-800'
                        }`}>
                        {target.enabled ? 'Enabled' : 'Disabled'}
                      </span>
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-sm">
                      <button
                        onClick={() => handleDelete(target.id)}
                        className="text-red-600 hover:text-red-900"
                      >
                        Delete
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </main>

      {showModal && (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center p-4 overflow-y-auto">
          <div className="bg-white rounded-lg max-w-md w-full p-6 my-8">
            <h3 className="text-lg font-semibold mb-4">Create Target</h3>
            <form onSubmit={handleSubmit}>
              <div className="space-y-4">
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">Zone</label>
                  <select
                    required
                    value={formData.zone_id}
                    onChange={(e) => setFormData({ ...formData, zone_id: e.target.value })}
                    className="w-full px-3 py-2 border border-gray-300 rounded-md"
                  >
                    <option value="">Select a zone</option>
                    {zones.map((zone) => (
                      <option key={zone.id} value={zone.id}>{zone.name}</option>
                    ))}
                  </select>
                </div>
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">Name</label>
                  <input
                    type="text"
                    required
                    value={formData.name}
                    onChange={(e) => setFormData({ ...formData, name: e.target.value })}
                    className="w-full px-3 py-2 border border-gray-300 rounded-md"
                  />
                </div>
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">Hostname</label>
                  <input
                    type="text"
                    required
                    value={formData.hostname}
                    onChange={(e) => setFormData({ ...formData, hostname: e.target.value })}
                    className="w-full px-3 py-2 border border-gray-300 rounded-md"
                  />
                </div>
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">Protocol</label>
                  <select
                    value={formData.protocol}
                    onChange={(e) => setFormData({
                      ...formData,
                      protocol: e.target.value as 'ssh' | 'rdp',
                      port: e.target.value === 'ssh' ? 22 : 3389
                    })}
                    className="w-full px-3 py-2 border border-gray-300 rounded-md"
                  >
                    <option value="ssh">SSH</option>
                    <option value="rdp">RDP</option>
                  </select>
                </div>
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">Port</label>
                  <input
                    type="number"
                    required
                    min="1"
                    max="65535"
                    value={formData.port}
                    onChange={(e) => setFormData({ ...formData, port: parseInt(e.target.value) })}
                    className="w-full px-3 py-2 border border-gray-300 rounded-md"
                  />
                </div>
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">Description</label>
                  <textarea
                    value={formData.description}
                    onChange={(e) => setFormData({ ...formData, description: e.target.value })}
                    className="w-full px-3 py-2 border border-gray-300 rounded-md"
                    rows={3}
                  />
                </div>
              </div>
              <div className="flex space-x-3 mt-6">
                <button
                  type="button"
                  onClick={() => setShowModal(false)}
                  className="flex-1 px-4 py-2 border border-gray-300 rounded-md text-gray-700 hover:bg-gray-50"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  className="flex-1 px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700"
                >
                  Create
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  )
}
