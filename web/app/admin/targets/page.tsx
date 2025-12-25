'use client'

import { useAuth } from '@/lib/auth-context'
import { api } from '@/lib/api'
import { Target, Zone } from '@/types'
import { useRouter } from 'next/navigation'
import { useEffect, useState } from 'react'
import Link from 'next/link'

export default function TargetsPage() {
  const { user, loading } = useAuth()
  const router = useRouter()
  const [targets, setTargets] = useState<Target[]>([])
  const [zones, setZones] = useState<Zone[]>([])
  const [loadingTargets, setLoadingTargets] = useState(true)
  const [showModal, setShowModal] = useState(false)
  const [editingTargetId, setEditingTargetId] = useState<string | null>(null)
  const [formData, setFormData] = useState({
    zone_id: '',
    name: '',
    hostname: '',
    protocol: 'ssh' as 'ssh' | 'rdp',
    port: 22,
    description: '',
    enabled: true,
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

  const openCreateModal = () => {
    setEditingTargetId(null)
    setFormData({
      zone_id: '',
      name: '',
      hostname: '',
      protocol: 'ssh',
      port: 22,
      description: '',
      enabled: true,
    })
    setShowModal(true)
  }

  const openEditModal = (target: Target) => {
    setEditingTargetId(target.id)
    setFormData({
      zone_id: target.zone_id || '',
      name: target.name,
      hostname: target.hostname,
      protocol: target.protocol as 'ssh' | 'rdp',
      port: target.port,
      description: target.description || '',
      enabled: target.enabled,
    })
    setShowModal(true)
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    try {
      if (editingTargetId) {
        await api.updateTarget(editingTargetId, formData)
      } else {
        await api.createTarget(formData)
      }
      setShowModal(false)
      loadTargets()
    } catch (error) {
      console.error('Failed to save target:', error)
      alert('Failed to save target')
    }
  }

  const handleDelete = async (id: string) => {
    if (!confirm('Are you sure you want to delete this target?')) return

    try {
      await api.deleteTarget(id)
      loadTargets()
    } catch (error) {
      console.error('Failed to delete target:', error)
      const msg = error instanceof Error ? error.message : 'Failed to delete target'
      alert(msg)
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
            <h1 className="text-2xl font-bold text-foreground">Targets</h1>
            <p className="text-sm text-muted-foreground mt-1">Manage SSH and RDP targets</p>
          </div>
          <button
            onClick={openCreateModal}
            className="px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700"
          >
            Create Target
          </button>
        </div>

        {loadingTargets ? (
          <div className="text-center py-12"><p className="text-muted-foreground">Loading...</p></div>
        ) : targets.length === 0 ? (
          <div className="text-center py-12"><p className="text-muted-foreground">No targets found</p></div>
        ) : (
          <div className="bg-card shadow rounded-lg overflow-hidden">
            <table className="min-w-full divide-y divide-border">
              <thead className="bg-background">
                <tr>
                  <th className="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase">Name</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase">Hostname</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase">Protocol</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase">Port</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase">Status</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase">Actions</th>
                </tr>
              </thead>
              <tbody className="bg-card divide-y divide-border">
                {targets.map((target) => (
                  <tr key={target.id}>
                    <td className="px-6 py-4 whitespace-nowrap text-sm font-medium text-foreground">{target.name}</td>
                    <td className="px-6 py-4 whitespace-nowrap text-sm text-muted-foreground">{target.hostname}</td>
                    <td className="px-6 py-4 whitespace-nowrap">
                      <span className={`px-2 py-1 text-xs font-semibold rounded ${target.protocol === 'ssh' ? 'bg-green-100 text-green-800' : 'bg-blue-100 text-blue-800'
                        }`}>
                        {target.protocol.toUpperCase()}
                      </span>
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-sm text-muted-foreground">{target.port}</td>
                    <td className="px-6 py-4 whitespace-nowrap">
                      <span className={`px-2 py-1 text-xs font-semibold rounded ${target.enabled ? 'bg-green-100 text-green-800' : 'bg-gray-100 text-gray-800'
                        }`}>
                        {target.enabled ? 'Enabled' : 'Disabled'}
                      </span>
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-sm space-x-3">
                      <button
                        onClick={() => openEditModal(target)}
                        className="text-blue-600 hover:text-blue-900"
                      >
                        Edit
                      </button>
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
          <div className="bg-card rounded-lg max-w-md w-full p-6 my-8">
            <h3 className="text-lg font-semibold mb-4">{editingTargetId ? 'Edit Target' : 'Create Target'}</h3>
            <form onSubmit={handleSubmit}>
              <div className="space-y-4">
                <div>
                  <label className="block text-sm font-medium text-foreground mb-1">Zone</label>
                  <select
                    required
                    value={formData.zone_id}
                    onChange={(e) => setFormData({ ...formData, zone_id: e.target.value })}
                    className="w-full px-3 py-2 border border-input rounded-md"
                  >
                    <option value="">Select a zone</option>
                    {zones.map((zone) => (
                      <option key={zone.id} value={zone.id}>{zone.name}</option>
                    ))}
                  </select>
                </div>
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
                  <label className="block text-sm font-medium text-foreground mb-1">Hostname</label>
                  <input
                    type="text"
                    required
                    value={formData.hostname}
                    onChange={(e) => setFormData({ ...formData, hostname: e.target.value })}
                    className="w-full px-3 py-2 border border-input rounded-md"
                  />
                </div>
                <div>
                  <label className="block text-sm font-medium text-foreground mb-1">Protocol</label>
                  <select
                    value={formData.protocol}
                    onChange={(e) => setFormData({
                      ...formData,
                      protocol: e.target.value as 'ssh' | 'rdp',
                      port: e.target.value === 'ssh' ? 22 : 3389
                    })}
                    className="w-full px-3 py-2 border border-input rounded-md"
                  >
                    <option value="ssh">SSH</option>
                    <option value="rdp">RDP</option>
                  </select>
                </div>
                <div>
                  <label className="block text-sm font-medium text-foreground mb-1">Port</label>
                  <input
                    type="number"
                    required
                    min="1"
                    max="65535"
                    value={formData.port}
                    onChange={(e) => setFormData({ ...formData, port: parseInt(e.target.value) })}
                    className="w-full px-3 py-2 border border-input rounded-md"
                  />
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
                <div className="flex items-center">
                  <input
                    id="enabled"
                    type="checkbox"
                    checked={formData.enabled}
                    onChange={(e) => setFormData({ ...formData, enabled: e.target.checked })}
                    className="h-4 w-4 text-blue-600 border-input rounded"
                  />
                  <label htmlFor="enabled" className="ml-2 block text-sm text-foreground">
                    Enabled
                  </label>
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
                  {editingTargetId ? 'Save Changes' : 'Create'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  )
}
