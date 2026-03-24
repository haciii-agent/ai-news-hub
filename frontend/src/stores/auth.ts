import { defineStore } from 'pinia'
import { ref } from 'vue'
import { authApi, setJWT, clearJWT, isLoggedIn } from '@/api'
import type { User } from '@/api'

export const useAuthStore = defineStore('auth', () => {
  const currentUser = ref<User | null>(null)
  const loggedIn = ref(false)
  const loading = ref(false)

  function init() {
    loggedIn.value = isLoggedIn()
  }

  async function login(loginVal: string, password: string) {
    const res = await authApi.login(loginVal, password)
    if (res.data?.token?.access_token) {
      setJWT(res.data.token.access_token, res.data.token.expires_in)
      currentUser.value = res.data.user
      loggedIn.value = true
    }
    return res.data
  }

  async function register(username: string, email: string, password: string) {
    return authApi.register(username, email, password)
  }

  async function fetchMe() {
    if (!loggedIn.value) return null
    try {
      const res = await authApi.getMe()
      currentUser.value = res.data?.user || res.data
      return currentUser.value
    } catch {
      logout()
      return null
    }
  }

  function logout() {
    clearJWT()
    currentUser.value = null
    loggedIn.value = false
    authApi.logout().catch(() => {})
  }

  return { currentUser, loggedIn, loading, init, login, register, fetchMe, logout }
})
