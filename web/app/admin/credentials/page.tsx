'use client'

import { useAuth } from '@/lib/auth-context'
import { api } from '@/lib/api'
import { Credential, Target } from '@/types'
import { useRouter } from 'next/navigation'
import { useEffect, useState } from 'react'
import Link from 'next/link'
import Header from '@/components/header'

export default function CredentialsPage() {
  const { user, loading: authLoading } = useAuth() // Renamed to avoid conflict with local 'loading' state
  const router = useRouter()
  const [credentials, setCredentials] = useState<Credential[]>([])
  const [targets, setTargets] = useState<Target[]>([])
  const [loading, setLoading] = useState(true) // For initial target loading
  const [loadingCredentials, setLoadingCredentials] = useState(false) // For credentials table loading
  const [showModal, setShowModal] = useState(false)
  const [selectedTargetId, setSelectedTargetId] = useState('')
  const [editingId, setEditingId] = useState<string | null>(null)
  const [formData, setFormData] = useState({
    target_id: '',
    username: '',
    vault_secret_path: '',
    description: '',
  })

  useEffect(() => {
    if (!loading && (!user || user.role.toLowerCase() !== 'admin')) {
      router.push('/login')
      return
    }

    if (user) {
      loadTargets()
    }
  }, [user, authLoading, router])

  const loadTargets = async () => {
    try {
      const response = await api.listTargets()
      setTargets(response.targets || [])
      if (response.targets && response.targets.length > 0) {
        setSelectedTargetId(response.targets[0].id)
        setFormData(prev => ({ ...prev, target_id: response.targets?.[0]?.id || '' })) // Set initial target_id for form
        loadCredentials(response.targets[0].id)
      }
      setLoading(false)
    } catch (error) {
      console.error('Failed to load targets:', error)
      setLoading(false)
    }
  }

  const loadCredentials = async (targetId: string) => {
    setLoadingCredentials(true)
    try {
      const response = await api.listCredentials(targetId)
      setCredentials(response.credentials || [])
    } catch (error) {
      console.error('Failed to load credentials:', error)
    } finally {
      setLoadingCredentials(false)
    }
  }

  const handleTargetChange = (targetId: string) => {
    setSelectedTargetId(targetId)
    setFormData(prev => ({ ...prev, target_id: targetId }))
    loadCredentials(targetId)
  }

  const handleEdit = (cred: Credential) => {
    setEditingId(cred.id)
    setFormData({
      target_id: cred.target_id,
      username: cred.username,
      vault_secret_path: '', // Don't show existing secret path for security, or fetch if needed
      description: cred.description || '',
    })
    setShowModal(true)
  }

  const handleCreate = () => {
    setEditingId(null)
    setFormData({
      target_id: selectedTargetId || '',
      username: '',
      vault_secret_path: '',
      description: '',
    })
    setShowModal(true)
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    try {
      if (editingId) {
        await api.updateCredential(editingId, formData)
      } else {
        await api.createCredential(formData)
      }
      setShowModal(false)
      setEditingId(null)
      setFormData({ target_id: '', username: '', vault_secret_path: '', description: '' })
      if (selectedTargetId) {
        loadCredentials(selectedTargetId)
      }
    } catch (error) {
      console.error('Failed to save credential:', error)
      alert('Failed to save credential')
    }
  }

  const handleDelete = async (id: string) => {
    if (!confirm('Are you sure you want to delete this credential?')) return

    try {
      await api.deleteCredential(id)
      if (selectedTargetId) {
        loadCredentials(selectedTargetId)
      }
    } catch (error) {
      console.error('Failed to delete credential:', error)
      alert('Failed to delete credential')
    }
  }

  if (authLoading || !user || user.role.toLowerCase() !== 'admin') {
    return <div className="flex min-h-screen items-center justify-center"><p>Loading...</p></div>
  }

  return (
    <div className="min-h-screen bg-gray-50">
      <Header />

      <main className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
        <div className="flex justify-between items-center mb-6">
          <div>
            <h1 className="text-2xl font-bold text-gray-900">Credentials</h1>
            <p className="text-sm text-gray-600 mt-1">Manage credentials for targets</p>
          </div>
          <button
            onClick={handleCreate}
            className="px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700"
            disabled={targets.length === 0}
          >
            Create Credential
          </button>
        </div>

        {targets.length === 0 ? (
          <div className="text-center py-12">
            <p className="text-gray-500">No targets found. Create a target first.</p>
          </div>
        ) : (
          <>
            <div className="mb-6">
              <label className="block text-sm font-medium text-gray-700 mb-2">Select Target</label>
              <select
                value={selectedTargetId}
                onChange={(e) => handleTargetChange(e.target.value)}
                className="w-full max-w-md px-3 py-2 border border-gray-300 rounded-md"
              >
                {targets.map((target) => (
                  <option key={target.id} value={target.id}>
                    {target.name} ({target.hostname})
                  </option>
                ))}
              </select>
            </div>

            {loadingCredentials ? (
              <div className="text-center py-12"><p className="text-gray-500">Loading...</p></div>
            ) : credentials.length === 0 ? (
              <div className="text-center py-12"><p className="text-gray-500">No credentials found for this target</p></div>
            ) : (
              <div className="bg-white shadow rounded-lg overflow-hidden">
                <table className="min-w-full divide-y divide-gray-200">
                  <thead className="bg-gray-50">
                    <tr>
                      <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Username</th>
                      <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Description</th>
                      <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Actions</th>
                    </tr>
                  </thead>
                  <tbody className="bg-white divide-y divide-gray-200">
                    {credentials.map((cred) => (
                      <tr key={cred.id}>
                        <td className="px-6 py-4 whitespace-nowrap text-sm font-medium text-gray-900">{cred.username}</td>
                        <td className="px-6 py-4 text-sm text-gray-500">{cred.description || '-'}</td>
                        <td className="px-6 py-4 whitespace-nowrap text-sm space-x-3">
                          <button
                            onClick={() => handleEdit(cred)}
                            className="text-blue-600 hover:text-blue-900"
                          >
                            Edit
                          </button>
                          <button
                            onClick={() => handleDelete(cred.id)}
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
          </>
        )}
      </main>

      {showModal && (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center p-4">
          <div className="bg-white rounded-lg max-w-md w-full p-6">
            <h3 className="text-lg font-semibold mb-4">{editingId ? 'Edit Credential' : 'Create Credential'}</h3>
            <form onSubmit={handleSubmit}>
              <div className="space-y-4">
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">Target</label>
                  <select
                    required
                    value={formData.target_id}
                    onChange={(e) => setFormData({ ...formData, target_id: e.target.value })}
                    className="w-full px-3 py-2 border border-gray-300 rounded-md"
                    disabled={!!editingId}
                  >
                    <option value="">Select a target</option>
                    {targets.map((target) => (
                      <option key={target.id} value={target.id}>{target.name}</option>
                    ))}
                  </select>
                </div>
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">Username</label>
                  <input
                    type="text"
                    required
                    value={formData.username}
                    onChange={(e) => setFormData({ ...formData, username: e.target.value })}
                    className="w-full px-3 py-2 border border-gray-300 rounded-md"
                  />
                </div>
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">Vault Secret Path</label>
                  <input
                    type="text"
                    required={!editingId} // Only required for new credentials
                    value={formData.vault_secret_path}
                    onChange={(e) => setFormData({ ...formData, vault_secret_path: e.target.value })}
                    className="w-full px-3 py-2 border border-gray-300 rounded-md"
                    placeholder={editingId ? 'Leave empty to keep unchanged' : ''}
                  />
                  {editingId && <p className="text-xs text-gray-500 mt-1">Enter new path to update, or leave as is</p>}
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
                  {editingId ? 'Save Changes' : 'Create'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  )
}
