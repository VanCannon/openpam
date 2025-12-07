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

    // Playback controls
    const [isPlaying, setIsPlaying] = useState(false)
    const [currentTime, setCurrentTime] = useState(0)
    const [duration, setDuration] = useState(0)
    const [playbackSpeed, setPlaybackSpeed] = useState(1)
    const [isSeeking, setIsSeeking] = useState(false) // Track if user is dragging slider
    const animationFrameRef = useRef<number | null>(null)
    const framesRef = useRef<any[]>([])
    const frameIndexRef = useRef(0)
    const startTimeRef = useRef(0)
    const pausedAtRef = useRef(0)
    const isPlayingRef = useRef(false) // Ref for immediate state checks in animation loop
    const seekTargetRef = useRef<number | null>(null) // Target time during seek drag
    const playbackSpeedRef = useRef(1) // Track previous playback speed for smooth speed changes

    // Cleanup function
    const cleanup = () => {
        // Stop any ongoing playback
        isPlayingRef.current = false
        setIsPlaying(false)
        if (animationFrameRef.current) {
            cancelAnimationFrame(animationFrameRef.current)
            animationFrameRef.current = null
        }

        if (clientRef.current) {
            clientRef.current.disconnect()
            clientRef.current = null
        }
        if (displayRef.current) {
            displayRef.current.innerHTML = ''
        }
        tunnelRef.current = null
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

    // Handle playback speed changes during playback
    useEffect(() => {
        if (isPlayingRef.current) {
            // Calculate current video position using the OLD speed
            const currentVideoTime = (Date.now() - startTimeRef.current) * playbackSpeedRef.current
            // Adjust startTimeRef so that with the NEW speed, we'll be at the same position
            startTimeRef.current = Date.now() - (currentVideoTime / playbackSpeed)
        }
        // Update the ref to track the current speed for next change
        playbackSpeedRef.current = playbackSpeed
    }, [playbackSpeed])

    useEffect(() => {
        cleanup()

        if (!displayRef.current) return

        // Initialize Guacamole client
        const tunnel = new Guacamole.Tunnel()
        tunnelRef.current = tunnel // Store tunnel reference for playback functions

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

    // Play/pause toggle
    const togglePlayPause = () => {
        if (isPlaying) {
            pause()
        } else {
            play()
        }
    }

    // Play
    const play = () => {
        if (framesRef.current.length === 0) return
        setIsPlaying(true)
        isPlayingRef.current = true

        if (frameIndexRef.current >= framesRef.current.length) {
            // Restart from beginning
            frameIndexRef.current = 0
            setCurrentTime(0)
            pausedAtRef.current = 0
        }

        // If starting from beginning, replay initial frames to show something immediately
        if (frameIndexRef.current === 0 && pausedAtRef.current === 0) {
            replayUpToFrame(0)
        }

        // pausedAtRef stores VIDEO time, convert to wall-clock time for current speed
        startTimeRef.current = Date.now() - (pausedAtRef.current / playbackSpeedRef.current)
        playFrames()
    }

    // Pause
    const pause = () => {
        setIsPlaying(false)
        isPlayingRef.current = false
        if (animationFrameRef.current) {
            cancelAnimationFrame(animationFrameRef.current)
            animationFrameRef.current = null
        }
        // Store VIDEO time, not wall-clock time
        pausedAtRef.current = (Date.now() - startTimeRef.current) * playbackSpeedRef.current
    }

    // Start seeking (user grabbed the slider)
    const handleSeekStart = () => {
        setIsSeeking(true)
        // Pause playback during seek
        if (isPlayingRef.current) {
            if (animationFrameRef.current) {
                cancelAnimationFrame(animationFrameRef.current)
                animationFrameRef.current = null
            }
        }
    }

    // During seeking (user is dragging the slider)
    const handleSeekChange = (timeMs: number) => {
        // Just update the visual time, don't replay frames yet
        seekTargetRef.current = timeMs
        setCurrentTime(timeMs)
    }

    // End seeking (user released the slider)
    const handleSeekEnd = () => {
        setIsSeeking(false)
        const timeMs = seekTargetRef.current
        if (timeMs !== null) {
            seekTo(timeMs)
            seekTargetRef.current = null
        }
    }

    // Seek to specific time (in milliseconds) - performs actual seek with frame replay
    const seekTo = (timeMs: number) => {
        const frames = framesRef.current
        if (frames.length === 0) return

        // Find the frame index closest to the requested time
        let targetIndex = 0
        for (let i = 0; i < frames.length; i++) {
            if (frames[i].timestamp <= timeMs) {
                targetIndex = i
            } else {
                break
            }
        }

        frameIndexRef.current = targetIndex
        pausedAtRef.current = timeMs
        setCurrentTime(timeMs)

        // Replay all frames up to this point to rebuild the display state
        // Note: This will cause a visible "fast forward" effect, but it's necessary
        // because Guacamole's protocol is stateful and we must replay all frames
        // to get the display into the correct state for the target time
        replayUpToFrame(targetIndex)

        if (isPlayingRef.current) {
            startTimeRef.current = Date.now() - (timeMs / playbackSpeedRef.current)
            playFrames()
        }
    }

    // Replay frames up to a specific index to rebuild display state
    const replayUpToFrame = (targetIndex: number) => {
        const frames = framesRef.current
        const parser = new Guacamole.Parser()

        parser.oninstruction = (opcode: any, args: any) => {
            if (tunnelRef.current && tunnelRef.current.oninstruction) {
                tunnelRef.current.oninstruction(opcode, args)
            }
            if (opcode === 'size') {
                requestAnimationFrame(scaleDisplay)
            }
        }

        for (let i = 0; i <= targetIndex && i < frames.length; i++) {
            const frame = frames[i]
            if (frame.type === 'standard' && frame.instruction) {
                parser.receive(frame.instruction)
            } else if (frame.type === 'csv' && frame.opcode) {
                if (tunnelRef.current && tunnelRef.current.oninstruction) {
                    tunnelRef.current.oninstruction(frame.opcode, frame.args || [])
                }
            }
        }
    }

    // Play frames with animation
    const playFrames = () => {
        const frames = framesRef.current

        const playFrame = () => {
            // Check ref instead of state for immediate updates
            if (!isPlayingRef.current || frameIndexRef.current >= frames.length) {
                if (frameIndexRef.current >= frames.length) {
                    setStatus('Playback finished')
                    setIsPlaying(false)
                    isPlayingRef.current = false
                    // Set current time to duration to ensure progress bar is at 100%
                    setCurrentTime(duration)
                }
                return
            }

            const elapsed = (Date.now() - startTimeRef.current) * playbackSpeedRef.current

            // Update current time smoothly every frame, capped at duration
            const smoothTime = Math.min(elapsed, duration)
            setCurrentTime(smoothTime)

            // Render all frames that should have been displayed by now
            while (frameIndexRef.current < frames.length &&
                   frames[frameIndexRef.current].timestamp <= elapsed) {
                const currentFrame = frames[frameIndexRef.current]

                const parser = new Guacamole.Parser()

                parser.oninstruction = (opcode: any, args: any) => {
                    if (tunnelRef.current && tunnelRef.current.oninstruction) {
                        tunnelRef.current.oninstruction(opcode, args)
                    }
                }

                if (currentFrame.type === 'standard' && currentFrame.instruction) {
                    parser.receive(currentFrame.instruction)
                } else if (currentFrame.type === 'csv' && currentFrame.opcode) {
                    if (tunnelRef.current && tunnelRef.current.oninstruction) {
                        tunnelRef.current.oninstruction(currentFrame.opcode, currentFrame.args || [])
                    }
                }

                // Scale on size instruction
                if (currentFrame.opcode === 'size' || (currentFrame.instruction && currentFrame.instruction.startsWith('4.size'))) {
                    requestAnimationFrame(scaleDisplay)
                }

                frameIndexRef.current++
            }

            animationFrameRef.current = requestAnimationFrame(playFrame)
        }

        playFrame()
    }

    // Format milliseconds to MM:SS
    const formatTime = (ms: number) => {
        const totalSeconds = Math.floor(ms / 1000)
        const minutes = Math.floor(totalSeconds / 60)
        const seconds = totalSeconds % 60
        return `${minutes}:${seconds.toString().padStart(2, '0')}`
    }

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

        // Store frames and duration
        framesRef.current = frames

        // Find the maximum timestamp (don't assume last frame has highest timestamp)
        const maxTimestamp = Math.max(...frames.map(f => f.timestamp))
        setDuration(maxTimestamp)
        setStatus('Ready to play')
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

            {/* Player controls - only show for replay mode */}
            {mode === 'replay' && duration > 0 && (
                <div className="absolute bottom-0 left-0 right-0 bg-gradient-to-t from-black via-black/90 to-transparent p-4 z-20">
                    <div className="max-w-4xl mx-auto space-y-2">
                        {/* Progress bar */}
                        <div className="flex items-center space-x-2">
                            <span className="text-white text-xs font-mono min-w-[45px]">
                                {formatTime(currentTime)}
                            </span>
                            <input
                                type="range"
                                min="0"
                                max={duration}
                                value={currentTime}
                                onMouseDown={handleSeekStart}
                                onTouchStart={handleSeekStart}
                                onChange={(e) => handleSeekChange(parseInt(e.target.value))}
                                onMouseUp={handleSeekEnd}
                                onTouchEnd={handleSeekEnd}
                                className="flex-1 h-2 bg-gray-700 rounded-lg appearance-none cursor-pointer slider"
                                style={{
                                    background: `linear-gradient(to right, #3b82f6 0%, #3b82f6 ${(currentTime / duration) * 100}%, #374151 ${(currentTime / duration) * 100}%, #374151 100%)`
                                }}
                            />
                            <span className="text-white text-xs font-mono min-w-[45px]">
                                {formatTime(duration)}
                            </span>
                        </div>

                        {/* Control buttons */}
                        <div className="flex items-center justify-center space-x-4">
                            {/* Play/Pause button */}
                            <button
                                onClick={togglePlayPause}
                                className="bg-blue-600 hover:bg-blue-700 text-white p-3 rounded-full transition-colors"
                                title={isPlaying ? 'Pause' : 'Play'}
                            >
                                {isPlaying ? (
                                    <svg className="w-5 h-5" fill="currentColor" viewBox="0 0 20 20">
                                        <path fillRule="evenodd" d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zM7 8a1 1 0 012 0v4a1 1 0 11-2 0V8zm5-1a1 1 0 00-1 1v4a1 1 0 102 0V8a1 1 0 00-1-1z" clipRule="evenodd" />
                                    </svg>
                                ) : (
                                    <svg className="w-5 h-5" fill="currentColor" viewBox="0 0 20 20">
                                        <path fillRule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM9.555 7.168A1 1 0 008 8v4a1 1 0 001.555.832l3-2a1 1 0 000-1.664l-3-2z" clipRule="evenodd" />
                                    </svg>
                                )}
                            </button>

                            {/* Speed control buttons */}
                            <div className="flex items-center space-x-1">
                                <span className="text-gray-400 text-xs mr-2">Speed:</span>
                                {[0.5, 1, 1.5, 2].map((speed) => (
                                    <button
                                        key={speed}
                                        onClick={() => setPlaybackSpeed(speed)}
                                        className={`px-3 py-1 rounded text-xs font-medium transition-colors ${
                                            playbackSpeed === speed
                                                ? 'bg-blue-600 text-white'
                                                : 'bg-gray-700 text-gray-300 hover:bg-gray-600'
                                        }`}
                                    >
                                        {speed}x
                                    </button>
                                ))}
                            </div>
                        </div>
                    </div>
                </div>
            )}
        </div >
    )
}
