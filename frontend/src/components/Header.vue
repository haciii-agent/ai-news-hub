<template>
  <header class="app-header">
    <div class="header-left">
      <router-link to="/" class="header-logo">
        <span class="logo-icon">📰</span>
        <span>AI News <span class="logo-accent">Hub</span></span>
      </router-link>
    </div>
    <div class="header-right">
      <router-link to="/recommendations" class="nav-icon" title="推荐">🎯</router-link>
      <router-link to="/dashboard" class="nav-icon" title="数据看板">📊</router-link>
      <router-link to="/trends" class="nav-icon" title="趋势分析">📈</router-link>
      <router-link to="/bookmarks" class="nav-icon" title="收藏">📌</router-link>
      <router-link to="/history" class="nav-icon" title="阅读历史">📖</router-link>

      <!-- Auth -->
      <template v-if="auth.loggedIn">
        <n-dropdown :options="userMenuOptions" @select="handleUserMenu">
          <button class="user-avatar-btn">{{ auth.currentUser?.username?.[0]?.toUpperCase() || '👤' }}</button>
        </n-dropdown>
      </template>
      <template v-else>
        <router-link to="/login" class="nav-link">登录</router-link>
        <router-link to="/login?tab=register" class="nav-link">注册</router-link>
      </template>

      <n-button quaternary circle @click="appStore.toggleTheme()" class="theme-btn">
        {{ appStore.theme === 'dark' ? '☀️' : '🌙' }}
      </n-button>
    </div>
  </header>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import { NDropdown, NButton } from 'naive-ui'
import { useAuthStore } from '@/stores/auth'
import { useAppStore } from '@/stores/app'

const auth = useAuthStore()
const appStore = useAppStore()
const router = useRouter()

const userMenuOptions = computed(() => [
  { label: '👤 个人中心', key: 'profile' },
  ...(auth.currentUser?.role === 'admin' ? [{ label: '🔧 管理后台', key: 'admin' }] : []),
  { label: '🚪 退出登录', key: 'logout' },
])

function handleUserMenu(key: string) {
  if (key === 'logout') {
    auth.logout()
    router.push('/')
  } else if (key === 'profile') {
    router.push('/profile')
  } else if (key === 'admin') {
    router.push('/admin')
  }
}
</script>

<style scoped>
.app-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  height: 60px;
  padding: 0 24px;
  background: var(--surface);
  border-bottom: 1px solid var(--border);
  position: sticky;
  top: 0;
  z-index: 100;
  box-shadow: var(--shadow);
}

.header-left, .header-right {
  display: flex;
  align-items: center;
  gap: 8px;
}

.header-logo {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 18px;
  font-weight: 700;
  color: var(--text-primary);
  text-decoration: none;
}
.header-logo:hover { color: var(--accent); }

.logo-accent { color: var(--accent); }

.nav-icon {
  font-size: 18px;
  padding: 4px 8px;
  border-radius: var(--radius-sm);
  transition: background 0.2s;
}
.nav-icon:hover { background: var(--border); }

.nav-link {
  font-size: 14px;
  padding: 4px 10px;
  color: var(--text-secondary);
  border-radius: var(--radius-sm);
  transition: color 0.2s;
}
.nav-link:hover { color: var(--accent); }

.user-avatar-btn {
  width: 32px;
  height: 32px;
  border-radius: 50%;
  background: var(--accent);
  color: #fff;
  border: none;
  cursor: pointer;
  font-size: 14px;
  font-weight: 600;
  transition: opacity 0.2s;
}
.user-avatar-btn:hover { opacity: 0.85; }

.theme-btn {
  font-size: 18px;
  border: none;
  background: transparent;
  cursor: pointer;
  padding: 4px;
}
</style>
