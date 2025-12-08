'use client'

import { useEffect, useState, useRef } from 'react'
import { useAuth } from '@/lib/auth-context'
import { useRouter } from 'next/navigation'
import { AuditLog, User, Target, Credential } from '@/types'
import Header from '@/components/header'
import { api } from '@/lib/api'
import type { Terminal as XTerm } from '@xterm/xterm'
import type { FitAddon } from '@xterm/addon-fit'
import '@xterm/xterm/css/xterm.css'
import RdpPlayer from '@/components/rdp-player'

export default function AuditorPage() {
    const { user, loading } = useAuth()
    const router = useRouter()
    const [sessions, setSessions] = useState<AuditLog[]>([])
    const [loadingSessions, setLoadingSessions] = useState(true)
    const [filter, setFilter] = useState<'all' | 'active' | 'completed'>('all')
    const [searchTerm, setSearchTerm] = useState('')
    const [selectedSession, setSelectedSession] = useState<AuditLog | null>(null)
    const [loadingRecording, setLoadingRecording] = useState(false)
    const [recordingContent, setRecordingContent] = useState<string>('')
    const [isFullscreen, setIsFullscreen] = useState(false)
    const [leftPanelWidth, setLeftPanelWidth] = useState(400) // Initial width in pixels
    const [isDragging, setIsDragging] = useState(false)
    const terminalRef = useRef<HTMLDivElement>(null)
    const xtermRef = useRef<XTerm | null>(null)
    const wsRef = useRef<WebSocket | null>(null)
    const fitAddonRef = useRef<FitAddon | null>(null)
    const viewerContainerRef = useRef<HTMLDivElement>(null)

    // Lookup data for human-friendly names
    const [users, setUsers] = useState<Map<string, User>>(new Map())
    const [targets, setTargets] = useState<Map<string, Target>>(new Map())
    const [credentials, setCredentials] = useState<Map<string, Credential>>(new Map())

    useEffect(() => {
        if (!loading && (!user || (user.role !== 'auditor' && user.role !== 'admin'))) {
            router.push('/dashboard')
        }
    }, [user, loading, router])

    // Load lookup data on mount
    useEffect(() => {
        if (user && (user.role === 'auditor' || user.role === 'admin')) {
            fetchLookupData()
        }
    }, [user])

    useEffect(() => {
        if (user && (user.role === 'auditor' || user.role === 'admin')) {
            fetchSessions()
            const interval = setInterval(fetchSessions, 5000) // Refresh every 5 seconds
            return () => clearInterval(interval)
        }
    }, [user, filter])

    const fetchLookupData = async () => {
        try {
            // Fetch users, targets in parallel
            const [usersResp, targetsResp] = await Promise.all([
                api.listUsers(),
                api.listTargets({})
            ])

            // Build user lookup map
            const userMap = new Map<string, User>()
            if (usersResp.users) {
                usersResp.users.forEach(u => userMap.set(u.id, u))
            }
            setUsers(userMap)

            // Build target lookup map
            const targetMap = new Map<string, Target>()
            if (targetsResp.targets) {
                targetsResp.targets.forEach(t => targetMap.set(t.id, t))
            }
            setTargets(targetMap)

            // Fetch credentials for all targets
            const credMap = new Map<string, Credential>()
            if (targetsResp.targets) {
                const credPromises = targetsResp.targets.map(t =>
                    api.listCredentials(t.id).catch(() => ({ credentials: [], count: 0 }))
                )
                const credResults = await Promise.all(credPromises)
                credResults.forEach(result => {
                    result.credentials.forEach(c => credMap.set(c.id, c))
                })
            }
            setCredentials(credMap)
        } catch (error) {
            console.error('Failed to fetch lookup data:', error)
        }
    }

    const fetchSessions = async () => {
        try {
            setLoadingSessions(true)
            let fetchedSessions: AuditLog[] = []

            if (filter === 'active') {
                const response = await api.getActiveSessions()
                fetchedSessions = response.sessions || []
            } else {
                const response = await api.listAuditLogs({})
                fetchedSessions = response.logs || []

                if (filter === 'completed') {
                    fetchedSessions = fetchedSessions.filter((s: AuditLog) => s.session_status === 'completed')
                }
            }

            setSessions(fetchedSessions)
        } catch (error) {
            console.error('Failed to fetch sessions:', error)
        } finally {
            setLoadingSessions(false)
        }
    }

    const handleSelectSession = async (session: AuditLog) => {
        setSelectedSession(session)
        setLoadingRecording(true)
    }

    const toggleFullscreen = () => {
        if (!viewerContainerRef.current) return

        if (!isFullscreen) {
            // Enter fullscreen
            if (viewerContainerRef.current.requestFullscreen) {
                viewerContainerRef.current.requestFullscreen()
            }
        } else {
            // Exit fullscreen
            if (document.exitFullscreen) {
                document.exitFullscreen()
            }
        }
    }

    // Listen for fullscreen changes
    useEffect(() => {
        const handleFullscreenChange = () => {
            setIsFullscreen(!!document.fullscreenElement)
        }

        document.addEventListener('fullscreenchange', handleFullscreenChange)
        return () => document.removeEventListener('fullscreenchange', handleFullscreenChange)
    }, [])

    // Handle resizable divider
    const handleMouseDown = () => {
        setIsDragging(true)
    }

    useEffect(() => {
        const handleMouseMove = (e: MouseEvent) => {
            if (!isDragging) return

            // Calculate new width based on mouse position
            const newWidth = e.clientX - 32 // Subtract page padding

            // Set min and max widths
            if (newWidth >= 300 && newWidth <= 800) {
                setLeftPanelWidth(newWidth)
            }
        }

        const handleMouseUp = () => {
            setIsDragging(false)
        }

        if (isDragging) {
            document.addEventListener('mousemove', handleMouseMove)
            document.addEventListener('mouseup', handleMouseUp)
        }

        return () => {
            document.removeEventListener('mousemove', handleMouseMove)
            document.removeEventListener('mouseup', handleMouseUp)
        }
    }, [isDragging])

    // Handle terminal loading when session is selected
    useEffect(() => {
        // Cleanup previous terminal and websocket
        if (xtermRef.current) {
            xtermRef.current.dispose()
            xtermRef.current = null
        }
        if (wsRef.current) {
            wsRef.current.close()
            wsRef.current = null
        }

        // If no session selected, we're done
        if (!selectedSession) {
            setLoadingRecording(false)
            return
        }

        // If RDP, we skip terminal init but still need to load recording
        if (selectedSession.protocol === 'rdp') {
            loadRecording(null as any, selectedSession.id)
            return
        }

        // Wait for terminal ref to be available, then initialize
        const initTerminal = async () => {
            // Wait for the DOM to be ready
            let attempts = 0
            while (!terminalRef.current && attempts < 50) {
                await new Promise(resolve => setTimeout(resolve, 10))
                attempts++
            }

            if (!terminalRef.current) {
                console.error('Terminal ref never became available')
                setLoadingRecording(false)
                return
            }

            try {
                await loadTerminal(selectedSession)
            } catch (error) {
                console.error('Failed to load session:', error)
            } finally {
                setLoadingRecording(false)
            }
        }

        initTerminal()
    }, [selectedSession])

    const loadTerminal = async (session: AuditLog) => {
        console.log('loadTerminal called for session:', session.id, 'status:', session.session_status)
        if (!terminalRef.current) {
            console.error('terminalRef.current is null')
            return
        }

        // Load xterm
        const [{ Terminal }, { FitAddon }] = await Promise.all([
            import('@xterm/xterm'),
            import('@xterm/addon-fit')
        ])

        const term = new Terminal({
            cursorBlink: false,
            disableStdin: true,
            fontSize: 14,
            fontFamily: 'Menlo, Monaco, "Courier New", monospace',
            theme: {
                background: '#1e1e1e',
                foreground: '#d4d4d4',
                cursor: '#d4d4d4',
            },
            convertEol: true,
        })

        const fitAddon = new FitAddon()
        term.loadAddon(fitAddon)
        term.open(terminalRef.current)
        fitAddon.fit()

        xtermRef.current = term
        fitAddonRef.current = fitAddon

        console.log('Terminal initialized and opened')

        // Handle resize - fit terminal to container size
        const handleResize = () => {
            // Use requestAnimationFrame to ensure DOM has updated
            requestAnimationFrame(() => {
                if (fitAddonRef.current) {
                    try {
                        fitAddonRef.current.fit()
                    } catch (e) {
                        console.error('Error fitting terminal:', e)
                    }
                }
            })
        }
        window.addEventListener('resize', handleResize)

        // Also handle fullscreen changes
        document.addEventListener('fullscreenchange', handleResize)

        // Trigger initial resize after a short delay to ensure container is sized
        setTimeout(handleResize, 100)

        const isActive = session.session_status === 'active'

        if (isActive) {
            // For active sessions, load history then connect to live stream
            if (session.protocol !== 'rdp') {
                term.write('\r\n[Loading session history...]\r\n\r\n')
            }
            try {
                // For RDP, we need to load recording content even for active sessions
                // to get the initial state (size, ready, etc.)
                if (session.protocol === 'rdp') {
                    await loadRecording(null as any, session.id)
                } else {
                    await loadRecording(term, session.id)
                }
            } catch (err) {
                console.log('No recording history yet:', err)
            }

            if (session.protocol !== 'rdp') {
                term.write('\r\n\r\n[Connecting to live session...]\r\n\r\n')
                await connectLiveMonitor(term, session.id)
            }
        } else {
            // Load completed recording
            console.log('Loading completed recording for session:', session.id)
            if (session.protocol === 'rdp') {
                await loadRecording(null as any, session.id)
            } else {
                await loadRecording(term, session.id)
            }
        }

        return () => {
            window.removeEventListener('resize', handleResize)
            document.removeEventListener('fullscreenchange', handleResize)
        }
    }

    const connectLiveMonitor = async (term: XTerm, sessionId: string) => {
        const wsUrl = process.env.NEXT_PUBLIC_WS_URL || 'ws://localhost:8080'
        const token = localStorage.getItem('openpam_token')

        const ws = new WebSocket(`${wsUrl}/api/ws/monitor/${sessionId}`)
        wsRef.current = ws

        ws.onopen = () => {
            console.log('Connected to live session monitor')
            if (token) {
                ws.send(JSON.stringify({ type: 'auth', token }))
            }
        }

        ws.onmessage = (event) => {
            if (event.data instanceof Blob) {
                event.data.text().then((text) => {
                    term.write(text)
                })
            } else if (typeof event.data === 'string') {
                term.write(event.data)
            } else if (event.data instanceof ArrayBuffer) {
                const text = new TextDecoder().decode(event.data)
                term.write(text)
            }
        }

        ws.onerror = (error) => {
            console.error('WebSocket error:', error)
            term.write('\r\n\r\n[Connection error]\r\n')
        }

        ws.onclose = () => {
            console.log('Live monitor disconnected')
            term.write('\r\n\r\n[Session ended]\r\n')
        }
    }

    const loadRecording = async (term: XTerm, sessionId: string) => {
        try {
            console.log('Fetching recording for session:', sessionId)
            const data = await api.getRecording(sessionId)
            console.log('Recording data received, length:', data.length)

            // Strip header/footer
            let content = data.replace(/^=== SSH Session Recording ===[\s\S]*?={29}\s+/, '')
            content = content.replace(/\s+={29}\s+End Time:[\s\S]*$/, '')
            content = content.replace(/\x7F/g, '') // Filter DEL characters

            console.log('Processed content length:', content.length)

            if (!content || content.trim().length === 0) {
                content = '[No recording content found]\r\n'
                console.warn('Empty recording content')
            }

            // Write content in chunks
            // Write content in chunks
            if (selectedSession?.protocol === 'rdp') {
                setRecordingContent(content)
            } else {
                if (!term) return

                const chunkSize = 1024
                let offset = 0
                const writeChunk = () => {
                    if (offset < content.length) {
                        const chunk = content.substring(offset, offset + chunkSize)
                        term.write(chunk)
                        offset += chunkSize
                        setTimeout(writeChunk, 0)
                    } else {
                        console.log('Finished writing recording to terminal')
                    }
                }
                writeChunk()
            }
        } catch (err) {
            console.error('Failed to load recording:', err)
            if (term) {
                term.write('[Failed to load recording]\r\n')
                if (err instanceof Error) {
                    term.write(`Error: ${err.message}\r\n`)
                }
            }
        }
    }

    // Helper functions to get human-friendly names
    const getUserName = (userId: string): string => {
        const u = users.get(userId)
        return u ? u.display_name || u.email : userId
    }

    const getTargetName = (targetId: string): string => {
        const t = targets.get(targetId)
        return t ? `${t.name} (${t.hostname})` : targetId
    }

    const getCredentialUsername = (credentialId: string): string => {
        const c = credentials.get(credentialId)
        if (c) {
            return c.username
        }
        // Debug: log when credential not found
        console.log('Credential not found for ID:', credentialId, 'Available credentials:', Array.from(credentials.keys()))
        return 'N/A'
    }

    const getStatusBadge = (status: string) => {
        const statusMap: Record<string, { bg: string, text: string, label: string }> = {
            active: { bg: 'bg-green-100', text: 'text-green-800', label: 'Active' },
            completed: { bg: 'bg-blue-100', text: 'text-blue-800', label: 'Completed' },
            failed: { bg: 'bg-red-100', text: 'text-red-800', label: 'Failed' },
            terminated: { bg: 'bg-yellow-100', text: 'text-yellow-800', label: 'Terminated' }
        }
        const config = statusMap[status] || statusMap.completed
        return (
            <span className={`px-2 py-1 text-xs font-semibold rounded-full ${config.bg} ${config.text}`}>
                {config.label}
            </span>
        )
    }

    // Filter and search sessions
    const filteredSessions = sessions.filter(session => {
        if (!searchTerm) return true

        const searchLower = searchTerm.toLowerCase()
        const userName = getUserName(session.user_id).toLowerCase()
        const targetName = getTargetName(session.target_id).toLowerCase()
        const credUsername = session.credential_id
            ? getCredentialUsername(session.credential_id).toLowerCase()
            : ''

        return userName.includes(searchLower) ||
               targetName.includes(searchLower) ||
               credUsername.includes(searchLower) ||
               session.id.toLowerCase().includes(searchLower)
    })

    if (loading || (!user || (user.role !== 'auditor' && user.role !== 'admin'))) {
        return null
    }

    return (
        <div className="min-h-screen bg-gray-50">
            <Header />
            <div className="w-full px-4 sm:px-6 lg:px-8 py-8">
                {/* Header */}
                <div className="mb-8">
                    <div className="flex justify-between items-center mb-4">
                        <div>
                            <h1 className="text-3xl font-bold text-gray-900">Session Audit</h1>
                            <p className="mt-2 text-gray-600">Monitor and review all sessions with live playback</p>
                        </div>
                        <div className="flex space-x-2">
                            <button
                                onClick={() => setFilter('all')}
                                className={`px-4 py-2 rounded-lg transition-colors ${filter === 'all'
                                    ? 'bg-indigo-600 text-white'
                                    : 'bg-gray-200 text-gray-700'
                                    }`}
                            >
                                All Sessions
                            </button>
                            <button
                                onClick={() => setFilter('active')}
                                className={`px-4 py-2 rounded-lg transition-colors ${filter === 'active'
                                    ? 'bg-indigo-600 text-white'
                                    : 'bg-gray-200 text-gray-700'
                                    }`}
                            >
                                Active ({sessions.filter(s => s.session_status === 'active').length})
                            </button>
                            <button
                                onClick={() => setFilter('completed')}
                                className={`px-4 py-2 rounded-lg transition-colors ${filter === 'completed'
                                    ? 'bg-indigo-600 text-white'
                                    : 'bg-gray-200 text-gray-700'
                                    }`}
                            >
                                Completed
                            </button>
                        </div>
                    </div>

                    {/* Search bar */}
                    <div className="relative">
                        <input
                            type="text"
                            placeholder="Search by user, resource, or account..."
                            value={searchTerm}
                            onChange={(e) => setSearchTerm(e.target.value)}
                            className="w-full px-4 py-2 pl-10 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-indigo-500"
                        />
                        <svg
                            className="absolute left-3 top-2.5 h-5 w-5 text-gray-400"
                            fill="none"
                            stroke="currentColor"
                            viewBox="0 0 24 24"
                        >
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
                        </svg>
                    </div>
                </div>

                <div className="flex gap-0">
                    {/* Sessions List */}
                    <div style={{ width: `${leftPanelWidth}px`, minWidth: '300px', maxWidth: '800px' }}>
                        <div className="bg-white shadow-md rounded-lg overflow-hidden">
                            <div className="px-4 py-3 bg-gray-50 border-b border-gray-200">
                                <h2 className="text-lg font-semibold text-gray-900">Sessions</h2>
                            </div>
                            <div className="divide-y divide-gray-200 max-h-[calc(100vh-300px)] overflow-y-auto">
                                {loadingSessions ? (
                                    <div className="px-4 py-8 text-center text-gray-500">
                                        Loading sessions...
                                    </div>
                                ) : filteredSessions.length === 0 ? (
                                    <div className="px-4 py-8 text-center text-gray-500">
                                        {searchTerm ? 'No sessions match your search' : 'No sessions found'}
                                    </div>
                                ) : (
                                    filteredSessions.map((session) => {
                                        const duration = session.end_time?.Valid && session.end_time.Time
                                            ? Math.round((new Date(session.end_time.Time).getTime() - new Date(session.start_time).getTime()) / 1000 / 60)
                                            : null
                                        const isSelected = selectedSession?.id === session.id
                                        const userName = getUserName(session.user_id)
                                        const targetName = getTargetName(session.target_id)
                                        const accountName = session.credential_id
                                            ? getCredentialUsername(session.credential_id)
                                            : 'N/A'

                                        return (
                                            <div
                                                key={session.id}
                                                onClick={() => handleSelectSession(session)}
                                                className={`px-4 py-3 cursor-pointer hover:bg-gray-50 transition-colors ${isSelected ? 'bg-indigo-50 border-l-4 border-indigo-600' : ''
                                                    }`}
                                            >
                                                <div className="flex items-start justify-between">
                                                    <div className="flex-1 min-w-0">
                                                        <div className="flex items-center gap-2 mb-2">
                                                            {getStatusBadge(session.session_status)}
                                                            <span className="px-2 py-0.5 text-xs font-semibold rounded bg-gray-100 text-gray-800">
                                                                {session.protocol?.toUpperCase() || 'SSH'}
                                                            </span>
                                                        </div>
                                                        <p className="text-sm font-semibold text-gray-900 truncate" title={userName}>
                                                            User: {userName}
                                                        </p>
                                                        <p className="text-xs text-gray-600 truncate mt-1" title={accountName}>
                                                            Account: {accountName}
                                                        </p>
                                                        <p className="text-xs text-gray-600 truncate" title={targetName}>
                                                            Resource: {targetName}
                                                        </p>
                                                        <p className="text-xs text-gray-400 mt-2">
                                                            {new Date(session.start_time).toLocaleString()}
                                                        </p>
                                                        {duration !== null && (
                                                            <p className="text-xs text-gray-400">
                                                                Duration: {duration} min
                                                            </p>
                                                        )}
                                                        {session.session_status === 'active' && (
                                                            <div className="flex items-center gap-1 mt-1">
                                                                <span className="w-2 h-2 bg-red-600 rounded-full animate-pulse"></span>
                                                                <span className="text-xs text-red-600 font-semibold">LIVE</span>
                                                            </div>
                                                        )}
                                                    </div>
                                                </div>
                                            </div>
                                        )
                                    })
                                )}
                            </div>
                        </div>
                    </div>

                    {/* Resizable Divider */}
                    <div
                        className={`w-1 bg-gray-300 hover:bg-indigo-500 cursor-col-resize transition-colors ${isDragging ? 'bg-indigo-500' : ''}`}
                        onMouseDown={handleMouseDown}
                        style={{ userSelect: 'none' }}
                    />

                    {/* Terminal Viewer */}
                    <div className="flex-1 ml-6">
                        <div ref={viewerContainerRef} className={`bg-white shadow-md rounded-lg overflow-hidden ${isFullscreen ? 'fixed inset-0 z-50' : ''}`}>
                            <div className="px-4 py-3 bg-gray-50 border-b border-gray-200 flex justify-between items-center">
                                <div>
                                    <h2 className="text-lg font-semibold text-gray-900">
                                        {selectedSession ? (
                                            <>
                                                {selectedSession.session_status === 'active' ? 'Live Monitor' : 'Session Replay'}
                                                {selectedSession.session_status === 'active' && (
                                                    <span className="ml-3 inline-flex items-center gap-2 px-3 py-1 text-sm font-semibold text-white bg-red-600 rounded-full animate-pulse">
                                                        <span className="w-2 h-2 bg-white rounded-full"></span>
                                                        LIVE
                                                    </span>
                                                )}
                                            </>
                                        ) : 'Select a session to view'}
                                    </h2>
                                    {selectedSession && (
                                        <p className="text-xs text-gray-500 mt-1">
                                            Session ID: {selectedSession.id} | Started: {new Date(selectedSession.start_time).toLocaleString()}
                                        </p>
                                    )}
                                </div>
                                {selectedSession && (
                                    <div className="flex items-center gap-2">
                                        <button
                                            onClick={toggleFullscreen}
                                            className="text-gray-400 hover:text-gray-600"
                                            title={isFullscreen ? 'Exit fullscreen' : 'Enter fullscreen'}
                                        >
                                            {isFullscreen ? (
                                                <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                                                </svg>
                                            ) : (
                                                <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 8V4m0 0h4M4 4l5 5m11-1V4m0 0h-4m4 0l-5 5M4 16v4m0 0h4m-4 0l5-5m11 5l-5-5m5 5v-4m0 4h-4" />
                                                </svg>
                                            )}
                                        </button>
                                        <button
                                            onClick={() => setSelectedSession(null)}
                                            className="text-gray-400 hover:text-gray-600"
                                            title="Close viewer"
                                        >
                                            <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                                            </svg>
                                        </button>
                                    </div>
                                )}
                            </div>

                            {selectedSession ? (
                                <div className={`bg-[#1e1e1e] relative ${isFullscreen ? 'h-[calc(100vh-60px)]' : 'h-[calc(100vh-300px)]'}`}>
                                    {selectedSession.protocol === 'rdp' ? (
                                        <RdpPlayer
                                            sessionId={selectedSession.id}
                                            mode={selectedSession.session_status === 'active' ? 'live' : 'replay'}
                                            recordingData={recordingContent}
                                        />
                                    ) : (
                                        <>
                                            <div ref={terminalRef} className="h-full p-2" />
                                            {loadingRecording && (
                                                <div className="absolute inset-0 flex items-center justify-center bg-[#1e1e1e] bg-opacity-90">
                                                    <div className="text-gray-400">Loading session...</div>
                                                </div>
                                            )}
                                        </>
                                    )}
                                </div>
                            ) : (
                                <div className="flex flex-col items-center justify-center h-[calc(100vh-300px)] text-gray-500">
                                    <svg className="w-16 h-16 mb-4 text-gray-300" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
                                    </svg>
                                    <p className="text-lg font-medium">No session selected</p>
                                    <p className="text-sm mt-1">Select a session from the list to view its recording</p>
                                </div>
                            )}
                        </div>
                    </div>
                </div>
            </div>
        </div>
    )
}
