'use client'

import { useEffect, useRef, useState } from 'react'
import type { Terminal as XTerm } from '@xterm/xterm'
import type { FitAddon } from '@xterm/addon-fit'
import '@xterm/xterm/css/xterm.css'

interface TerminalProps {
  wsUrl: string
  onClose?: () => void
}

export default function TerminalClient({ wsUrl, onClose }: TerminalProps) {
  const terminalRef = useRef<HTMLDivElement>(null)
  const xtermRef = useRef<XTerm | null>(null)
  const wsRef = useRef<WebSocket | null>(null)
  const fitAddonRef = useRef<FitAddon | null>(null)
  const [connectionStatus, setConnectionStatus] = useState<'connecting' | 'connected' | 'disconnected' | 'error'>('connecting')
  const [error, setError] = useState<string>('')

  useEffect(() => {
    if (!terminalRef.current) return

    const isDisposed = { current: false }

    const loadTerminal = async () => {
      try {
        // Dynamically import xterm and addons
        const [
          { Terminal },
          { FitAddon },
          { WebLinksAddon }
        ] = await Promise.all([
          import('@xterm/xterm'),
          import('@xterm/addon-fit'),
          import('@xterm/addon-web-links')
        ])

        if (isDisposed.current) return

        // Initialize xterm.js - use minimal config
        const term = new Terminal({
          cursorBlink: true,
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
        const webLinksAddon = new WebLinksAddon()

        term.loadAddon(fitAddon)
        term.loadAddon(webLinksAddon)

        xtermRef.current = term
        fitAddonRef.current = fitAddon

        // Custom key handler to allow clipboard shortcuts to propagate to the browser
        term.attachCustomKeyEventHandler((e) => {
          // Allow Ctrl+V, Ctrl+Shift+V, Shift+Insert to bubble (triggering paste)
          if (e.type === 'keydown') {
            if ((e.ctrlKey && e.key.toLowerCase() === 'v') ||
              (e.shiftKey && e.key === 'Insert')) {
              return false // Skip xterm processing, allow browser default
            }
          }
          return true // Process other keys
        })

        // Helper to safely fit terminal
        const safeFit = () => {
          if (isDisposed.current) return false
          try {
            if (terminalRef.current && terminalRef.current.offsetHeight > 0 && terminalRef.current.offsetWidth > 0 && term.element) {
              fitAddon.fit()
              return true
            }
          } catch (err) {
            console.warn('Failed to fit terminal:', err)
          }
          return false
        }

        // Open terminal and connect WebSocket
        if (!terminalRef.current || isDisposed.current) return

        term.open(terminalRef.current)

        // Wait for terminal to be fully rendered before fitting
        setTimeout(() => {
          if (isDisposed.current) return
          if (safeFit()) {
            // Setup resize observer only after initial fit succeeds
            const resizeObserver = new ResizeObserver(() => {
              if (isDisposed.current) return
              if (safeFit()) {
                if (wsRef.current?.readyState === WebSocket.OPEN) {
                  const msg = JSON.stringify({
                    type: 'resize',
                    cols: term.cols,
                    rows: term.rows,
                  })
                  wsRef.current.send(msg)
                }
              }
            })

            if (terminalRef.current) {
              resizeObserver.observe(terminalRef.current)
            }
            // @ts-ignore
            term._resizeObserver = resizeObserver
          }
        }, 100)

        // Use xterm.js onData event for all input handling
        // This correctly handles all keys, control sequences, and pastes
        term.onData(data => {
          if (isDisposed.current) return
          if (wsRef.current?.readyState === WebSocket.OPEN) {
            wsRef.current.send(data)
          }
        })

        // Setup Paste Handler
        const handlePaste = (e: ClipboardEvent) => {
          if (isDisposed.current) return
          e.preventDefault()
          e.stopPropagation()

          const text = e.clipboardData?.getData('text')
          if (text && wsRef.current?.readyState === WebSocket.OPEN) {
            wsRef.current.send(text)
          }
        }
        window.addEventListener('paste', handlePaste, true)
        // @ts-ignore
        term._pasteHandler = handlePaste

        // Setup Drag and Drop Handler
        const handleDragOver = (e: DragEvent) => {
          e.preventDefault()
          e.stopPropagation()
          if (e.dataTransfer) {
            e.dataTransfer.dropEffect = 'copy'
          }
        }

        const handleDrop = async (e: DragEvent) => {
          e.preventDefault()
          e.stopPropagation()
          if (isDisposed.current) return

          if (e.dataTransfer && e.dataTransfer.files.length > 0) {
            const file = e.dataTransfer.files[0]
            // Limit file size to 1MB for now to avoid blocking the shell/socket
            if (file.size > 1024 * 1024) {
              term.writeln('\r\n\x1b[31mError: File too large (max 1MB for drag-and-drop)\x1b[0m\r\n')
              return
            }

            term.writeln(`\r\n\x1b[36mUploading ${file.name}...\x1b[0m\r\n`)

            try {
              const buffer = await file.arrayBuffer()
              const bytes = new Uint8Array(buffer)
              let binary = ''
              for (let i = 0; i < bytes.byteLength; i++) {
                binary += String.fromCharCode(bytes[i])
              }
              const base64 = btoa(binary)

              if (wsRef.current?.readyState === WebSocket.OPEN) {
                wsRef.current.send(JSON.stringify({
                  type: 'file_upload',
                  name: file.name,
                  data: base64
                }))
              }
            } catch (err) {
              console.error('File read error:', err)
              term.writeln(`\r\n\x1b[31mError reading file: ${err}\x1b[0m\r\n`)
            }
          }
        }

        window.addEventListener('dragover', handleDragOver)
        window.addEventListener('drop', handleDrop)
        // @ts-ignore
        term._dragOverHandler = handleDragOver
        // @ts-ignore
        term._dropHandler = handleDrop

        // Connect WebSocket
        const ws = new WebSocket(wsUrl)
        wsRef.current = ws

        ws.onopen = () => {
          if (isDisposed.current) {
            ws.close()
            return
          }
          setConnectionStatus('connected')

          // Send initial terminal size
          setTimeout(() => {
            if (isDisposed.current) return
            if (ws.readyState === WebSocket.OPEN) {
              const msg = JSON.stringify({
                type: 'resize',
                cols: term.cols,
                rows: term.rows,
              })
              ws.send(msg)
            }
          }, 100)
        }

        ws.onmessage = (event) => {
          if (isDisposed.current) return
          if (typeof event.data === 'string') {
            try {
              const msg = JSON.parse(event.data)
              if (msg.type === 'error') {
                setError(msg.message)
                setConnectionStatus('error')
                term.writeln(`\r\n\x1b[31mError: ${msg.message}\x1b[0m\r\n`)
              }
            } catch {
              term.write(event.data)
            }
          } else if (event.data instanceof Blob) {
            event.data.arrayBuffer().then(buffer => {
              if (isDisposed.current) return
              const data = new Uint8Array(buffer)
              term.write(data)
            })
          } else {
            const data = new Uint8Array(event.data)
            term.write(data)
          }
        }

        ws.onerror = (error) => {
          if (isDisposed.current) return
          console.error('WebSocket error:', error)
          setConnectionStatus('error')
          setError('Connection error')
        }

        ws.onclose = () => {
          if (isDisposed.current) return
          setConnectionStatus('disconnected')
          term.writeln('\r\n\x1b[33mSession ended. Redirecting to dashboard...\x1b[0m\r\n')
          // Redirect to dashboard after 2 seconds
          setTimeout(() => {
            if (onClose) {
              onClose()
            }
          }, 2000)
        }

      } catch (err) {
        console.error('Failed to load terminal:', err)
        setError('Failed to load terminal')
        setConnectionStatus('error')
      }
    }

    loadTerminal()

    // Cleanup
    return () => {
      isDisposed.current = true
      // @ts-ignore
      if (xtermRef.current?._resizeObserver) {
        // @ts-ignore
        xtermRef.current._resizeObserver.disconnect()
      }
      if (xtermRef.current?._pasteHandler) {
        // @ts-ignore
        window.removeEventListener('paste', xtermRef.current._pasteHandler, true)
      }
      // @ts-ignore
      if (xtermRef.current?._dragOverHandler) {
        // @ts-ignore
        window.removeEventListener('dragover', xtermRef.current._dragOverHandler)
      }
      // @ts-ignore
      if (xtermRef.current?._dropHandler) {
        // @ts-ignore
        window.removeEventListener('drop', xtermRef.current._dropHandler)
      }
      if (wsRef.current?.readyState === WebSocket.OPEN) {
        wsRef.current.close()
      }
      if (xtermRef.current) {
        xtermRef.current.dispose()
      }
    }
  }, [wsUrl, onClose])

  return (
    <div className="flex flex-col h-full bg-[#1e1e1e] relative">
      {/* Recording Indicator - Positioned absolutely in top-right */}
      {connectionStatus === 'connected' && (
        <div className="absolute top-16 right-4 z-50 flex flex-col items-center gap-1">
          <div className="relative">
            {/* Pulsing animation */}
            <div className="absolute inset-0 w-3 h-3 bg-red-500 rounded-full animate-ping opacity-75"></div>
            {/* Solid dot */}
            <div className="relative w-3 h-3 bg-red-500 rounded-full"></div>
          </div>
          <span className="text-xs font-semibold text-red-500 tracking-wide">RECORDING</span>
        </div>
      )}

      <div className="flex items-center justify-between px-4 py-2 bg-[#2d2d2d] border-b border-[#3e3e3e]">
        <div className="flex items-center gap-2">
          <div className={`w-2 h-2 rounded-full ${connectionStatus === 'connected' ? 'bg-green-500' :
            connectionStatus === 'connecting' ? 'bg-yellow-500' :
              connectionStatus === 'error' ? 'bg-red-500' :
                'bg-gray-500'
            }`} />
          <span className="text-sm text-gray-400">
            {connectionStatus === 'connected' ? 'Connected' :
              connectionStatus === 'connecting' ? 'Connecting...' :
                connectionStatus === 'error' ? `Error: ${error}` :
                  'Disconnected'}
          </span>
        </div>
        <button
          onClick={() => {
            // Close WebSocket gracefully before calling onClose
            if (wsRef.current?.readyState === WebSocket.OPEN) {
              wsRef.current.close(1000, 'User closed connection')
            }
            // onClose will be called by ws.onclose handler
          }}
          className="p-1 hover:bg-[#3e3e3e] rounded text-gray-400 hover:text-white transition-colors"
          title="Close Connection"
        >
          <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>
      </div>
      <div ref={terminalRef} className="flex-1 overflow-hidden" />
    </div>
  )
}
