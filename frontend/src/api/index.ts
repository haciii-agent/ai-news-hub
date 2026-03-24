import axios, { AxiosInstance, AxiosRequestConfig } from 'axios'

const BASE_URL = '/api/v1'
const JWT_KEY = 'jwt_token'
const JWT_EXPIRY_KEY = 'jwt_expiry'
const ANON_TOKEN_KEY = 'user_token'

// ─── Token helpers ─────────────────────────────────────────────────────────

export function getJWT(): string | null {
  return localStorage.getItem(JWT_KEY)
}

export function setJWT(token: string, expiresIn: number): void {
  localStorage.setItem(JWT_KEY, token)
  localStorage.setItem(JWT_EXPIRY_KEY, String(Date.now() + expiresIn * 1000))
}

export function clearJWT(): void {
  localStorage.removeItem(JWT_KEY)
  localStorage.removeItem(JWT_EXPIRY_KEY)
}

export function isJWTExpired(): boolean {
  const e = localStorage.getItem(JWT_EXPIRY_KEY)
  return !e || Date.now() > parseInt(e, 10)
}

export function isLoggedIn(): boolean {
  return !!getJWT() && !isJWTExpired()
}

export function getAnonToken(): string {
  let t = localStorage.getItem(ANON_TOKEN_KEY)
  if (!t) { t = crypto.randomUUID(); localStorage.setItem(ANON_TOKEN_KEY, t) }
  return t
}

function authHeaders(): Record<string, string> {
  const h: Record<string, string> = { 'Content-Type': 'application/json' }
  if (isLoggedIn()) h['Authorization'] = 'Bearer ' + getJWT()
  else h['X-User-Token'] = getAnonToken()
  return h
}

// ─── Axios instance ─────────────────────────────────────────────────────────

const api: AxiosInstance = axios.create({
  baseURL: BASE_URL,
  timeout: 15000,
  headers: { 'Content-Type': 'application/json' },
})

api.interceptors.request.use((config) => {
  const h = authHeaders()
  // Don't override Content-Type if body is FormData
  if (!(config.data instanceof FormData)) {
    config.headers['Content-Type'] = h['Content-Type']
  }
  if (isLoggedIn()) {
    config.headers['Authorization'] = 'Bearer ' + getJWT()
  } else {
    config.headers['X-User-Token'] = h['X-User-Token']
  }
  return config
})

api.interceptors.response.use(
  (res) => res,
  (err) => {
    if (err.response?.status === 401 && getJWT()) {
      // JWT was sent but server returned 401 → token invalid/expired
      clearJWT()
      window.location.href = '/login'
    }
    return Promise.reject(err)
  }
)

// ─── API response shapes ────────────────────────────────────────────────────

export interface ApiResponse<T = any> {
  error?: boolean
  message?: string
  data?: T
  [key: string]: any
}

export interface Article {
  id: number
  title: string
  summary?: string
  content_html?: string
  source: string
  url?: string
  image_url?: string
  language: string
  category?: string
  categories?: string
  published_at?: string
  collected_at?: string
  ai_summary?: string
  importance_score?: number
  is_bookmarked?: boolean
}

export interface User {
  id: number
  username: string
  email: string
  role: string
  created_at?: string
  profile?: {
    interests?: Record<string, number>
    [key: string]: any
  }
}

// ─── Auth API ─────────────────────────────────────────────────────────────

export const authApi = {
  register: (username: string, email: string, password: string) =>
    api.post('/auth/register', { username, email, password }),

  login: (loginVal: string, password: string) =>
    api.post('/auth/login', { login: loginVal, password }),

  getMe: () => api.get('/auth/me'),

  refresh: () => api.post('/auth/refresh'),

  logout: () => api.post('/auth/logout'),

  checkUsername: (username: string) =>
    api.get('/auth/check-username', { params: { username } }),

  checkEmail: (email: string) =>
    api.get('/auth/check-email', { params: { email } }),
}

// ─── Articles API ─────────────────────────────────────────────────────────

export const articlesApi = {
  list: (params: { page?: number; per_page?: number; category?: string; q?: string; lang?: string } = {}) =>
    api.get('/articles', { params: { per_page: 20, page: 1, ...params } }),

  getContent: (id: number) => api.get(`/articles/${id}/content`),

  like: (id: number) => api.post(`/articles/${id}/like`),

  unlike: (id: number) => api.delete(`/articles/${id}/like`),

  getComments: (id: number, params: { page?: number; per_page?: number } = {}) =>
    api.get(`/articles/${id}/comments`, { params: { per_page: 50, page: 1, ...params } }),

  addComment: (id: number, content: string) =>
    api.post(`/articles/${id}/comments`, { content }),

  deleteComment: (id: number, commentId: number) =>
    api.delete(`/articles/${id}/comments/${commentId}`),
}

// ─── Bookmarks API ─────────────────────────────────────────────────────────

export const bookmarksApi = {
  list: (params: { page?: number; per_page?: number } = {}) =>
    api.get('/bookmarks', { params: { per_page: 20, page: 1, ...params } }),

  add: (articleId: number, title: string, source: string) =>
    api.post('/bookmarks', { article_id: articleId, title, source }),

  remove: (id: number) => api.delete(`/bookmarks/${id}`),
}

// ─── History API ─────────────────────────────────────────────────────────

export const historyApi = {
  list: (params: { page?: number; per_page?: number } = {}) =>
    api.get('/history', { params: { per_page: 20, page: 1, ...params } }),
}

// ─── Recommendations API ──────────────────────────────────────────────────

export const recommendationsApi = {
  list: (params: { page?: number; per_page?: number } = {}) =>
    api.get('/recommendations', { params: { per_page: 20, page: 1, ...params } }),
}

// ─── User API ─────────────────────────────────────────────────────────────

export const userApi = {
  getProfile: () => api.get('/user/profile'),

  updateProfile: (data: any) => api.put('/user/profile', data),

  getStreak: () => api.get('/user/streak'),

  updatePassword: (oldPassword: string, newPassword: string) =>
    api.put('/user/password', { old_password: oldPassword, new_password: newPassword }),
}

// ─── Dashboard API ─────────────────────────────────────────────────────────

export const dashboardApi = {
  getStats: () => api.get('/dashboard/stats'),
  getTrend: (days = 7) => api.get('/dashboard/trend', { params: { days } }),
  getCategories: () => api.get('/dashboard/categories'),
  getSources: () => api.get('/dashboard/sources'),
  getRecentArticles: (limit = 10) => api.get('/dashboard/recent-articles', { params: { limit } }),
  getCollectHistory: (limit = 10) => api.get('/dashboard/collect-history', { params: { limit } }),
}

// ─── Trends API ───────────────────────────────────────────────────────────

export const trendsApi = {
  hot: () => api.get('/trends/hot'),
  timeline: () => api.get('/trends/timeline'),
  storyPitches: () => api.get('/trends/story-pitches'),
  related: (params: { article_id?: number } = {}) => api.get('/trends/related', { params }),
}

// ─── Admin API ────────────────────────────────────────────────────────────

export const adminApi = {
  getUsers: (params: { page?: number; q?: string; per_page?: number } = {}) =>
    api.get('/admin/users', { params: { page: 1, per_page: 20, ...params } }),

  getUser: (id: number) => api.get(`/admin/users/${id}`),

  updateRole: (id: number, role: string) =>
    api.put(`/admin/users/${id}/role`, { role }),

  updateStatus: (id: number, disabled: boolean) =>
    api.put(`/admin/users/${id}/status`, { disabled }),
}

// ─── Categories API ───────────────────────────────────────────────────────

export const categoriesApi = {
  list: () => api.get('/categories'),
}

export default api

// ─── Convenience wrappers for views ───────────────────────────────────────
export const getDashboardStats = () => dashboardApi.getStats().then((r: any) => r.data)
export const getDashboardCategories = () => dashboardApi.getCategories().then((r: any) => r.data ?? [])
export const getDashboardSources = () => dashboardApi.getSources().then((r: any) => r.data?.sources ?? [])
export const getDashboardRecent = (limit = 10) => dashboardApi.getRecentArticles(limit).then((r: any) => r.data?.articles ?? [])
export const getTrendsHot = () => trendsApi.hot().then((r: any) => r.data ?? [])
export const getTrendsTimeline = () => trendsApi.timeline().then((r: any) => r.data ?? [])
export const getTrendsStories = () => trendsApi.storyPitches().then((r: any) => r.data ?? [])
export const getAdminUsers = (extra = '') => {
  const params = extra ? { q: extra.replace('&q=', '') } : {}
  return adminApi.getUsers(params).then((r: any) => r.data?.users ?? r.data ?? r)
}
export const updateUserRole = (id: number, role: string) => adminApi.updateRole(id, role)
export const updateUserStatus = (id: number, disabled: boolean) => adminApi.updateStatus(id, disabled)
