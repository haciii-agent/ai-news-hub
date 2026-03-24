import { defineStore } from 'pinia'
import { ref } from 'vue'

const THEME_KEY = 'theme'

export const useAppStore = defineStore('app', () => {
  const theme = ref<'light' | 'dark'>((localStorage.getItem(THEME_KEY) as 'light' | 'dark') || 'light')
  const loading = ref(false)

  function toggleTheme() {
    theme.value = theme.value === 'light' ? 'dark' : 'light'
    localStorage.setItem(THEME_KEY, theme.value)
    document.documentElement.setAttribute('data-theme', theme.value)
  }

  function initTheme() {
    document.documentElement.setAttribute('data-theme', theme.value)
  }

  return { theme, loading, toggleTheme, initTheme }
})
