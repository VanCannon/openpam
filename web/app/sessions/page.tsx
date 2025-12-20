'use client'

import { useAuth } from '@/lib/auth-context'
import { api } from '@/lib/api'
import { Target, Credential, User, ManagedAccount } from '@/types'
import { Schedule } from '@/types/schedule'
import { useRouter } from 'next/navigation'
import { useEffect, useState } from 'react'
import dynamic from 'next/dynamic'

const Terminal = dynamic(() => import('@/components/terminal'), { ssr: false })
const RdpViewer = dynamic(() => import('@/components/rdp-viewer'), { ssr: false })

export default function DashboardPage() {
  const { user, loading, logout } = useAuth()
  const router = useRouter()
  const [targets, setTargets] = useState<Target[]>([])
  const [schedules, setSchedules] = useState<Schedule[]>([])
  const [users, setUsers] = useState<User[]>([])
  const [loadingData, setLoadingData] = useState(true)
  const [activeConnection, setActiveConnection] = useState<{ target: Target; credential?: Credential; schedule?: Schedule; promotionPassword?: string } | null>(null)

  // Request Modal State
  const [showRequestModal, setShowRequestModal] = useState(false)
  const [selectedTargetId, setSelectedTargetId] = useState('')
  const [startTime, setStartTime] = useState('')
  const [endTime, setEndTime] = useState('')
  const [requesting, setRequesting] = useState(false)
  const [scheduleType, setScheduleType] = useState('scheduled')
  const [accountType, setAccountType] = useState('static')
  const [requestForUserId, setRequestForUserId] = useState('')
  // New State for Dynamic Accounts
  const [managedAccounts, setManagedAccounts] = useState<ManagedAccount[]>([])
  const [selectedManagedAccountId, setSelectedManagedAccountId] = useState('')
  const [ephemeralPrefix, setEphemeralPrefix] = useState('pam-user')
  const [promotionGroup, setPromotionGroup] = useState('')
  const [connecting, setConnecting] = useState(false)
  const [connectionStatus, setConnectionStatus] = useState('')
  // Password prompt for promotion accounts
  const [showPasswordModal, setShowPasswordModal] = useState(false)
  const [promotionPassword, setPromotionPassword] = useState('')
  const [pendingPromotionSchedule, setPendingPromotionSchedule] = useState<Schedule | null>(null)

  useEffect(() => {
    if (!loading && !user) {
      router.push('/login')
    }
  }, [user, loading, router])

  useEffect(() => {
    if (!user) return

    fetchData()

    // Use SSE for real-time schedule updates with auto-reconnect
    const token = localStorage.getItem('openpam_token')
    if (!token) {
      // Token not available yet, skip SSE connection
      return
    }

    let eventSource: EventSource | null = null
    let reconnectTimeout: NodeJS.Timeout | null = null
    let reconnectAttempts = 0
    const maxReconnectDelay = 30000 // Max 30 seconds

    const connect = () => {
      console.log('Connecting to SSE stream...')

      eventSource = new EventSource(
        `http://localhost:8080/api/v1/schedules/stream?token=${encodeURIComponent(token)}`,
        { withCredentials: true }
      )

      eventSource.onopen = () => {
        console.log('SSE connection established')
        reconnectAttempts = 0 // Reset on successful connection
      }

      eventSource.onmessage = (event) => {
        try {
          const data = JSON.parse(event.data)

          if (data.type === 'connected') {
            console.log('SSE connected:', data)
            return
          }

          // Handle schedule update events
          if (data.type === 'schedule.activated' ||
            data.type === 'schedule.expired' ||
            data.type === 'schedule.updated' ||
            data.type === 'schedule.approved' ||
            data.type === 'schedule.rejected') {
            console.log('Schedule update received:', data.type)
            fetchData()
          }
        } catch (error) {
          console.error('Failed to parse SSE event:', error)
        }
      }

      eventSource.onerror = (error) => {
        console.error('SSE connection error, will reconnect...', error)
        eventSource?.close()

        // Exponential backoff: 1s, 2s, 4s, 8s, ... up to 30s
        const delay = Math.min(1000 * Math.pow(2, reconnectAttempts), maxReconnectDelay)
        reconnectAttempts++

        console.log(`Reconnecting in ${delay}ms (attempt ${reconnectAttempts})`)
        reconnectTimeout = setTimeout(connect, delay)
      }
    }

    connect()

    return () => {
      console.log('Cleaning up SSE connection')
      if (reconnectTimeout) clearTimeout(reconnectTimeout)
      eventSource?.close()
    }
  }, [user])

  useEffect(() => {
    if (showRequestModal && accountType === 'managed') {
      api.listManagedAccounts()
        .then(resp => setManagedAccounts(resp.accounts || []))
        .catch(err => console.error('Failed to fetch managed accounts:', err))
    }
  }, [showRequestModal, accountType])

  const fetchData = async () => {
    try {
      setLoadingData(true)
      const [targetsResponse, schedulesResponse] = await Promise.all([
        api.listTargets(),
        api.listSchedules({ user_id: user?.id })
      ])
      setTargets(targetsResponse.targets || [])
      setSchedules(schedulesResponse.schedules || [])

      if (user?.role === 'admin') {
        const usersResponse = await api.listUsers()
        setUsers(usersResponse.users || [])
      }
    } catch (error) {
      console.error('Failed to load data:', error)
    } finally {
      setLoadingData(false)
    }
  }

  const handleConnect = async (schedule: Schedule, password?: string) => {
    const target = targets.find(t => t.id === schedule.target_id)
    if (!target) return

    // For promotion accounts, prompt for password if not provided
    if (schedule.account_type === 'promotion' && !password) {
      setPendingPromotionSchedule(schedule)
      setShowPasswordModal(true)
      return
    }

    setConnecting(true)
    if (schedule.account_type === 'managed') {
      setConnectionStatus('Enabling Managed Account in Active Directory...')
      await new Promise(resolve => setTimeout(resolve, 1500))
    } else if (schedule.account_type === 'promotion') {
      setConnectionStatus('Promoting user to Domain Admins...')
      await new Promise(resolve => setTimeout(resolve, 1000))
    }
    setConnectionStatus('Establishing secure connection...')
    await new Promise(resolve => setTimeout(resolve, 800))

    setActiveConnection({ target, schedule, promotionPassword: password })
    setConnecting(false)
    setConnectionStatus('')
  }

  const handlePasswordSubmit = () => {
    if (pendingPromotionSchedule && promotionPassword) {
      setShowPasswordModal(false)
      handleConnect(pendingPromotionSchedule, promotionPassword)
      setPromotionPassword('')
      setPendingPromotionSchedule(null)
    }
  }

  const handlePasswordCancel = () => {
    setShowPasswordModal(false)
    setPromotionPassword('')
    setPendingPromotionSchedule(null)
  }

  const handleDisconnect = () => {
    setActiveConnection(null)
  }

  const handleRequestSchedule = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!selectedTargetId || !user) return
    if (scheduleType === 'scheduled' && (!startTime || !endTime)) return

    try {
      setRequesting(true)
      await api.requestSchedule({
        user_id: requestForUserId || user.id,
        target_id: selectedTargetId,
        start_time: scheduleType === 'standing' ? new Date().toISOString() : new Date(startTime).toISOString(),
        end_time: scheduleType === 'standing' ? new Date(new Date().setFullYear(new Date().getFullYear() + 100)).toISOString() : new Date(endTime).toISOString(),
        timezone: Intl.DateTimeFormat().resolvedOptions().timeZone,
        type: scheduleType,
        account_type: accountType,
        account_details: {
          managed_account_id: accountType === 'managed' ? managedAccounts.find(a => a.sam_account_name === selectedManagedAccountId)?.id : undefined,
          vault_secret_path: accountType === 'managed' ? managedAccounts.find(a => a.sam_account_name === selectedManagedAccountId)?.vault_secret_path : undefined,
          ephemeral_prefix: accountType === 'ephemeral' ? ephemeralPrefix : undefined,
          promotion_group: accountType === 'promotion' ? promotionGroup : undefined
        }
      })

      setShowRequestModal(false)
      setSelectedTargetId('')
      setStartTime('')
      setEndTime('')
      setRequestForUserId('')
      fetchData()
    } catch (error) {
      console.error('Failed to request schedule:', error)
      alert('Failed to request schedule')
    } finally {
      setRequesting(false)
    }
  }

  if (loading || !user) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <p>Loading...</p>
      </div>
    )
  }

  if (activeConnection) {
    // We need a credential ID for the websocket URL. 
    // If we have a schedule, the backend should know which credential to use or we pass a placeholder.
    // For now, let's assume we need to pass something.
    // TODO: Update backend to handle session-based connection without explicit credential ID if session ID is provided.
    // For now, we will use a dummy credential ID if not available, relying on the backend to validate the session.

    const wsUrl = api.getWebSocketUrl(
      activeConnection.target.protocol,
      activeConnection.target.id,
      'session-' + activeConnection.schedule?.id, // Hack: Pass session ID as credential ID for now
      activeConnection.promotionPassword // Pass password for promotion accounts
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

  const activeSessions = schedules.filter(s => s.status === 'active' && s.type === 'scheduled')
  const standingAccess = schedules.filter(s => s.type === 'standing' && s.status === 'active')
  const upcomingSessions = schedules.filter(s => s.status !== 'active' && (s.status === 'pending' || (s.approval_status === 'approved' && new Date(s.start_time) > new Date())))

  return (
    <div className="min-h-screen bg-background ">

      <main className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
        <div className="flex justify-between items-center mb-8">
          <h1 className="text-2xl font-bold text-foreground ">Sessions</h1>
          <button
            onClick={() => setShowRequestModal(true)}
            className="px-4 py-2 bg-indigo-600 text-white rounded-lg hover:bg-indigo-700 transition-colors"
          >
            Request Access
          </button>
        </div>

        {loadingData ? (
          <div className="text-center py-12">
            <p className="text-muted-foreground">Loading sessions...</p>
          </div>
        ) : (
          <div className="space-y-8">
            {/* Active Sessions */}
            <section>
              <h2 className="text-lg font-semibold text-foreground  mb-4 flex items-center">
                <span className="w-2 h-2 bg-green-500 rounded-full mr-2"></span>
                Active Sessions
              </h2>
              {activeSessions.length === 0 ? (
                <p className="text-muted-foreground text-sm">No active scheduled sessions.</p>
              ) : (
                <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
                  {activeSessions.map(session => {
                    const target = targets.find(t => t.id === session.target_id)
                    return (
                      <div key={session.id} className="bg-card  p-6 rounded-lg shadow border-l-4 border-green-500">
                        <h3 className="font-semibold text-foreground ">{target?.name || 'Unknown Target'}</h3>
                        <p className="text-sm text-muted-foreground mb-4">{target?.hostname}</p>
                        <div className="flex justify-between items-center">
                          <span className="text-xs text-muted-foreground">Ends at {new Date(session.end_time).toLocaleTimeString()}</span>
                          <button
                            onClick={() => handleConnect(session)}
                            className="px-3 py-1 bg-green-600 text-white text-sm rounded hover:bg-green-700"
                          >
                            Connect
                          </button>
                        </div>
                      </div>
                    )
                  })}
                </div>
              )}
            </section>

            {/* Standing Access */}
            <section>
              <h2 className="text-lg font-semibold text-foreground  mb-4 flex items-center">
                <span className="w-2 h-2 bg-blue-500 rounded-full mr-2"></span>
                Standing Access
              </h2>
              {standingAccess.length === 0 ? (
                <p className="text-muted-foreground text-sm">No standing access configured.</p>
              ) : (
                <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
                  {standingAccess.map(session => {
                    const target = targets.find(t => t.id === session.target_id)
                    return (
                      <div key={session.id} className="bg-card  p-6 rounded-lg shadow border-l-4 border-blue-500">
                        <h3 className="font-semibold text-foreground ">{target?.name || 'Unknown Target'}</h3>
                        <p className="text-sm text-muted-foreground mb-4">{target?.hostname}</p>
                        <div className="flex justify-between items-center">
                          <span className="text-xs text-muted-foreground">Always Available</span>
                          <button
                            onClick={() => handleConnect(session)}
                            className="px-3 py-1 bg-blue-600 text-white text-sm rounded hover:bg-blue-700"
                          >
                            Connect
                          </button>
                        </div>
                      </div>
                    )
                  })}
                </div>
              )}
            </section>

            {/* Upcoming Sessions */}
            <section>
              <h2 className="text-lg font-semibold text-foreground  mb-4">Upcoming Sessions</h2>
              {upcomingSessions.length === 0 ? (
                <p className="text-muted-foreground text-sm">No upcoming sessions.</p>
              ) : (
                <div className="bg-card  shadow rounded-lg overflow-hidden">
                  <table className="min-w-full divide-y divide-border ">
                    <thead className="bg-background ">
                      <tr>
                        <th className="px-6 py-3 text-left text-xs font-medium text-muted-foreground  uppercase tracking-wider">Target</th>
                        <th className="px-6 py-3 text-left text-xs font-medium text-muted-foreground  uppercase tracking-wider">Start Time</th>
                        <th className="px-6 py-3 text-left text-xs font-medium text-muted-foreground  uppercase tracking-wider">Duration</th>
                        <th className="px-6 py-3 text-left text-xs font-medium text-muted-foreground  uppercase tracking-wider">Status</th>
                      </tr>
                    </thead>
                    <tbody className="bg-card  divide-y divide-border ">
                      {upcomingSessions.map(session => {
                        const target = targets.find(t => t.id === session.target_id)
                        const duration = (new Date(session.end_time).getTime() - new Date(session.start_time).getTime()) / (1000 * 60) // minutes
                        return (
                          <tr key={session.id}>
                            <td className="px-6 py-4 whitespace-nowrap text-sm font-medium text-foreground ">
                              {target?.name || 'Unknown'}
                            </td>
                            <td className="px-6 py-4 whitespace-nowrap text-sm text-muted-foreground ">
                              {new Date(session.start_time).toLocaleString(undefined, {
                                timeZone: session.timezone || undefined,
                                year: 'numeric',
                                month: 'numeric',
                                day: 'numeric',
                                hour: 'numeric',
                                minute: 'numeric',
                              })}
                              {session.timezone ? ` (${session.timezone})` : ''}
                            </td>
                            <td className="px-6 py-4 whitespace-nowrap text-sm text-muted-foreground ">
                              {duration} mins
                            </td>
                            <td className="px-6 py-4 whitespace-nowrap">
                              <span className={`px-2 py-1 text-xs font-semibold rounded-full ${session.approval_status === 'approved'
                                ? 'bg-green-100 text-green-800'
                                : session.approval_status === 'rejected'
                                  ? 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200'
                                  : 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200'
                                }`}>
                                {session.approval_status === 'approved'
                                  ? 'Approved'
                                  : session.approval_status === 'rejected'
                                    ? 'Rejected'
                                    : 'Pending Approval'}
                              </span>
                              {session.approval_status === 'rejected' && session.rejection_reason && (
                                <p className="text-xs text-red-600 dark:text-red-400 mt-1">
                                  Reason: {session.rejection_reason}
                                </p>
                              )}
                            </td>
                          </tr>
                        )
                      })}
                    </tbody>
                  </table>
                </div>
              )}
            </section>
          </div>
        )}
      </main>

      {/* Request Modal */}
      {showRequestModal && (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
          <div className="bg-card  rounded-lg p-6 max-w-md w-full mx-4">
            <h2 className="text-xl font-bold text-foreground  mb-4">Request Access</h2>
            <form onSubmit={handleRequestSchedule} className="space-y-4">
              {user?.role === 'admin' && (
                <div>
                  <label className="block text-sm font-medium text-foreground  mb-1">Request For User</label>
                  <select
                    value={requestForUserId}
                    onChange={(e) => setRequestForUserId(e.target.value)}
                    className="w-full px-3 py-2 border border-input  rounded-lg bg-card  text-foreground "
                  >
                    <option value="">Myself ({user.display_name})</option>
                    {users.map((u) => (
                      <option key={u.id} value={u.id}>
                        {u.display_name} ({u.email})
                      </option>
                    ))}
                  </select>
                </div>
              )}

              <div>
                <label className="block text-sm font-medium text-foreground  mb-1">Target</label>
                <select
                  value={selectedTargetId}
                  onChange={(e) => setSelectedTargetId(e.target.value)}
                  required
                  className="w-full px-3 py-2 border border-input  rounded-lg bg-card  text-foreground "
                >
                  <option value="">Select a target...</option>
                  {targets.map((target) => (
                    <option key={target.id} value={target.id}>
                      {target.name} ({target.protocol.toUpperCase()})
                    </option>
                  ))}
                </select>
              </div>

              <div>
                <label className="block text-sm font-medium text-foreground  mb-1">Access Type</label>
                <select
                  value={scheduleType}
                  onChange={(e) => setScheduleType(e.target.value)}
                  className="w-full px-3 py-2 border border-input  rounded-lg bg-card  text-foreground "
                >
                  <option value="scheduled">Scheduled Session</option>
                  <option value="standing">Standing Access</option>
                </select>
              </div>

              <div>
                <label className="block text-sm font-medium text-foreground  mb-1">Account Type</label>
                <select
                  value={accountType}
                  onChange={(e) => setAccountType(e.target.value)}
                  className="w-full px-3 py-2 border border-input  rounded-lg bg-card  text-foreground "
                >
                  <option value="static">Static Credential</option>
                  <option value="managed">Managed Account</option>
                  <option value="ephemeral">Ephemeral Account</option>
                  <option value="promotion">AD User Promotion</option>
                </select>
              </div>

              {accountType === 'managed' && (
                <div>
                  <label className="block text-sm font-medium text-foreground mb-1">Select Account</label>
                  <select
                    value={selectedManagedAccountId}
                    onChange={(e) => setSelectedManagedAccountId(e.target.value)}
                    required
                    className="w-full px-3 py-2 border border-input rounded-lg bg-card text-foreground"
                  >
                    <option value="">Select a managed account...</option>
                    {managedAccounts.map((acc) => (
                      <option key={acc.id} value={acc.sam_account_name}>
                        {acc.display_name} ({acc.sam_account_name})
                      </option>
                    ))}
                  </select>
                </div>
              )}

              {accountType === 'ephemeral' && (
                <div>
                  <label className="block text-sm font-medium text-foreground mb-1">Username Prefix</label>
                  <input
                    type="text"
                    value={ephemeralPrefix}
                    onChange={(e) => setEphemeralPrefix(e.target.value)}
                    placeholder="e.g. pam-user"
                    className="w-full px-3 py-2 border border-input rounded-lg bg-card text-foreground"
                  />
                  <p className="text-xs text-muted-foreground mt-1">
                    A temporary AD account will be created with this prefix (e.g., {ephemeralPrefix}-xxxx).
                  </p>
                </div>
              )}

              {accountType === 'promotion' && (
                <div>
                  <label className="block text-sm font-medium text-foreground mb-1">Group to Promote To</label>
                  <input
                    type="text"
                    value={promotionGroup}
                    onChange={(e) => setPromotionGroup(e.target.value)}
                    placeholder="e.g. Domain Admins"
                    required
                    className="w-full px-3 py-2 border border-input rounded-lg bg-card text-foreground"
                  />
                  <p className="text-xs text-muted-foreground mt-1">
                    Your AD account will be temporarily added to this group.
                  </p>
                </div>
              )}

              {scheduleType === 'scheduled' && (
                <>
                  <div>
                    <label className="block text-sm font-medium text-foreground  mb-1">Start Time</label>
                    <input
                      type="datetime-local"
                      value={startTime}
                      onChange={(e) => setStartTime(e.target.value)}
                      required={scheduleType === 'scheduled'}
                      className="w-full px-3 py-2 border border-input  rounded-lg bg-card  text-foreground "
                    />
                  </div>

                  <div>
                    <label className="block text-sm font-medium text-foreground  mb-1">End Time</label>
                    <input
                      type="datetime-local"
                      value={endTime}
                      onChange={(e) => setEndTime(e.target.value)}
                      required={scheduleType === 'scheduled'}
                      className="w-full px-3 py-2 border border-input  rounded-lg bg-card  text-foreground "
                    />
                  </div>
                </>
              )}

              <div className="flex justify-end space-x-3 mt-6">
                <button
                  type="button"
                  onClick={() => setShowRequestModal(false)}
                  className="px-4 py-2 text-foreground  hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={requesting}
                  className="px-4 py-2 bg-indigo-600 text-white rounded-lg hover:bg-indigo-700 disabled:opacity-50"
                >
                  {requesting ? 'Requesting...' : 'Submit Request'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
      {/* Connecting Overlay */}
      {connecting && (
        <div className="fixed inset-0 bg-background/80 backdrop-blur-sm flex items-center justify-center z-[100]">
          <div className="flex flex-col items-center space-y-4 p-8 bg-card rounded-2xl shadow-2xl border border-border max-w-sm w-full animate-in fade-in zoom-in duration-300">
            <div className="relative w-16 h-16">
              <div className="absolute inset-0 border-4 border-indigo-500/20 rounded-full"></div>
              <div className="absolute inset-0 border-4 border-indigo-500 rounded-full border-t-transparent animate-spin"></div>
            </div>
            <div className="text-center space-y-2">
              <h3 className="text-lg font-semibold text-foreground">Initiating Session</h3>
              <p className="text-sm text-muted-foreground animate-pulse">{connectionStatus}</p>
            </div>
          </div>
        </div>
      )}

      {/* Password Modal for Promotion Accounts */}
      {showPasswordModal && (
        <div className="fixed inset-0 bg-black/60 backdrop-blur-sm flex items-center justify-center z-50">
          <div className="bg-card border border-border rounded-xl shadow-2xl p-6 max-w-md w-full mx-4">
            <h3 className="text-lg font-semibold text-foreground mb-4">Enter Your Password</h3>
            <p className="text-sm text-muted-foreground mb-4">
              This session requires your AD password for authentication.
            </p>
            <input
              type="password"
              value={promotionPassword}
              onChange={(e) => setPromotionPassword(e.target.value)}
              onKeyDown={(e) => e.key === 'Enter' && handlePasswordSubmit()}
              placeholder="Enter your password"
              className="w-full px-4 py-2 border border-border rounded-lg bg-background text-foreground mb-4 focus:outline-none focus:ring-2 focus:ring-indigo-500"
              autoFocus
            />
            <div className="flex justify-end space-x-3">
              <button
                onClick={handlePasswordCancel}
                className="px-4 py-2 text-muted-foreground hover:text-foreground transition-colors"
              >
                Cancel
              </button>
              <button
                onClick={handlePasswordSubmit}
                disabled={!promotionPassword}
                className="px-4 py-2 bg-indigo-600 text-white rounded-lg hover:bg-indigo-700 transition-colors disabled:opacity-50"
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
