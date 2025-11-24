'use client'

import { useEffect, useRef, useState } from 'react'
import { Terminal as XTerm } from 'xterm'
import { FitAddon } from 'xterm-addon-fit'
import { WebLinksAddon } from 'xterm-addon-web-links'
import 'xterm/css/xterm.css'

interface TerminalProps {
  wsUrl: string
  onClose?: () => void
}

export default function Terminal({ wsUrl, onClose }: TerminalProps) {
  const terminalRef = useRef<HTMLDivElement>(null)
  const xtermRef = useRef<XTerm | null>(null)
  const wsRef = useRef<WebSocket | null>(null)
  const fitAddonRef = useRef<FitAddon | null>(null)
  const [connectionStatus, setConnectionStatus] = useState<'connecting' | 'connected' | 'disconnected' | 'error'>('connecting')
  const [error, setError] = useState<string>('')

  useEffect(() => {
    if (!terminalRef.current) return

    // Initialize xterm.js
    const term = new XTerm({
      cursorBlink: true,
      fontSize: 14,
      fontFamily: 'Menlo, Monaco, "Courier New", monospace',
      theme: {
        background: '#1e1e1e',
        foreground: '#d4d4d4',
        cursor: '#d4d4d4',
      },
      rows: 40,
      cols: 80,
    })

    const fitAddon = new FitAddon()
    const webLinksAddon = new WebLinksAddon()

    term.loadAddon(fitAddon)
    term.loadAddon(webLinksAddon)

    term.open(terminalRef.current)
    fitAddon.fit()

    xtermRef.current = term
    fitAddonRef.current = fitAddon

    // Handle window resize
    const handleResize = () => {
      fitAddon.fit()
      if (wsRef.current?.readyState === WebSocket.OPEN) {
        // Send terminal size to backend
        const msg = JSON.stringify({
          type: 'resize',
          cols: term.cols,
          rows: term.rows,
        })
        wsRef.current.send(msg)
      }
    }

    window.addEventListener('resize', handleResize)

    // Connect to WebSocket
    const ws = new WebSocket(wsUrl)
    wsRef.current = ws

    ws.onopen = () => {
      setConnectionStatus('connected')
      term.writeln('Connected to target...\r\n')
    }

    ws.onmessage = (event) => {
      if (typeof event.data === 'string') {
        // Handle control messages
        try {
          const msg = JSON.parse(event.data)
          if (msg.type === 'error') {
            setError(msg.message)
            setConnectionStatus('error')
            term.writeln(`\r\n\x1b[31mError: ${msg.message}\x1b[0m\r\n`)
          }
        } catch {
          // Not JSON, treat as terminal data
          term.write(event.data)
        }
      } else {
        // Binary data from terminal
        term.write(new Uint8Array(event.data))
      }
    }

    ws.onerror = (error) => {
      console.error('WebSocket error:', error)
      setConnectionStatus('error')
      setError('Connection error')
      term.writeln('\r\n\x1b[31mConnection error\x1b[0m\r\n')
    }

    ws.onclose = () => {
      setConnectionStatus('disconnected')
      term.writeln('\r\n\x1b[33mConnection closed\x1b[0m\r\n')
      if (onClose) {
        onClose()
      }
    }

    // Handle terminal input
    term.onData((data) => {
      if (ws.readyState === WebSocket.OPEN) {
        ws.send(data)
      }
    })

    // Cleanup
    return () => {
      window.removeEventListener('resize', handleResize)
      if (ws.readyState === WebSocket.OPEN) {
        ws.close()
      }
      term.dispose()
    }
  }, [wsUrl, onClose])

  return (
    <div className="flex flex-col h-full bg-[#1e1e1e]">
      <div className="flex items-center justify-between px-4 py-2 bg-gray-800 border-b border-gray-700">
        <div className="flex items-center space-x-2">
          <div className={`w-3 h-3 rounded-full ${
            connectionStatus === 'connected' ? 'bg-green-500' :
            connectionStatus === 'connecting' ? 'bg-yellow-500' :
            connectionStatus === 'error' ? 'bg-red-500' :
            'bg-gray-500'
          }`} />
          <span className="text-sm text-gray-300">
            {connectionStatus === 'connected' && 'Connected'}
            {connectionStatus === 'connecting' && 'Connecting...'}
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
      <div ref={terminalRef} className="flex-1" />
    </div>
  )
}
