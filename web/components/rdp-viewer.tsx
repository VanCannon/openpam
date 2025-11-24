'use client'

import { useEffect, useRef, useState } from 'react'

interface RdpViewerProps {
  wsUrl: string
  onClose?: () => void
}

export default function RdpViewer({ wsUrl, onClose }: RdpViewerProps) {
  const displayRef = useRef<HTMLDivElement>(null)
  const wsRef = useRef<WebSocket | null>(null)
  const [connectionStatus, setConnectionStatus] = useState<'connecting' | 'connected' | 'disconnected' | 'error'>('connecting')
  const [error, setError] = useState<string>('')

  useEffect(() => {
    if (!displayRef.current) return

    // Connect to WebSocket for RDP
    const ws = new WebSocket(wsUrl)
    wsRef.current = ws

    ws.onopen = () => {
      setConnectionStatus('connected')
    }

    ws.onmessage = (event) => {
      if (typeof event.data === 'string') {
        try {
          const msg = JSON.parse(event.data)
          if (msg.type === 'error') {
            setError(msg.message)
            setConnectionStatus('error')
          }
        } catch {
          // Handle Guacamole protocol messages
          handleGuacamoleMessage(event.data)
        }
      }
    }

    ws.onerror = (error) => {
      console.error('WebSocket error:', error)
      setConnectionStatus('error')
      setError('Connection error')
    }

    ws.onclose = () => {
      setConnectionStatus('disconnected')
      if (onClose) {
        onClose()
      }
    }

    // Cleanup
    return () => {
      if (ws.readyState === WebSocket.OPEN) {
        ws.close()
      }
    }
  }, [wsUrl, onClose])

  const handleGuacamoleMessage = (data: string) => {
    // This is a simplified handler
    // In production, you would use the full Guacamole JavaScript client
    // For now, we'll display a message that RDP is connected
    console.log('Guacamole data:', data)
  }

  const sendMouseEvent = (event: React.MouseEvent) => {
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      const rect = displayRef.current?.getBoundingClientRect()
      if (!rect) return

      const x = event.clientX - rect.left
      const y = event.clientY - rect.top

      // Send Guacamole mouse event
      const msg = `5.mouse,${Math.floor(x)},${Math.floor(y)},${event.buttons};`
      wsRef.current.send(msg)
    }
  }

  const sendKeyEvent = (event: React.KeyboardEvent) => {
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      // Send Guacamole key event
      const pressed = event.type === 'keydown' ? 1 : 0
      const msg = `3.key,${event.keyCode},${pressed};`
      wsRef.current.send(msg)
    }
  }

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
        className="flex-1 relative bg-black cursor-crosshair"
        onMouseMove={sendMouseEvent}
        onMouseDown={sendMouseEvent}
        onMouseUp={sendMouseEvent}
        onKeyDown={sendKeyEvent}
        onKeyUp={sendKeyEvent}
        tabIndex={0}
      >
        {connectionStatus === 'connected' && (
          <div className="absolute inset-0 flex items-center justify-center text-gray-500">
            <div className="text-center">
              <p className="text-xl mb-2">RDP Session Active</p>
              <p className="text-sm">Click to interact with remote desktop</p>
            </div>
          </div>
        )}
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
