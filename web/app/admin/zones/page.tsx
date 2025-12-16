'use client'

import { useAuth } from '@/lib/auth-context'
import { api } from '@/lib/api'
import { Zone } from '@/types'
import { useRouter } from 'next/navigation'
import { useEffect, useState } from 'react'
import Link from 'next/link'

export default function ZonesPage() {
  const { user, loading } = useAuth()
  const router = useRouter()
  const [zones, setZones] = useState<Zone[]>([])
  const [loadingZones, setLoadingZones] = useState(true)
  const [showModal, setShowModal] = useState(false)
  const [formData, setFormData] = useState({ name: '', type: 'hub' as 'hub' | 'satellite', description: '' })

  useEffect(() => {
    if (!loading && (!user || user.role.toLowerCase() !== 'admin')) {
      router.push('/login')
    }
  }, [user, loading, router])

  useEffect(() => {
    if (user) {
      loadZones()
    }
  }, [user])

  const loadZones = async () => {
    try {
      setLoadingZones(true)
      const response = await api.listZones()
      setZones(response.zones || [])
    } catch (error) {
      console.error('Failed to load zones:', error)
    } finally {
      setLoadingZones(false)
    }
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    try {
      await api.createZone(formData)
      setShowModal(false)
      setFormData({ name: '', type: 'hub', description: '' })
      loadZones()
    } catch (error) {
      console.error('Failed to create zone:', error)
      alert('Failed to create zone')
    }
  }

  const handleDelete = async (id: string) => {
    if (!confirm('Are you sure you want to delete this zone?')) return

    try {
      await api.deleteZone(id)
      loadZones()
    } catch (error) {
      console.error('Failed to delete zone:', error)
      alert('Failed to delete zone')
    }
  }

  if (loading || user?.role.toLowerCase() !== 'admin') {
    return <div className="flex min-h-screen items-center justify-center"><p>Loading...</p></div>
  }

  return (
    <div className="min-h-screen bg-background">

      <main className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
        <div className="flex justify-between items-center mb-6">
          <div>
            <h1 className="text-2xl font-bold text-foreground">Zones</h1>
            <p className="text-sm text-muted-foreground mt-1">Manage hub and satellite zones</p>
          </div>
          <button
            onClick={() => setShowModal(true)}
            className="px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700"
          >
            Create Zone
          </button>
        </div>

        {loadingZones ? (
          <div className="text-center py-12"><p className="text-muted-foreground">Loading...</p></div>
        ) : zones.length === 0 ? (
          <div className="text-center py-12"><p className="text-muted-foreground">No zones found</p></div>
        ) : (
          <div className="bg-card shadow rounded-lg overflow-hidden">
            <table className="min-w-full divide-y divide-border">
              <thead className="bg-background">
                <tr>
                  <th className="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase">Name</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase">Type</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase">Description</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase">Actions</th>
                </tr>
              </thead>
              <tbody className="bg-card divide-y divide-border">
                {zones.map((zone) => (
                  <tr key={zone.id}>
                    <td className="px-6 py-4 whitespace-nowrap text-sm font-medium text-foreground">{zone.name}</td>
                    <td className="px-6 py-4 whitespace-nowrap">
                      <span className={`px-2 py-1 text-xs font-semibold rounded ${zone.type === 'hub' ? 'bg-purple-100 text-purple-800' : 'bg-green-100 text-green-800'
                        }`}>
                        {zone.type}
                      </span>
                    </td>
                    <td className="px-6 py-4 text-sm text-muted-foreground">{zone.description || '-'}</td>
                    <td className="px-6 py-4 whitespace-nowrap text-sm">
                      <button
                        onClick={() => handleDelete(zone.id)}
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
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center p-4">
          <div className="bg-card rounded-lg max-w-md w-full p-6">
            <h3 className="text-lg font-semibold mb-4">Create Zone</h3>
            <form onSubmit={handleSubmit}>
              <div className="space-y-4">
                <div>
                  <label className="block text-sm font-medium text-foreground mb-1">Name</label>
                  <input
                    type="text"
                    required
                    value={formData.name}
                    onChange={(e) => setFormData({ ...formData, name: e.target.value })}
                    className="w-full px-3 py-2 border border-input rounded-md"
                  />
                </div>
                <div>
                  <label className="block text-sm font-medium text-foreground mb-1">Type</label>
                  <select
                    value={formData.type}
                    onChange={(e) => setFormData({ ...formData, type: e.target.value as 'hub' | 'satellite' })}
                    className="w-full px-3 py-2 border border-input rounded-md"
                  >
                    <option value="hub">Hub</option>
                    <option value="satellite">Satellite</option>
                  </select>
                </div>
                <div>
                  <label className="block text-sm font-medium text-foreground mb-1">Description</label>
                  <textarea
                    value={formData.description}
                    onChange={(e) => setFormData({ ...formData, description: e.target.value })}
                    className="w-full px-3 py-2 border border-input rounded-md"
                    rows={3}
                  />
                </div>
              </div>
              <div className="flex space-x-3 mt-6">
                <button
                  type="button"
                  onClick={() => setShowModal(false)}
                  className="flex-1 px-4 py-2 border border-input rounded-md text-foreground hover:bg-background"
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
