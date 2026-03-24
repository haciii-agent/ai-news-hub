<template>
  <main class="profile-page">
    <n-spin :show="loading">
      <div v-if="profile" class="profile-card">
        <div class="profile-header">
          <div class="avatar">{{ profile.username?.[0]?.toUpperCase() }}</div>
          <div class="profile-info">
            <h2>{{ profile.username }}</h2>
            <n-tag :type="roleType">{{ roleLabel }}</n-tag>
            <p class="profile-email">{{ profile.email }}</p>
            <p class="profile-date">注册于 {{ formatDate(profile.created_at) }}</p>
          </div>
        </div>

        <!-- Stats -->
        <div class="profile-stats">
          <div class="stat-item">
            <span class="stat-value">{{ streak?.current_streak || 0 }}</span>
            <span class="stat-label">连续阅读天数</span>
          </div>
          <div class="stat-item">
            <span class="stat-value">{{ streak?.longest_streak || 0 }}</span>
            <span class="stat-label">最长连续天数</span>
          </div>
        </div>

        <!-- Interest Tags -->
        <div v-if="interestTags.length" class="interest-section">
          <h3>兴趣标签</h3>
          <div class="interest-tags">
            <n-tag v-for="{ name, weight } in interestTags" :key="name" size="small">
              {{ name }} ({{ Math.round((weight as number) * 100) }}%)
            </n-tag>
          </div>
        </div>

        <!-- Change Password -->
        <div class="pwd-section">
          <h3>修改密码</h3>
          <n-form @submit.prevent="handleChangePwd" label-placement="top">
            <n-form-item label="原密码">
              <n-input v-model:value="oldPwd" type="password" placeholder="输入原密码" show-password-on="mousedown" />
            </n-form-item>
            <n-form-item label="新密码">
              <n-input v-model:value="newPwd" type="password" placeholder="至少8位，含大小写字母和数字" show-password-on="mousedown" />
            </n-form-item>
            <n-form-item label="确认新密码">
              <n-input v-model:value="newPwd2" type="password" placeholder="再次输入新密码" show-password-on="mousedown" />
            </n-form-item>
            <n-alert v-if="pwdError" type="error" :title="pwdError" style="margin-bottom: 12px" />
            <n-alert v-if="pwdSuccess" type="success" :title="pwdSuccess" style="margin-bottom: 12px" />
            <n-button type="primary" attr-type="submit" :loading="changingPwd">修改密码</n-button>
          </n-form>
        </div>

        <n-button type="error" @click="handleLogout" block style="margin-top: 20px">🚪 退出登录</n-button>
      </div>
    </n-spin>
  </main>
</template>

<script setup lang="ts">
import { ref, computed, reactive, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useMessage } from 'naive-ui'
import { useAuthStore } from '@/stores/auth'
import { userApi } from '@/api'

const router = useRouter()
const message = useMessage()
const auth = useAuthStore()

const loading = ref(true)
const profile = ref<any>(null)
const streak = ref<any>(null)
const oldPwd = ref('')
const newPwd = ref('')
const newPwd2 = ref('')
const pwdError = ref('')
const pwdSuccess = ref('')
const changingPwd = ref(false)

const interestTags = computed(() => {
  if (!profile.value?.profile?.interests) return []
  return Object.entries(profile.value.profile.interests)
    .sort(([, a]: any, [, b]: any) => b - a)
    .slice(0, 20)
    .map(([name, weight]) => ({ name, weight }))
})

const roleLabel = computed(() => {
  const map: Record<string, string> = { admin: '管理员', editor: '编辑', viewer: '普通用户', anonymous: '匿名' }
  return map[profile.value?.role || ''] || profile.value?.role || ''
})

const roleType = computed(() => {
  const map: Record<string, any> = { admin: 'error', editor: 'warning', viewer: 'default' }
  return map[profile.value?.role || ''] || 'default'
})

function formatDate(d?: string) {
  if (!d) return '—'
  return new Date(d).toLocaleString('zh-CN')
}

async function handleChangePwd() {
  pwdError.value = ''
  pwdSuccess.value = ''
  if (newPwd.value.length < 8 || !/[a-z]/.test(newPwd.value) || !/[A-Z]/.test(newPwd.value) || !/\d/.test(newPwd.value)) {
    pwdError.value = '新密码至少8位，需含大小写字母和数字'; return
  }
  if (newPwd.value !== newPwd2.value) { pwdError.value = '两次密码不一致'; return }
  changingPwd.value = true
  try {
    const res = await userApi.updatePassword(oldPwd.value, newPwd.value)
    if (res.data?.error) { pwdError.value = res.data.message || '修改失败' }
    else { pwdSuccess.value = '密码修改成功'; oldPwd.value = ''; newPwd.value = ''; newPwd2.value = '' }
  } catch { pwdError.value = '请求失败' }
  finally { changingPwd.value = false }
}

async function handleLogout() {
  auth.logout()
  message.success('已退出登录')
  router.push('/')
}

onMounted(async () => {
  try {
    const [profileRes, streakRes] = await Promise.all([
      userApi.getProfile(),
      userApi.getStreak().catch(() => null),
    ])
    profile.value = profileRes.data?.user || profileRes.data
    streak.value = streakRes?.data
  } catch {}
  loading.value = false
})
</script>

<style scoped>
.profile-page {
  max-width: var(--max-width);
  margin: 0 auto;
  padding: 20px 16px;
}

.profile-card {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  padding: 24px;
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.profile-header {
  display: flex;
  gap: 16px;
  align-items: center;
}

.avatar {
  width: 64px;
  height: 64px;
  border-radius: 50%;
  background: var(--accent);
  color: #fff;
  font-size: 28px;
  font-weight: 700;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.profile-info h2 { margin-bottom: 4px; }
.profile-email, .profile-date { font-size: 13px; color: var(--text-secondary); margin-top: 4px; }

.profile-stats {
  display: flex;
  gap: 24px;
  padding: 16px;
  background: var(--bg);
  border-radius: var(--radius-sm);
}

.stat-item {
  display: flex;
  flex-direction: column;
  align-items: center;
}

.stat-value {
  font-size: 24px;
  font-weight: 700;
  color: var(--accent);
}

.stat-label {
  font-size: 12px;
  color: var(--text-secondary);
}

.interest-section h3, .pwd-section h3 {
  margin-bottom: 12px;
  font-size: 15px;
}

.interest-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}
</style>
