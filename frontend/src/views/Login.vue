<template>
  <main class="login-page">
    <div class="auth-card">
      <n-tabs type="line" :value="tab" @update:value="tab = $event">
        <n-tab name="login" tab="登录" />
        <n-tab name="register" tab="注册" />
      </n-tabs>

      <!-- Login Form -->
      <n-form v-if="tab === 'login'" @submit.prevent="handleLogin" class="auth-form">
        <n-form-item label="用户名或邮箱">
          <n-input v-model:value="loginVal" autocomplete="username" placeholder="输入用户名或邮箱" />
        </n-form-item>
        <n-form-item label="密码">
          <n-input v-model:value="loginPwd" type="password" show-password-on="mousedown" autocomplete="current-password" placeholder="输入密码" />
        </n-form-item>
        <n-alert v-if="loginError" type="error" :title="loginError" style="margin-bottom: 12px" />
        <n-button type="primary" attr-type="submit" :loading="loading" block>登录</n-button>
      </n-form>

      <!-- Register Form -->
      <n-form v-else @submit.prevent="handleRegister" class="auth-form">
        <n-form-item label="用户名">
          <n-input v-model:value="regUsername" placeholder="3-20字符，字母/数字/下划线/中文" @input="checkUsername" maxlength="20" />
          <div class="form-hint" :class="usernameHint.class">{{ usernameHint.text }}</div>
        </n-form-item>
        <n-form-item label="邮箱">
          <n-input v-model:value="regEmail" placeholder="your@email.com" @blur="checkEmail" />
          <div class="form-hint" :class="emailHint.class">{{ emailHint.text }}</div>
        </n-form-item>
        <n-form-item label="密码">
          <n-input v-model:value="regPwd" type="password" show-password-on="mousedown" placeholder="至少8位，含大小写字母和数字" @input="updateStrength" />
          <n-progress
            v-if="strength.score > 0"
            type="line"
            :percentage="strength.w"
            :color="strength.color"
            :show-indicator="false"
            style="margin-top: 4px"
          />
          <div class="form-hint" :class="strength.class">{{ strength.text }}</div>
        </n-form-item>
        <n-form-item label="确认密码">
          <n-input v-model:value="regPwd2" type="password" show-password-on="mousedown" placeholder="再次输入密码" @input="checkConfirm" />
          <div class="form-hint" :class="confirmHint.class">{{ confirmHint.text }}</div>
        </n-form-item>
        <n-alert v-if="regError" type="error" :title="regError" style="margin-bottom: 12px" />
        <n-button type="primary" attr-type="submit" :loading="loading" block>注册</n-button>
      </n-form>
    </div>
  </main>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useMessage } from 'naive-ui'
import { useAuthStore } from '@/stores/auth'
import { authApi, setJWT } from '@/api'

const router = useRouter()
const route = useRoute()
const message = useMessage()
const auth = useAuthStore()

const tab = ref((route.query.tab as string) === 'register' ? 'register' : 'login')
const loading = ref(false)

// Login
const loginVal = ref('')
const loginPwd = ref('')
const loginError = ref('')

async function handleLogin() {
  loginError.value = ''
  loading.value = true
  try {
    const res = await authApi.login(loginVal.value, loginPwd.value)
    const data = res.data
    if (data?.error) { loginError.value = data.message || '登录失败'; return }
    if (data?.token?.access_token) {
      setJWT(data.token.access_token, data.token.expires_in)
      auth.currentUser = data.user
      auth.loggedIn = true
      message.success('登录成功')
      const redirect = localStorage.getItem('login_redirect') || '/'
      localStorage.removeItem('login_redirect')
      router.push(redirect)
    }
  } finally { loading.value = false }
}

// Register
const regUsername = ref('')
const regEmail = ref('')
const regPwd = ref('')
const regPwd2 = ref('')
const regError = ref('')
const usernameHint = reactive({ text: '', class: '' })
const emailHint = reactive({ text: '', class: '' })
const confirmHint = reactive({ text: '', class: '' })
const strength = reactive({ score: 0, w: 0, color: '', text: '', class: '' })

let usernameTimer: any = null
function checkUsername() {
  clearTimeout(usernameTimer)
  const val = regUsername.value.trim()
  if (!val) { usernameHint.text = ''; return }
  if (!/^[\w\u4e00-\u9fff]{3,20}$/.test(val)) {
    usernameHint.text = '3-20字符，仅字母/数字/下划线/中文'; usernameHint.class = 'error'; return
  }
  usernameHint.text = '检查中...'; usernameHint.class = ''
  usernameTimer = setTimeout(async () => {
    try {
      const res = await authApi.checkUsername(val)
      if (res.data?.available) { usernameHint.text = '✓ 用户名可用'; usernameHint.class = 'success' }
      else { usernameHint.text = '✗ 用户名已被占用'; usernameHint.class = 'error' }
    } catch { usernameHint.text = '' }
  }, 500)
}

function checkEmail() {
  const val = regEmail.value.trim()
  if (!val) { emailHint.text = ''; return }
  if (!/^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$/.test(val)) {
    emailHint.text = '邮箱格式不正确'; emailHint.class = 'error'; return
  }
  emailHint.text = '检查中...'; emailHint.class = ''
  authApi.checkEmail(val).then(res => {
    if (res.data?.available) { emailHint.text = '✓ 邮箱可用'; emailHint.class = 'success' }
    else { emailHint.text = '✗ 邮箱已注册'; emailHint.class = 'error' }
  }).catch(() => { emailHint.text = '' })
}

function updateStrength() {
  const val = regPwd.value
  if (!val) { strength.score = 0; strength.w = 0; strength.text = ''; return }
  let score = 0
  if (val.length >= 8) score++
  if (val.length >= 12) score++
  if (/[a-z]/.test(val) && /[A-Z]/.test(val)) score++
  if (/\d/.test(val)) score++
  if (/[^a-zA-Z0-9]/.test(val)) score++
  const levels = ['', '弱', '较弱', '中', '较强', '强']
  const colors = ['', '#f44336', '#ff9800', '#ff9800', '#4caf50', '#4caf50']
  const widths = [0, 20, 40, 60, 80, 100]
  strength.score = score; strength.w = widths[score]; strength.color = colors[score]
  strength.text = levels[score]
  strength.class = score <= 1 ? 'error' : score <= 2 ? '' : 'success'
  checkConfirm()
}

function checkConfirm() {
  if (!regPwd2.value) { confirmHint.text = ''; return }
  if (regPwd.value === regPwd2.value) { confirmHint.text = '✓ 密码一致'; confirmHint.class = 'success' }
  else { confirmHint.text = '✗ 密码不一致'; confirmHint.class = 'error' }
}

async function handleRegister() {
  regError.value = ''
  const username = regUsername.value.trim()
  const email = regEmail.value.trim()
  const password = regPwd.value
  if (!/^[\w\u4e00-\u9fff]{3,20}$/.test(username)) { regError.value = '用户名格式不正确'; return }
  if (!/^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$/.test(email)) { regError.value = '邮箱格式不正确'; return }
  if (password.length < 8 || !/[a-z]/.test(password) || !/[A-Z]/.test(password) || !/\d/.test(password)) {
    regError.value = '密码至少8位，需含大小写字母和数字'; return
  }
  if (password !== regPwd2.value) { regError.value = '两次密码不一致'; return }
  loading.value = true
  try {
    const res = await authApi.register(username, email, password)
    if (res.data?.error) { regError.value = res.data.message || '注册失败' }
    else {
      message.success('注册成功，请登录')
      tab.value = 'login'
      loginVal.value = username
    }
  } finally { loading.value = false }
}

onMounted(() => {
  if (auth.loggedIn) router.push('/')
})
</script>

<style scoped>
.login-page {
  display: flex;
  justify-content: center;
  padding: 40px 16px;
  min-height: calc(100vh - 120px);
}

.auth-card {
  width: 100%;
  max-width: 420px;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  padding: 24px;
}

.auth-form {
  margin-top: 20px;
}

.form-hint {
  font-size: 12px;
  margin-top: 4px;
  color: var(--text-secondary);
}
.form-hint.success { color: #4caf50; }
.form-hint.error { color: #f44336; }
</style>
