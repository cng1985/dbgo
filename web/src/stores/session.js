import { defineStore } from 'pinia'
import { api } from '../api'

export const useSession = defineStore('session', {
  state: () => ({
    profile: null,
    ready: false
  }),
  actions: {
    async load() {
      try {
        const result = await api('/api/auth/profile', {})
        this.profile = result.data
      } catch {
        this.profile = null
      } finally {
        this.ready = true
      }
    },
    async login(username, password) {
      const result = await api('/api/auth/login', { username, password })
      this.profile = result.data
      this.ready = true
    },
    async logout() {
      try {
        await api('/api/auth/logout', {})
      } finally {
        this.profile = null
      }
    }
  }
})
