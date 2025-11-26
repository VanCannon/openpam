'use client'

import { useEffect, useRef, useState } from 'react'
// @ts-ignore - guacamole-common-js doesn't have TypeScript types
import Guacamole from 'guacamole-common-js'

interface RdpViewerProps {
  wsUrl: string
  onClose?: () => void
}

export default function RdpViewer({ wsUrl, onClose }: RdpViewerProps) {
  const displayRef = useRef<HTMLDivElement>(null)
  const clientRef = useRef<any>(null)
  const [connectionStatus, setConnectionStatus] = useState<'connecting' | 'connected' | 'disconnected' | 'error'>('connecting')
  const [error, setError] = useState<string>('')

  useEffect(() => {
    if (!displayRef.current) return

    try {
      // Create WebSocket tunnel
      const tunnel = new Guacamole.WebSocketTunnel(wsUrl)

      // Create Guacamole client
      const client = new Guacamole.Client(tunnel)
      clientRef.current = client

      // Get display element
      const display = client.getDisplay()

      // Add display to DOM
      displayRef.current.innerHTML = ''
      displayRef.current.appendChild(display.getElement())

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
            if (onClose) onClose()
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

      // Mouse handling
      const mouse = new Guacamole.Mouse(display.getElement())

      mouse.onmousedown =
      mouse.onmouseup =
      mouse.onmousemove = (mouseState: any) => {
        client.sendMouseState(mouseState)
      }

      // Keyboard handling
      const keyboard = new Guacamole.Keyboard(document)

      keyboard.onkeydown = (keysym: number) => {
        client.sendKeyEvent(1, keysym)
      }

      keyboard.onkeyup = (keysym: number) => {
        client.sendKeyEvent(0, keysym)
      }

      // Connect
      client.connect()

      // Handle window resize
      const handleResize = () => {
        if (displayRef.current) {
          const width = displayRef.current.clientWidth
          const height = displayRef.current.clientHeight
          client.sendSize(width, height)
        }
      }

      window.addEventListener('resize', handleResize)
      // Send initial size after a short delay to ensure connection is established
      setTimeout(handleResize, 100)

      // Cleanup
      return () => {
        window.removeEventListener('resize', handleResize)
        if (keyboard) {
          keyboard.onkeydown = null
          keyboard.onkeyup = null
        }
        if (mouse) {
          mouse.onmousedown = null
          mouse.onmouseup = null
          mouse.onmousemove = null
        }
        if (client) {
          client.disconnect()
        }
      }
    } catch (err) {
      console.error('Failed to initialize Guacamole client:', err)
      setError('Failed to initialize RDP client')
      setConnectionStatus('error')
    }
  }, [wsUrl, onClose])

  return (
    <div className="flex flex-col h-full bg-gray-900">
      <div className="flex items-center justify-between px-4 py-2 bg-gray-800 border-b border-gray-700">
        <div className="flex items-center space-x-2">
          <div className={`w-3 h-3 rounded-full ${
            connectionStatus === 'connected' ? 'bg-green-500' :
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
      <div
        ref={displayRef}
        className="flex-1 relative bg-black"
      >
        {connectionStatus === 'connecting' && (
          <div className="absolute inset-0 flex items-center justify-center text-gray-500">
            <p>Connecting to remote desktop...</p>
          </div>
        )}
        {connectionStatus === 'error' && (
          <div className="absolute inset-0 flex items-center justify-center text-red-500">
            <p>Connection error: {error}</p>
          </div>
        )}
      </div>
    </div>
  )
}
