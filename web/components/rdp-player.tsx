import { useEffect, useRef, useState } from 'react'
import Guacamole from 'guacamole-common-js'
import { BinaryWebSocketTunnel } from '../utils/BinaryWebSocketTunnel'

interface RdpPlayerProps {
    sessionId: string
    mode: 'live' | 'replay'
    recordingData?: string
}

export default function RdpPlayer({ sessionId, mode, recordingData }: RdpPlayerProps) {
    const displayRef = useRef<HTMLDivElement>(null)
    const clientRef = useRef<Guacamole.Client | null>(null)
    const tunnelRef = useRef<Guacamole.Tunnel | null>(null)
    const [status, setStatus] = useState<string>('Initializing...')

    // Cleanup function
    const cleanup = () => {
        if (clientRef.current) {
            clientRef.current.disconnect()
            clientRef.current = null
        }
        if (displayRef.current) {
            displayRef.current.innerHTML = ''
        }
    }

    // Scale display to fit container
    const scaleDisplay = () => {
        if (!clientRef.current || !displayRef.current) return

        const display = clientRef.current.getDisplay()
        const displayElement = display.getElement()
        const container = displayRef.current.parentElement

        if (!container) return

        const containerWidth = container.clientWidth
        const containerHeight = container.clientHeight

        if (containerWidth === 0 || containerHeight === 0) return

        const displayWidth = display.getWidth()
        const displayHeight = display.getHeight()

        if (displayWidth === 0 || displayHeight === 0) return

        const scale = Math.min(
            containerWidth / displayWidth,
            containerHeight / displayHeight
        )

        display.scale(scale)
    }

    useEffect(() => {
        window.addEventListener('resize', scaleDisplay)
        return () => window.removeEventListener('resize', scaleDisplay)
    }, [])

    useEffect(() => {
        cleanup()

        if (!displayRef.current) return

        // Initialize Guacamole client
        const tunnel = new Guacamole.Tunnel()

        // Mock connect
        tunnel.connect = function (data) {
            this.state = Guacamole.Tunnel.State.OPEN
            if (this.onstatechange)
                this.onstatechange(this.state)
        }

        // Mock sendMessage
        tunnel.sendMessage = function (opcode, args) {
            // Handle outgoing messages if needed
        }

        if (mode === 'live') {
            const wsUrl = process.env.NEXT_PUBLIC_WS_URL || 'ws://localhost:8080'
            const monitorUrl = `${wsUrl}/api/ws/monitor/${sessionId}`

            const ws = new WebSocket(monitorUrl)
            ws.binaryType = 'arraybuffer'

            ws.onopen = () => {
                setStatus('Connected to live stream')
                const token = localStorage.getItem('openpam_token')
                if (token) {
                    ws.send(JSON.stringify({ type: 'auth', token }))
                }
            }

            ws.onmessage = (event) => {
                if (event.data instanceof ArrayBuffer) {
                    const text = new TextDecoder().decode(event.data)
                    parser.receive(text)
                } else if (typeof event.data === 'string') {
                    parser.receive(event.data)
                }
            }

            ws.onclose = () => {
                setStatus('Connection closed')
            }

            ws.onerror = (err) => {
                console.error('RdpPlayer: WebSocket error:', err)
                setStatus('Connection error')
            }

        } else {
            setStatus('Loading recording...')
        }

        const client = new Guacamole.Client(tunnel)
        clientRef.current = client

        // Add display to DOM
        const display = client.getDisplay().getElement()
        displayRef.current.appendChild(display)

        // Error handling
        client.onerror = (status) => {
            console.error('Guacamole client error:', status)
            setStatus(`Error: ${status.message}`)
        }

        client.connect()

        // Parser for incoming data
        const parser = new Guacamole.Parser()

        parser.oninstruction = (opcode, args) => {
            if (tunnel.oninstruction) {
                tunnel.oninstruction(opcode, args)
            }
            // Trigger scaling on size change
            if (opcode === 'size') {
                requestAnimationFrame(scaleDisplay)
            }
        }

        if (recordingData) {
            if (mode === 'live') {
                const lines = recordingData.split('\n')
                for (const rawLine of lines) {
                    const line = rawLine.trim()
                    if (!line) continue

                    const firstComma = line.indexOf(',')
                    if (firstComma === -1) continue

                    const instruction = line.substring(firstComma + 1)
                    if (/^\d/.test(instruction)) {
                        parser.receive(instruction)
                    } else {
                        const parts = instruction.replace(/;$/, '').split(',')
                        if (parts.length >= 1) {
                            const opcode = parts[0]
                            const args = parts.slice(1)
                            if (tunnel.oninstruction) {
                                tunnel.oninstruction(opcode, args)
                            }
                        }
                    }
                }
                // Initial scale
                requestAnimationFrame(scaleDisplay)
            } else {
                playRecording(recordingData, parser, setStatus)
            }
        }

        return cleanup
    }, [sessionId, mode, recordingData])

    const playRecording = async (
        data: string,
        parser: Guacamole.Parser,
        setStatus: (s: string) => void
    ) => {
        const lines = data.split('\n')
        const frames: {
            timestamp: number,
            instruction?: string,
            type: 'standard' | 'csv',
            opcode?: string,
            args?: string[]
        }[] = []

        for (const rawLine of lines) {
            const line = rawLine.trim()
            if (!line) continue

            const firstComma = line.indexOf(',')
            if (firstComma === -1) continue

            const timestamp = parseInt(line.substring(0, firstComma))
            const instruction = line.substring(firstComma + 1)

            if (/^\d/.test(instruction)) {
                frames.push({ timestamp, instruction, type: 'standard' })
            } else {
                const parts = instruction.replace(/;$/, '').split(',')
                if (parts.length >= 1) {
                    frames.push({
                        timestamp,
                        instruction: '',
                        type: 'csv',
                        opcode: parts[0],
                        args: parts.slice(1)
                    })
                }
            }
        }

        if (frames.length === 0) {
            setStatus('Empty recording')
            return
        }

        setStatus('Playing...')

        const startTime = Date.now()
        let frameIndex = 0

        const playFrame = () => {
            if (frameIndex >= frames.length) {
                setStatus('Playback finished')
                return
            }

            const frame = frames[frameIndex]
            const currentTime = Date.now() - startTime

            if (currentTime >= frame.timestamp) {
                if (frame.type === 'standard' && frame.instruction) {
                    parser.receive(frame.instruction)
                } else if (frame.type === 'csv' && frame.opcode) {
                    if (tunnelRef.current && tunnelRef.current.oninstruction) {
                        tunnelRef.current.oninstruction(frame.opcode, frame.args || [])
                    }
                }

                // Scale on size instruction
                if (frame.opcode === 'size' || (frame.instruction && frame.instruction.startsWith('4.size'))) {
                    requestAnimationFrame(scaleDisplay)
                }

                frameIndex++

                if (frameIndex < frames.length && (Date.now() - startTime) >= frames[frameIndex].timestamp) {
                    playFrame()
                } else {
                    requestAnimationFrame(playFrame)
                }
            } else {
                requestAnimationFrame(playFrame)
            }
        }

        playFrame()
    }

    return (
        <div className="relative w-full h-full bg-black flex items-center justify-center overflow-hidden">
            <div
                ref={displayRef}
                className="relative z-10"
                style={{ transformOrigin: 'center center' }}
            >
                <style>{`
                    canvas {
                        z-index: 10 !important;
                    }
                `}</style>
            </div>
            <div className="absolute top-2 right-2 bg-black bg-opacity-50 text-white px-2 py-1 rounded text-xs">
                {status}
            </div>
        </div >
    )
}
