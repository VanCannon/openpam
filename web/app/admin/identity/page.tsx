'use client'

import { useState, useEffect } from 'react'
import { useRouter } from 'next/navigation'
import { useAuth } from '@/lib/auth-context'
import { Button } from '@/components/ui/button'

interface ADUser {
    id: string
    dn: string
    sam_account_name: string
    user_principal_name: string
    display_name: string
    mail: string
    ou: string
    status: string
    password_status: string
    last_sync: string
}

interface ADComputer {
    id: string
    dn: string
    name: string
    dns_host_name: string
    operating_system: string
    operating_system_version: string
    last_sync: string
}

interface ADGroup {
    id: string
    dn: string
    name: string
    description: string
    member_count: number
    last_sync: string
}

export default function IdentityPage() {
    const { user, loading } = useAuth()
    const router = useRouter()
    const [activeTab, setActiveTab] = useState<'users' | 'computers' | 'groups'>('users')
    const [syncLoading, setSyncLoading] = useState(false)
    const [syncStatus, setSyncStatus] = useState<'idle' | 'syncing' | 'success' | 'error'>('idle')
    const [lastSync, setLastSync] = useState<string | null>(null)
    const [zones, setZones] = useState<{ id: string, name: string }[]>([])
    const [importZone, setImportZone] = useState('')
    const [config, setConfig] = useState({
        host: '',
        port: 389,
        base_dn: '',
        bind_dn: '',
        bind_password: '',
        user_filter: '(objectClass=user)',
        computer_filter: '(objectClass=computer)',
        group_filter: '(objectClass=group)'
    })
    const [adUsers, setAdUsers] = useState<ADUser[]>([])
    const [adComputers, setAdComputers] = useState<ADComputer[]>([])
    const [adGroups, setAdGroups] = useState<ADGroup[]>([])
    const [selectedUsers, setSelectedUsers] = useState<Set<string>>(new Set())
    const [selectedComputers, setSelectedComputers] = useState<Set<string>>(new Set())
    const [selectedGroups, setSelectedGroups] = useState<Set<string>>(new Set())
    const [showImportModal, setShowImportModal] = useState(false)
    const [importRole, setImportRole] = useState('user')
    const [importing, setImporting] = useState(false)

    useEffect(() => {
        if (!loading && (!user || user.role.toLowerCase() !== 'admin')) {
            router.push('/sessions')
        }
    }, [user, loading, router])

    useEffect(() => {
        if (user?.role.toLowerCase() === 'admin') {
            // Fetch config on load
            fetch('/api/v1/identity/config')
                .then(res => res.json())
                .then(data => {
                    if (data.host) {
                        setConfig(prev => ({ ...prev, ...data }))
                    }
                })
                .catch(err => console.error('Failed to fetch config:', err))

            fetchADData()
            fetchZones()
        }
    }, [user])

    const fetchZones = () => {
        fetch('/api/v1/zones')
            .then(res => res.json())
            .then(data => {
                setZones(data.zones || [])
                if (data.zones && data.zones.length > 0) {
                    setImportZone(data.zones[0].id)
                }
            })
            .catch(err => console.error('Failed to fetch zones:', err))
    }

    const fetchADData = () => {
        fetch('/api/v1/ad-users')
            .then(res => res.json())
            .then(data => setAdUsers(data.users || []))
            .catch(err => console.error('Failed to fetch AD users:', err))

        fetch('/api/v1/ad-computers')
            .then(res => res.json())
            .then(data => setAdComputers(data.computers || []))
            .catch(err => console.error('Failed to fetch AD computers:', err))

        fetch('/api/v1/ad-groups')
            .then(res => res.json())
            .then(data => setAdGroups(data.groups || []))
            .catch(err => console.error('Failed to fetch AD groups:', err))
    }

    const handleSaveConfig = async (e: React.FormEvent) => {
        e.preventDefault()
        try {
            const res = await fetch('/api/v1/identity/config', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(config)
            })
            if (!res.ok) throw new Error('Failed to save config')
            alert('Configuration saved successfully')
        } catch (error) {
            console.error(error)
            alert('Failed to save configuration')
        }
    }

    const handleSync = async () => {
        setSyncLoading(true)
        setSyncStatus('syncing')
        try {
            const res = await fetch('/api/v1/orchestrator/sync/ad', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({}) // Empty body triggers sync with saved config
            })
            if (!res.ok) throw new Error('Sync failed')

            setSyncStatus('success')
            setLastSync(new Date().toLocaleString())
            fetchADData() // Refresh data after sync
        } catch (error) {
            setSyncStatus('error')
        } finally {
            setSyncLoading(false)
        }
    }

    const handleSelectUser = (id: string) => {
        const newSelected = new Set(selectedUsers)
        if (newSelected.has(id)) {
            newSelected.delete(id)
        } else {
            newSelected.add(id)
        }
        setSelectedUsers(newSelected)
    }

    const handleSelectGroup = (id: string) => {
        const newSelected = new Set(selectedGroups)
        if (newSelected.has(id)) {
            newSelected.delete(id)
        } else {
            newSelected.add(id)
        }
        setSelectedGroups(newSelected)
    }

    const handleSelectComputer = (id: string) => {
        const newSelected = new Set(selectedComputers)
        if (newSelected.has(id)) {
            newSelected.delete(id)
        } else {
            newSelected.add(id)
        }
        setSelectedComputers(newSelected)
    }

    const handleSelectAll = () => {
        if (activeTab === 'users') {
            if (selectedUsers.size === adUsers.length) {
                setSelectedUsers(new Set())
            } else {
                setSelectedUsers(new Set(adUsers.map(u => u.id)))
            }
        } else if (activeTab === 'groups') {
            if (selectedGroups.size === adGroups.length) {
                setSelectedGroups(new Set())
            } else {
                setSelectedGroups(new Set(adGroups.map(g => g.id)))
            }
        } else if (activeTab === 'computers') {
            if (selectedComputers.size === adComputers.length) {
                setSelectedComputers(new Set())
            } else {
                setSelectedComputers(new Set(adComputers.map(c => c.id)))
            }
        }
    }

    const handleImport = async () => {
        setImporting(true)
        try {
            if (activeTab === 'users') {
                for (const userId of Array.from(selectedUsers)) {
                    const res = await fetch('/api/v1/users/import', {
                        method: 'POST',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify({
                            ad_user_id: userId,
                            role: importRole
                        })
                    })
                    if (!res.ok) {
                        throw new Error(`Failed to import user ${userId}: ${res.statusText}`)
                    }
                }
                alert('Users imported successfully')
                setSelectedUsers(new Set())
            } else if (activeTab === 'groups') {
                for (const groupId of Array.from(selectedGroups)) {
                    const res = await fetch('/api/v1/groups/import', {
                        method: 'POST',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify({
                            ad_group_id: groupId,
                            role: importRole
                        })
                    })
                    if (!res.ok) {
                        throw new Error(`Failed to import group ${groupId}: ${res.statusText}`)
                    }
                }
                alert('Groups imported successfully')
                setSelectedGroups(new Set())
            } else if (activeTab === 'computers') {
                for (const computerId of Array.from(selectedComputers)) {
                    const res = await fetch('/api/v1/computers/import', {
                        method: 'POST',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify({
                            ad_computer_id: computerId,
                            zone_id: importZone,
                            protocol: 'rdp',
                            port: 3389
                        })
                    })
                    if (!res.ok) {
                        throw new Error(`Failed to import computer ${computerId}: ${res.statusText}`)
                    }
                }
                alert('Devices imported successfully')
                setSelectedComputers(new Set())
            }
            setShowImportModal(false)
            // Optionally refresh users list if we were displaying it here, but we aren't.
        } catch (error) {
            console.error('Import failed:', error)
            alert('Failed to import some users')
        } finally {
            setImporting(false)
        }
    }

    return (
        <div className="min-h-screen bg-background">

            <main className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
                <div className="mb-8 flex justify-between items-center">
                    <div>
                        <h1 className="text-3xl font-bold text-foreground">AD Synchronization</h1>
                        <p className="text-muted-foreground mt-2">Configure and manage Active Directory synchronization</p>
                    </div>
                    <div className="flex items-center space-x-4">
                        <div className="text-sm text-right">
                            <div className="text-muted-foreground">Last Sync: {lastSync || 'Never'}</div>
                            <div className={`font-medium ${syncStatus === 'success' ? 'text-green-600' :
                                syncStatus === 'error' ? 'text-red-600' :
                                    syncStatus === 'syncing' ? 'text-blue-600' : 'text-muted-foreground'
                                }`}>
                                {syncStatus === 'idle' ? 'Idle' :
                                    syncStatus.charAt(0).toUpperCase() + syncStatus.slice(1)}
                            </div>
                        </div>
                    </div>
                </div>

                <div className="grid grid-cols-1 lg:grid-cols-3 gap-8 mb-8">
                    {/* Configuration Form */}
                    <div className="lg:col-span-2 space-y-6">
                        <div className="bg-card shadow rounded-lg p-6">
                            <h2 className="text-xl font-semibold text-foreground mb-4">Connection Settings</h2>
                            <form className="space-y-4" onSubmit={handleSaveConfig}>
                                <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                                    <div>
                                        <label className="block text-sm font-medium text-foreground">Host</label>
                                        <input
                                            type="text"
                                            className="mt-1 block w-full rounded-md border-input shadow-sm focus:border-indigo-500 focus:ring-indigo-500 sm:text-sm p-2 border"
                                            placeholder="dc.example.com"
                                            value={config.host}
                                            onChange={e => setConfig({ ...config, host: e.target.value })}
                                        />
                                    </div>
                                    <div>
                                        <label className="block text-sm font-medium text-foreground">Port</label>
                                        <input
                                            type="number"
                                            className="mt-1 block w-full rounded-md border-input shadow-sm focus:border-indigo-500 focus:ring-indigo-500 sm:text-sm p-2 border"
                                            placeholder="389"
                                            value={config.port}
                                            onChange={e => setConfig({ ...config, port: parseInt(e.target.value) })}
                                        />
                                    </div>
                                </div>

                                <div>
                                    <label className="block text-sm font-medium text-foreground">Base DN</label>
                                    <input
                                        type="text"
                                        className="mt-1 block w-full rounded-md border-input shadow-sm focus:border-indigo-500 focus:ring-indigo-500 sm:text-sm p-2 border"
                                        placeholder="DC=example,DC=com"
                                        value={config.base_dn}
                                        onChange={e => setConfig({ ...config, base_dn: e.target.value })}
                                    />
                                </div>

                                <div>
                                    <label className="block text-sm font-medium text-foreground">Bind DN</label>
                                    <input
                                        type="text"
                                        className="mt-1 block w-full rounded-md border-input shadow-sm focus:border-indigo-500 focus:ring-indigo-500 sm:text-sm p-2 border"
                                        placeholder="CN=service,OU=Users,DC=example,DC=com"
                                        value={config.bind_dn}
                                        onChange={e => setConfig({ ...config, bind_dn: e.target.value })}
                                    />
                                </div>

                                <div>
                                    <label className="block text-sm font-medium text-foreground">Bind Password</label>
                                    <input
                                        type="password"
                                        className="mt-1 block w-full rounded-md border-input shadow-sm focus:border-indigo-500 focus:ring-indigo-500 sm:text-sm p-2 border"
                                        placeholder="********"
                                        value={config.bind_password}
                                        onChange={e => setConfig({ ...config, bind_password: e.target.value })}
                                    />
                                </div>

                                <div className="pt-4">
                                    <Button type="submit" className="w-full">Save Configuration</Button>
                                </div>
                            </form>
                        </div>

                        <div className="bg-card shadow rounded-lg p-6">
                            <h2 className="text-xl font-semibold text-foreground mb-4">Filters</h2>
                            <form className="space-y-4">
                                <div>
                                    <label className="block text-sm font-medium text-foreground">User Filter</label>
                                    <input
                                        type="text"
                                        className="mt-1 block w-full rounded-md border-input shadow-sm focus:border-indigo-500 focus:ring-indigo-500 sm:text-sm p-2 border"
                                        value={config.user_filter}
                                        onChange={e => setConfig({ ...config, user_filter: e.target.value })}
                                    />
                                </div>
                                <div>
                                    <label className="block text-sm font-medium text-foreground">Computer Filter</label>
                                    <input
                                        type="text"
                                        className="mt-1 block w-full rounded-md border-input shadow-sm focus:border-indigo-500 focus:ring-indigo-500 sm:text-sm p-2 border"
                                        value={config.computer_filter}
                                        onChange={e => setConfig({ ...config, computer_filter: e.target.value })}
                                    />
                                </div>
                                <div>
                                    <label className="block text-sm font-medium text-foreground">Group Filter</label>
                                    <input
                                        type="text"
                                        className="mt-1 block w-full rounded-md border-input shadow-sm focus:border-indigo-500 focus:ring-indigo-500 sm:text-sm p-2 border"
                                        value={config.group_filter}
                                        onChange={e => setConfig({ ...config, group_filter: e.target.value })}
                                    />
                                </div>
                                <div className="pt-4">
                                    <Button variant="outline" className="w-full">Update Filters</Button>
                                </div>
                            </form>
                        </div>
                    </div>

                    {/* Sync Status and Schedule Side Panel */}
                    <div className="space-y-6">
                        <div className="bg-card shadow rounded-lg p-6">
                            <h2 className="text-xl font-semibold text-foreground mb-4">Sync Status</h2>

                            <div className="space-y-4">
                                <div className="flex justify-between items-center">
                                    <span className="text-muted-foreground">Last Sync:</span>
                                    <span className="font-medium">{lastSync || 'Never'}</span>
                                </div>

                                <div className="flex justify-between items-center">
                                    <span className="text-muted-foreground">Status:</span>
                                    <span className={`font-medium ${syncStatus === 'success' ? 'text-green-600' :
                                        syncStatus === 'error' ? 'text-red-600' :
                                            syncStatus === 'syncing' ? 'text-blue-600' : 'text-muted-foreground'
                                        }`}>
                                        {syncStatus.charAt(0).toUpperCase() + syncStatus.slice(1)}
                                    </span>
                                </div>

                                <div className="pt-6">
                                    <Button
                                        onClick={handleSync}
                                        disabled={syncLoading}
                                        className="w-full"
                                        variant={syncStatus === 'error' ? 'destructive' : 'default'}
                                    >
                                        {syncLoading ? 'Syncing...' : 'Sync Now'}
                                    </Button>
                                </div>
                            </div>
                        </div>

                        <div className="bg-card shadow rounded-lg p-6">
                            <h2 className="text-xl font-semibold text-foreground mb-4">Schedule</h2>
                            <div className="space-y-4">
                                <div className="flex items-center justify-between">
                                    <span className="text-foreground">Daily Sync</span>
                                    <div className="relative inline-block w-10 mr-2 align-middle select-none transition duration-200 ease-in">
                                        <input type="checkbox" name="toggle" id="toggle" className="toggle-checkbox absolute block w-6 h-6 rounded-full bg-card border-4 appearance-none cursor-pointer" />
                                        <label htmlFor="toggle" className="toggle-label block overflow-hidden h-6 rounded-full bg-gray-300 cursor-pointer"></label>
                                    </div>
                                </div>
                                <p className="text-sm text-muted-foreground">Automatically sync every day at 00:00 UTC</p>
                            </div>
                        </div>
                    </div>
                </div>

                {/* Tabs and Actions */}
                <div className="flex justify-between items-end border-b border-border mb-6">
                    <nav className="-mb-px flex space-x-8">
                        <button
                            onClick={() => setActiveTab('users')}
                            className={`${activeTab === 'users'
                                ? 'border-indigo-500 text-indigo-600'
                                : 'border-transparent text-muted-foreground hover:text-foreground hover:border-input'
                                } whitespace-nowrap py-4 px-1 border-b-2 font-medium text-sm`}
                        >
                            AD Users ({adUsers.length})
                        </button>
                        <button
                            onClick={() => setActiveTab('computers')}
                            className={`${activeTab === 'computers'
                                ? 'border-indigo-500 text-indigo-600'
                                : 'border-transparent text-muted-foreground hover:text-foreground hover:border-input'
                                } whitespace-nowrap py-4 px-1 border-b-2 font-medium text-sm`}
                        >
                            AD Devices ({adComputers.length})
                        </button>
                        <button
                            onClick={() => setActiveTab('groups')}
                            className={`${activeTab === 'groups'
                                ? 'border-indigo-500 text-indigo-600'
                                : 'border-transparent text-muted-foreground hover:text-foreground hover:border-input'
                                } whitespace-nowrap py-4 px-1 border-b-2 font-medium text-sm`}
                        >
                            AD Groups ({adGroups.length})
                        </button>
                    </nav>
                    {activeTab === 'users' && (
                        <div className="pb-2">
                            <Button
                                onClick={() => setShowImportModal(true)}
                                disabled={selectedUsers.size === 0}
                                variant="default"
                            >
                                Add to OpenPAM ({selectedUsers.size})
                            </Button>
                        </div>
                    )}
                    {activeTab === 'groups' && (
                        <div className="pb-2">
                            <Button
                                onClick={() => setShowImportModal(true)}
                                disabled={selectedGroups.size === 0}
                                variant="default"
                            >
                                Add to OpenPAM ({selectedGroups.size})
                            </Button>
                        </div>
                    )}
                    {activeTab === 'computers' && (
                        <div className="pb-2">
                            <Button
                                onClick={() => setShowImportModal(true)}
                                disabled={selectedComputers.size === 0}
                                variant="default"
                            >
                                Add to OpenPAM ({selectedComputers.size})
                            </Button>
                        </div>
                    )}
                </div>

                {/* Tab Content */}
                {activeTab === 'users' && (
                    <div className="bg-card shadow rounded-lg overflow-hidden">
                        <div className="overflow-x-auto">
                            <table className="min-w-full divide-y divide-border">
                                <thead className="bg-background">
                                    <tr>
                                        <th className="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase tracking-wider">
                                            <input
                                                type="checkbox"
                                                className="rounded border-input text-indigo-600 focus:ring-indigo-500"
                                                checked={adUsers.length > 0 && selectedUsers.size === adUsers.length}
                                                onChange={handleSelectAll}
                                            />
                                        </th>
                                        <th className="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase tracking-wider">Display Name</th>
                                        <th className="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase tracking-wider">SAM Account</th>
                                        <th className="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase tracking-wider">UPN</th>
                                        <th className="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase tracking-wider">Email</th>
                                        <th className="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase tracking-wider">OU</th>
                                        <th className="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase tracking-wider">Status</th>
                                        <th className="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase tracking-wider">Password</th>
                                        <th className="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase tracking-wider">Last Sync</th>
                                    </tr>
                                </thead>
                                <tbody className="bg-card divide-y divide-border">
                                    {adUsers.map((user) => (
                                        <tr key={user.id}>
                                            <td className="px-6 py-4 whitespace-nowrap">
                                                <input
                                                    type="checkbox"
                                                    className="rounded border-input text-indigo-600 focus:ring-indigo-500"
                                                    checked={selectedUsers.has(user.id)}
                                                    onChange={() => handleSelectUser(user.id)}
                                                />
                                            </td>
                                            <td className="px-6 py-4 whitespace-nowrap text-sm font-medium text-foreground">{user.display_name}</td>
                                            <td className="px-6 py-4 whitespace-nowrap text-sm text-muted-foreground">{user.sam_account_name}</td>
                                            <td className="px-6 py-4 whitespace-nowrap text-sm text-muted-foreground">{user.user_principal_name}</td>
                                            <td className="px-6 py-4 whitespace-nowrap text-sm text-muted-foreground">{user.mail}</td>
                                            <td className="px-6 py-4 whitespace-nowrap text-sm text-muted-foreground">{user.ou}</td>
                                            <td className="px-6 py-4 whitespace-nowrap text-sm text-muted-foreground">
                                                <span className={`px-2 inline-flex text-xs leading-5 font-semibold rounded-full ${user.status === 'Active' ? 'bg-green-100 text-green-800' :
                                                    user.status === 'Disabled' ? 'bg-gray-100 text-gray-800' :
                                                        'bg-red-100 text-red-800'
                                                    }`}>
                                                    {user.status}
                                                </span>
                                            </td>
                                            <td className="px-6 py-4 whitespace-nowrap text-sm text-muted-foreground">{user.password_status}</td>
                                            <td className="px-6 py-4 whitespace-nowrap text-sm text-muted-foreground">{new Date(user.last_sync).toLocaleString()}</td>
                                        </tr>
                                    ))}
                                    {adUsers.length === 0 && (
                                        <tr>
                                            <td colSpan={9} className="px-6 py-4 text-center text-sm text-muted-foreground">No AD users found. Run a sync to populate.</td>
                                        </tr>
                                    )}
                                </tbody>
                            </table>
                        </div>
                    </div>
                )}

                {activeTab === 'computers' && (
                    <div className="bg-card shadow rounded-lg overflow-hidden">
                        <div className="overflow-x-auto">
                            <table className="min-w-full divide-y divide-border">
                                <thead className="bg-background">
                                    <tr>
                                        <th className="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase tracking-wider">
                                            <input
                                                type="checkbox"
                                                className="rounded border-input text-indigo-600 focus:ring-indigo-500"
                                                checked={adComputers.length > 0 && selectedComputers.size === adComputers.length}
                                                onChange={handleSelectAll}
                                            />
                                        </th>
                                        <th className="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase tracking-wider">Name</th>
                                        <th className="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase tracking-wider">DNS Hostname</th>
                                        <th className="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase tracking-wider">OS</th>
                                        <th className="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase tracking-wider">OS Version</th>
                                        <th className="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase tracking-wider">Last Sync</th>
                                    </tr>
                                </thead>
                                <tbody className="bg-card divide-y divide-border">
                                    {adComputers.map((computer) => (
                                        <tr key={computer.id}>
                                            <td className="px-6 py-4 whitespace-nowrap">
                                                <input
                                                    type="checkbox"
                                                    className="rounded border-input text-indigo-600 focus:ring-indigo-500"
                                                    checked={selectedComputers.has(computer.id)}
                                                    onChange={() => handleSelectComputer(computer.id)}
                                                />
                                            </td>
                                            <td className="px-6 py-4 whitespace-nowrap text-sm font-medium text-foreground">{computer.name}</td>
                                            <td className="px-6 py-4 whitespace-nowrap text-sm text-muted-foreground">{computer.dns_host_name}</td>
                                            <td className="px-6 py-4 whitespace-nowrap text-sm text-muted-foreground">{computer.operating_system}</td>
                                            <td className="px-6 py-4 whitespace-nowrap text-sm text-muted-foreground">{computer.operating_system_version}</td>
                                            <td className="px-6 py-4 whitespace-nowrap text-sm text-muted-foreground">{new Date(computer.last_sync).toLocaleString()}</td>
                                        </tr>
                                    ))}
                                    {adComputers.length === 0 && (
                                        <tr>
                                            <td colSpan={6} className="px-6 py-4 text-center text-sm text-muted-foreground">No AD devices found. Run a sync to populate.</td>
                                        </tr>
                                    )}
                                </tbody>
                            </table>
                        </div>
                    </div>
                )}

                {activeTab === 'groups' && (
                    <div className="bg-card shadow rounded-lg overflow-hidden">
                        <div className="overflow-x-auto">
                            <table className="min-w-full divide-y divide-border">
                                <thead className="bg-background">
                                    <tr>
                                        <th className="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase tracking-wider">
                                            <input
                                                type="checkbox"
                                                className="rounded border-input text-indigo-600 focus:ring-indigo-500"
                                                checked={adGroups.length > 0 && selectedGroups.size === adGroups.length}
                                                onChange={handleSelectAll}
                                            />
                                        </th>
                                        <th className="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase tracking-wider">Name</th>
                                        <th className="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase tracking-wider">Description</th>
                                        <th className="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase tracking-wider">Members</th>
                                        <th className="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase tracking-wider">Last Sync</th>
                                    </tr>
                                </thead>
                                <tbody className="bg-card divide-y divide-border">
                                    {adGroups.map((group) => (
                                        <tr key={group.id}>
                                            <td className="px-6 py-4 whitespace-nowrap">
                                                <input
                                                    type="checkbox"
                                                    className="rounded border-input text-indigo-600 focus:ring-indigo-500"
                                                    checked={selectedGroups.has(group.id)}
                                                    onChange={() => handleSelectGroup(group.id)}
                                                />
                                            </td>
                                            <td className="px-6 py-4 whitespace-nowrap text-sm font-medium text-foreground">{group.name}</td>
                                            <td className="px-6 py-4 whitespace-nowrap text-sm text-muted-foreground">{group.description}</td>
                                            <td className="px-6 py-4 whitespace-nowrap text-sm text-muted-foreground">{group.member_count}</td>
                                            <td className="px-6 py-4 whitespace-nowrap text-sm text-muted-foreground">{new Date(group.last_sync).toLocaleString()}</td>
                                        </tr>
                                    ))}
                                    {adGroups.length === 0 && (
                                        <tr>
                                            <td colSpan={5} className="px-6 py-4 text-center text-sm text-muted-foreground">No AD groups found. Run a sync to populate.</td>
                                        </tr>
                                    )}
                                </tbody>
                            </table>
                        </div>
                    </div>
                )}
            </main>

            {/* Import Role Modal */}
            {showImportModal && (
                <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
                    <div className="bg-card rounded-lg p-6 max-w-md w-full mx-4">
                        <h2 className="text-xl font-bold text-foreground mb-4">
                            {activeTab === 'users' ? 'Add Users to OpenPAM' :
                                activeTab === 'groups' ? 'Add Groups to OpenPAM' :
                                    'Add Devices to OpenPAM'}
                        </h2>
                        <p className="text-muted-foreground mb-6">
                            {activeTab === 'computers'
                                ? `Select a zone for the ${selectedComputers.size} selected ${selectedComputers.size !== 1 ? 'devices' : 'device'}.`
                                : `Select a role for the ${activeTab === 'users' ? selectedUsers.size : selectedGroups.size} selected ${activeTab === 'users' ? (selectedUsers.size !== 1 ? 'users' : 'user') : (selectedGroups.size !== 1 ? 'groups' : 'group')}.`
                            }
                        </p>

                        <div className="space-y-3 mb-6">
                            {activeTab === 'computers' ? (
                                <div>
                                    <label className="block text-sm font-medium text-foreground mb-1">Zone</label>
                                    {zones.length > 0 ? (
                                        <select
                                            value={importZone}
                                            onChange={(e) => setImportZone(e.target.value)}
                                            className="w-full px-3 py-2 border border-input rounded-md"
                                        >
                                            {zones.map((zone) => (
                                                <option key={zone.id} value={zone.id}>{zone.name}</option>
                                            ))}
                                        </select>
                                    ) : (
                                        <div className="text-sm text-red-600 bg-red-50 p-3 rounded-md border border-red-200">
                                            No zones found. You must <a href="/admin/zones" className="underline font-medium hover:text-red-800">create a zone</a> before importing devices.
                                        </div>
                                    )}
                                </div>
                            ) : (
                                <>
                                    {['admin', 'user', 'auditor'].map((role) => (
                                        <div key={role} className="flex items-center">
                                            <input
                                                type="radio"
                                                id={role}
                                                name="role"
                                                value={role}
                                                checked={importRole === role}
                                                onChange={(e) => setImportRole(e.target.value)}
                                                className="h-4 w-4 text-indigo-600 focus:ring-indigo-500 border-input"
                                            />
                                            <label htmlFor={role} className="ml-3 block text-sm font-medium text-foreground capitalize">
                                                {role}
                                            </label>
                                        </div>
                                    ))}
                                    <div className="flex items-center">
                                        <input
                                            type="radio"
                                            id="managed"
                                            name="role"
                                            value="managed"
                                            checked={importRole === 'managed'}
                                            onChange={(e) => setImportRole(e.target.value)}
                                            className="h-4 w-4 text-indigo-600 focus:ring-indigo-500 border-input"
                                        />
                                        <label htmlFor="managed" className="ml-3 block text-sm font-medium text-foreground">
                                            Managed Account (No Login)
                                        </label>
                                    </div>
                                </>
                            )}
                        </div>

                        <div className="flex justify-end space-x-3">
                            <Button
                                variant="outline"
                                onClick={() => setShowImportModal(false)}
                                disabled={importing}
                            >
                                Cancel
                            </Button>
                            <Button
                                onClick={handleImport}
                                disabled={importing || (activeTab === 'computers' && zones.length === 0)}
                            >
                                {importing ? 'Importing...' : 'Confirm Import'}
                            </Button>
                        </div>
                    </div>
                </div>
            )}
        </div>
    )
}
