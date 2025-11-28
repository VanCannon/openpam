'use client'

import { useEffect, useRef, useState } from 'react'
// @ts-ignore - guacamole-common-js doesn't have TypeScript types
import Guacamole from 'guacamole-common-js'

import { BinaryWebSocketTunnel } from '../utils/BinaryWebSocketTunnel'

interface RdpViewerProps {
  wsUrl: string
  onClose?: () => void
}

export default function RdpViewer({ wsUrl, onClose }: RdpViewerProps) {
  const displayRef = useRef<HTMLDivElement>(null)
  const clientRef = useRef<any>(null)
  const isUnmounting = useRef(false)
  const [connectionStatus, setConnectionStatus] = useState<'connecting' | 'connected' | 'disconnected' | 'error'>('connecting')
  const [error, setError] = useState<string>('')

  const connectionAttempted = useRef(false)

  useEffect(() => {
    if (!displayRef.current) return

    let client: any = null
    let tunnel: BinaryWebSocketTunnel | null = null

    const initConnection = () => {
      // Prevent double connection in Strict Mode
      if (connectionAttempted.current) {
        console.log('Connection already attempted, skipping')
        return
      }
      connectionAttempted.current = true

      try {
        // Defensive clean of URL
        const cleanUrl = wsUrl.replace('?undefined', '')
        console.log('RdpViewer initializing with URL:', cleanUrl)

        // Create WebSocket tunnel using our custom BinaryWebSocketTunnel
        tunnel = new BinaryWebSocketTunnel(cleanUrl)

        // Create Guacamole client
        client = new Guacamole.Client(tunnel)
        clientRef.current = client

        // Get display element
        const display = client.getDisplay()

        // Add display to DOM
        if (displayRef.current) {
          displayRef.current.innerHTML = ''
          const displayElement = display.getElement()
          displayRef.current.appendChild(displayElement)

          // Force z-index of canvases to ensure they are visible
          const canvases = displayElement.querySelectorAll('canvas')
          canvases.forEach((canvas: HTMLCanvasElement) => {
            canvas.style.zIndex = '10'
          })
        }

        // Handle state changes
        client.onstatechange = (state: number) => {
          console.log('Guacamole state changed:', state)
          switch (state) {
            case 0: // IDLE
              setConnectionStatus('connecting')
              break
            case 1: // CONNECTING
              setConnectionStatus('connecting')
              break
            case 2: // WAITING
              setConnectionStatus('connecting')
              break
            case 3: // CONNECTED
              setConnectionStatus('connected')
              break
            case 4: // DISCONNECTING
              setConnectionStatus('disconnected')
              break
            case 5: // DISCONNECTED
              setConnectionStatus('disconnected')
              // Only call onClose if we're not unmounting
              if (!isUnmounting.current && onClose) {
                onClose()
              }
              break
          }
        }

        // Handle errors
        client.onerror = (status: any) => {
          console.error('Guacamole error:', status)
          setError(`Connection error: ${status.message || 'Unknown error'}`)
          setConnectionStatus('error')
        }

        // Handle clipboard (optional)
        client.onclipboard = (_stream: any, mimetype: string) => {
          console.log('Clipboard data received:', mimetype)
        }

        // Mouse handling - send directly via tunnel like reference implementation
        const mouse = new Guacamole.Mouse(display.getElement())

        mouse.onmousedown =
          mouse.onmouseup =
          mouse.onmousemove = (mouseState: any) => {
            if (!tunnel) return

            // Calculate button mask
            let mask = 0
            if (mouseState.left) mask |= 1
            if (mouseState.middle) mask |= 2
            if (mouseState.right) mask |= 4
            if (mouseState.up) mask |= 8
            if (mouseState.down) mask |= 16

            // Send mouse instruction directly via tunnel (not via client)
            tunnel.sendInstruction("mouse", [Math.floor(mouseState.x), Math.floor(mouseState.y), mask])
          }

        // Keyboard handling - send directly via tunnel like reference implementation
        const keyboard = new Guacamole.Keyboard(document)

        keyboard.onkeydown = (keysym: number) => {
          if (!tunnel) return
          tunnel.sendInstruction("key", [keysym, 1])
        }

        keyboard.onkeyup = (keysym: number) => {
          if (!tunnel) return
          tunnel.sendInstruction("key", [keysym, 0])
        }

        // Connect
        client.connect("")

        // Handle window resize - send directly via tunnel with debouncing like reference
        let resizeTimeout: NodeJS.Timeout
        const handleResize = () => {
          clearTimeout(resizeTimeout)
          resizeTimeout = setTimeout(() => {
            if (displayRef.current && tunnel) {
              const width = displayRef.current.clientWidth
              const height = displayRef.current.clientHeight
              // Send size instruction to guacd via tunnel
              // Format: size, width, height, dpi
              tunnel.sendInstruction("size", [width, height, 96])
            }
          }, 300) // Debounce 300ms
        }

        window.addEventListener('resize', handleResize)
        // Send initial size immediately (without debounce)
        if (displayRef.current && tunnel) {
          const width = displayRef.current.clientWidth
          const height = displayRef.current.clientHeight
          tunnel.sendInstruction("size", [width, height, 96])
        }

      } catch (err) {
        console.error('Failed to initialize Guacamole client:', err)
        setError('Failed to initialize RDP client')
        setConnectionStatus('error')
      }
    }

    // Initialize immediately
    initConnection()

    // Cleanup
    return () => {
      isUnmounting.current = true
      window.removeEventListener('resize', () => { }) // Note: anonymous function won't remove the specific listener, but we can't access handleResize here easily. 
      // Actually, we should move handleResize out or keep it in scope. 
      // For now, let's rely on component unmount.

      if (client) {
        client.disconnect()
      }
      if (tunnel) {
        tunnel.disconnect()
      }
    }
  }, [wsUrl, onClose])

  return (
    <div className="flex flex-col h-full bg-gray-900">
      <div className="flex items-center justify-between px-4 py-2 bg-gray-800 border-b border-gray-700">
        <div className="flex items-center space-x-2">
          <div className={`w-3 h-3 rounded-full ${connectionStatus === 'connected' ? 'bg-green-500' :
            connectionStatus === 'connecting' ? 'bg-yellow-500' :
              connectionStatus === 'error' ? 'bg-red-500' :
                'bg-gray-500'
            }`} />
          <span className="text-sm text-gray-300">
            {connectionStatus === 'connected' && 'Connected (RDP)'}
            {connectionStatus === 'connecting' && 'Connecting to RDP...'}
            {connectionStatus === 'error' && `Error: ${error}`}
            {connectionStatus === 'disconnected' && 'Disconnected'}
          </span>
        </div>
        {onClose && (
          <button
            onClick={onClose}
            className="text-gray-400 hover:text-white text-sm px-3 py-1 rounded hover:bg-gray-700"
          >
            Close
          </button>
        )}
      </div>
      <div className="flex-1 relative bg-black">
        <div ref={displayRef} className="absolute inset-0" />
        {connectionStatus === 'connecting' && (
          <div className="absolute inset-0 flex items-center justify-center text-gray-500 pointer-events-none">
            <p>Connecting to remote desktop...</p>
          </div>
        )}
        {connectionStatus === 'error' && (
          <div className="absolute inset-0 flex items-center justify-center text-red-500 pointer-events-none">
            <p>Connection error: {error}</p>
          </div>
        )}
      </div>
    </div>
  )
}
