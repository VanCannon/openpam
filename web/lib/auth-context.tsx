'use client'

import { createContext, useContext, useEffect, useState } from 'react'
import { User } from '@/types'
import { api } from './api'

interface AuthContextType {
  user: User | null
  loading: boolean
  login: () => void
  logout: () => Promise<void>
  setToken: (token: string) => void
}

const AuthContext = createContext<AuthContextType | undefined>(undefined)

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [user, setUser] = useState<User | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    checkAuth()
  }, [])

  const checkAuth = async () => {
    try {
      const currentUser = await api.getCurrentUser()
      setUser(currentUser)
    } catch (error) {
      console.error('Auth check failed:', error)
      setUser(null)
    } finally {
      setLoading(false)
    }
  }

  const login = () => {
    api.login()
  }

  const logout = async () => {
    try {
      await api.logout()
      setUser(null)
    } catch (error) {
      console.error('Logout failed:', error)
    }
  }

  const setToken = (token: string) => {
    api.setToken(token)
    checkAuth()
  }

  return (
    <AuthContext.Provider value={{ user, loading, login, logout, setToken }}>
      {children}
    </AuthContext.Provider>
  )
}

export function useAuth() {
  const context = useContext(AuthContext)
  if (context === undefined) {
    throw new Error('useAuth must be used within an AuthProvider')
  }
  return context
}
