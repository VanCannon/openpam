'use client'

import { useAuth } from '@/lib/auth-context'
import { api } from '@/lib/api'
import { useRouter, useParams } from 'next/navigation'
import { useEffect, useRef, useState } from 'react'
import Link from 'next/link'
import type { Terminal as XTerm } from '@xterm/xterm'
import type { FitAddon } from '@xterm/addon-fit'
import type { AuditLog } from '@/types'
import '@xterm/xterm/css/xterm.css'
import Header from '@/components/header'

export default function SessionPlayerPage() {
    const { user, loading } = useAuth()
    const router = useRouter()
    const params = useParams()
    const id = params.id as string
    const terminalRef = useRef<HTMLDivElement>(null)
    const xtermRef = useRef<XTerm | null>(null)
    const wsRef = useRef<WebSocket | null>(null)
    const initializedRef = useRef(false)
    const [loadingSession, setLoadingSession] = useState(true)
    const [error, setError] = useState('')
    const [isLive, setIsLive] = useState(false)
    const [session, setSession] = useState<AuditLog | null>(null)

    useEffect(() => {
        if (!loading && !user) {
            router.push('/login')
        }
    }, [user, loading, router])

    useEffect(() => {
        if (!user || !id) return
        if (initializedRef.current) return
        initializedRef.current = true

        const loadSession = async () => {
            try {
                setLoadingSession(true)

                // Fetch session info to check if it's active
                const auditLog = await api.getAuditLog(id)
                setSession(auditLog)

                const isActive = auditLog.session_status === 'active'
                setIsLive(isActive)

                // Load xterm
                const [{ Terminal }, { FitAddon }] = await Promise.all([
                    import('@xterm/xterm'),
                    import('@xterm/addon-fit')
                ])

                if (!terminalRef.current) return

                const term = new Terminal({
                    cursorBlink: false,
                    disableStdin: true, // Read-only
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

                // Handle resize
                window.addEventListener('resize', () => fitAddon.fit())

                if (isActive) {
                    // For active sessions, first load the history, then connect to live stream
                    try {
                        await loadRecording(term, id)
                    } catch (err) {
                        // If no recording exists yet (session just started), that's okay
                        console.log('No recording history yet, starting fresh')
                    }
                    // Now connect to live monitoring (backend will write audit message to recording)
                    await connectLiveMonitor(term, id)
                } else {
                    // Load recording file
                    await loadRecording(term, id)
                }

            } catch (err) {
                console.error('Failed to load session:', err)
                setError('Failed to load session. It might not exist or is corrupted.')
            } finally {
                setLoadingSession(false)
            }
        }

        loadSession()

        return () => {
            if (xtermRef.current) {
                xtermRef.current.dispose()
            }
            if (wsRef.current) {
                wsRef.current.close()
            }
        }
    }, [user, id])

    const connectLiveMonitor = async (term: XTerm, sessionId: string) => {
        const wsUrl = process.env.NEXT_PUBLIC_WS_URL || 'ws://localhost:8080'
        const token = localStorage.getItem('token')

        const ws = new WebSocket(`${wsUrl}/api/ws/monitor/${sessionId}`)
        wsRef.current = ws

        ws.onopen = () => {
            console.log('Connected to live session monitor')
            // Send auth token
            if (token) {
                ws.send(JSON.stringify({ type: 'auth', token }))
            }
        }

        ws.onmessage = (event) => {
            if (event.data instanceof Blob) {
                // Convert blob to text
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
            setError('Connection error. The session may have ended.')
        }

        ws.onclose = () => {
            console.log('Live monitor disconnected')
            setIsLive(false)
            term.write('\r\n\r\n[Session ended - Switching to replay mode...]\r\n')

            // Reload as recording after a short delay
            setTimeout(async () => {
                try {
                    await loadRecording(term, sessionId)
                } catch (err) {
                    console.error('Failed to load recording:', err)
                }
            }, 1000)
        }
    }

    const loadRecording = async (term: XTerm, sessionId: string) => {
        try {
            const data = await api.getRecording(sessionId)

            // Parse data (strip header/footer)
            let content = data

            // Strip header using regex
            content = content.replace(/^=== SSH Session Recording ===[\s\S]*?={29}\s+/, '')

            // Strip footer
            content = content.replace(/\s+={29}\s+End Time:[\s\S]*$/, '')

            if (!content) {
                content = 'No recording content found or empty session.'
            }

            // Filter out DEL characters (ASCII 127)
            const beforeLength = content.length
            content = content.replace(/\x7F/g, '')
            const afterLength = content.length
            console.log(`Filtered ${beforeLength - afterLength} DEL characters from recording`)

            // Write content in chunks
            const chunkSize = 1024
            let offset = 0
            const writeChunk = () => {
                if (offset < content.length) {
                    const chunk = content.substring(offset, offset + chunkSize)
                    term.write(chunk)
                    offset += chunkSize
                    setTimeout(writeChunk, 0)
                }
            }
            writeChunk()

        } catch (err) {
            console.error('Failed to load recording:', err)
            setError('Failed to load recording.')
        }
    }

    if (loading || !user) {
        return <div className="flex min-h-screen items-center justify-center"><p>Loading...</p></div>
    }

    return (
        <div className="min-h-screen bg-gray-50 flex flex-col">
            <Header />

            <main className="flex-1 max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8 w-full">
                <div className="mb-6 flex items-center justify-between">
                    <div>
                        <h1 className="text-2xl font-bold text-gray-900 flex items-center gap-3">
                            {isLive ? 'Live Session Monitor' : 'Session Replay'}
                            {isLive && (
                                <span className="inline-flex items-center gap-2 px-3 py-1 text-sm font-semibold text-white bg-red-600 rounded-full animate-pulse">
                                    <span className="w-2 h-2 bg-white rounded-full"></span>
                                    LIVE
                                </span>
                            )}
                        </h1>
                        <p className="text-sm text-gray-600 mt-1">
                            Session ID: {params.id}
                            {session && (
                                <span className="ml-4">
                                    Started: {new Date(session.start_time).toLocaleString()}
                                </span>
                            )}
                        </p>
                    </div>
                </div>

                {error ? (
                    <div className="bg-red-50 border border-red-200 text-red-700 px-4 py-3 rounded">
                        {error}
                    </div>
                ) : (
                    <div className="bg-[#1e1e1e] rounded-lg shadow-lg overflow-hidden h-[600px] flex flex-col">
                        <div className="bg-[#2d2d2d] px-4 py-2 border-b border-[#3e3e3e] flex justify-between items-center">
                            <span className="text-gray-400 text-sm">
                                {isLive ? 'Monitoring Active Session' : 'Terminal Replay'}
                            </span>
                            {loadingSession && <span className="text-yellow-500 text-sm">Loading...</span>}
                        </div>
                        <div ref={terminalRef} className="flex-1 p-2" />
                    </div>
                )}
            </main>
        </div>
    )
}
