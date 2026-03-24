<template>
  <n-config-provider :theme="isDark ? darkTheme : undefined" :theme-overrides="themeOverrides">
    <n-message-provider>
      <n-dialog-provider>
        <n-notification-provider>
          <div class="app-root" :data-theme="isDark ? 'dark' : 'light'">
            <AppHeader />
            <router-view />
            <AppFooter />
          </div>
        </n-notification-provider>
      </n-dialog-provider>
    </n-message-provider>
  </n-config-provider>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { darkTheme } from 'naive-ui'
import { useAppStore } from './stores/app'
import AppHeader from './components/Header.vue'
import AppFooter from './components/Footer.vue'

const appStore = useAppStore()
const isDark = computed(() => appStore.theme === 'dark')

const themeOverrides = {
  common: {
    primaryColor: '#e94560',
    primaryColorHover: '#ff5a75',
    fontFamily: "-apple-system, BlinkMacSystemFont, 'Segoe UI', 'Noto Sans SC', 'PingFang SC', 'Hiragino Sans GB', 'Microsoft YaHei', sans-serif",
  }
}
</script>

<style>
*, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }

:root {
  --bg: #f5f5f7;
  --surface: #ffffff;
  --text-primary: #1d1d1f;
  --text-secondary: #6e6e73;
  --border: #d2d2d7;
  --accent: #e94560;
  --accent-hover: #ff5a75;
  --radius: 10px;
  --radius-sm: 6px;
  --shadow: 0 2px 8px rgba(0,0,0,0.1);
  --shadow-hover: 0 6px 20px rgba(0,0,0,0.15);
  --font-sans: -apple-system, BlinkMacSystemFont, 'Segoe UI', 'Noto Sans SC', 'PingFang SC', 'Hiragino Sans GB', 'Microsoft YaHei', sans-serif;
  --max-width: 900px;
  --header-height: 60px;
}

[data-theme="dark"] {
  --bg: #0f0f0f;
  --surface: #1a1a2e;
  --text-primary: #eaeaea;
  --text-secondary: #a0a0b0;
  --border: #2a2a3e;
  --accent: #e94560;
  --accent-hover: #ff5a75;
  --shadow: 0 2px 8px rgba(0,0,0,0.3);
  --shadow-hover: 0 6px 20px rgba(0,0,0,0.5);
}

body {
  background: var(--bg);
  color: var(--text-primary);
  font-family: var(--font-sans);
  line-height: 1.6;
  min-height: 100vh;
}

.app-root {
  display: flex;
  flex-direction: column;
  min-height: 100vh;
}

a { color: var(--accent); text-decoration: none; }
a:hover { color: var(--accent-hover); }
</style>
