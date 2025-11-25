'use client'

import dynamic from 'next/dynamic'

// Dynamically import the terminal component with SSR disabled
// This ensures xterm.js only runs on the client side, avoiding React 19 SSR issues
const TerminalClient = dynamic(
  () => import('./terminal-client'),
  {
    ssr: false,
    loading: () => (
      <div className="flex items-center justify-center h-full bg-[#1e1e1e]">
        <div className="text-gray-400">Loading terminal...</div>
      </div>
    )
  }
)

interface TerminalProps {
  wsUrl: string
  onClose?: () => void
}

export default function Terminal(props: TerminalProps) {
  return <TerminalClient {...props} />
}
