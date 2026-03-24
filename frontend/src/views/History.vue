<template>
  <main class="history-page">
    <h1 class="page-title">📖 阅读历史</h1>

    <n-spin :show="loading && !articles.length">
      <div v-if="!auth.loggedIn" class="empty-state">
        <p>登录后可查看阅读历史</p>
        <router-link to="/login" class="n-button n-button--primary">去登录</router-link>
      </div>
      <div v-else-if="!loading && !articles.length" class="empty-state">
        <p>暂无阅读历史</p>
        <router-link to="/" class="n-button n-button--primary">去首页看看</router-link>
      </div>

      <div v-else class="article-list">
        <div v-for="article in articles" :key="article.id" class="article-card">
          <div class="article-meta">
            <span class="article-source">{{ article.source }}</span>
            <span>·</span>
            <span>{{ formatTime(article.published_at || article.read_at) }}</span>
          </div>
          <h2 class="article-title">
            <router-link :to="'/article/' + article.id" target="_blank">{{ article.title }}</router-link>
          </h2>
          <p v-if="article.summary" class="article-summary">{{ article.summary }}</p>
        </div>
      </div>

      <div v-if="hasMore" class="load-more">
        <n-button @click="loadMore" :loading="loading">加载更多</n-button>
      </div>
    </n-spin>
  </main>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useMessage } from 'naive-ui'
import { useAuthStore } from '@/stores/auth'
import { historyApi } from '@/api'
import type { Article } from '@/api'

const PER_PAGE = 20
const message = useMessage()
const auth = useAuthStore()

const articles = ref<any[]>([])
const currentPage = ref(1)
const totalPages = ref(1)
const loading = ref(true)
const hasMore = ref(false)

function formatTime(ts?: string) {
  if (!ts) return ''
  const d = new Date(ts)
  const diff = (Date.now() - d.getTime()) / 1000
  if (diff < 60) return '刚刚'
  if (diff < 3600) return Math.floor(diff / 60) + '分钟前'
  if (diff < 86400) return Math.floor(diff / 3600) + '小时前'
  if (diff < 604800) return Math.floor(diff / 86400) + '天前'
  return d.toLocaleDateString('zh-CN', { month: 'short', day: 'numeric' })
}

async function loadHistory(append = false) {
  if (!append) loading.value = true
  try {
    const res = await historyApi.list({ page: currentPage.value, per_page: PER_PAGE })
    const data = res.data
    const list: any[] = data?.articles || data?.history || []
    if (append) articles.value.push(...list)
    else articles.value = list
    totalPages.value = data?.total_pages || 1
    hasMore.value = currentPage.value < totalPages.value
  } catch { message.error('加载失败') }
  finally { loading.value = false }
}

function loadMore() {
  currentPage.value++
  loadHistory(true)
}

onMounted(async () => {
  await auth.init()
  if (auth.loggedIn) await loadHistory()
  else loading.value = false
})
</script>

<style scoped>
.history-page {
  max-width: var(--max-width);
  margin: 0 auto;
  padding: 20px 16px;
}
.page-title { margin-bottom: 20px; font-size: 22px; }
.article-list { display: flex; flex-direction: column; gap: 16px; }
.article-card {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  padding: 16px;
}
.article-card:hover { box-shadow: var(--shadow-hover); }
.article-meta { display: flex; gap: 6px; font-size: 13px; color: var(--text-secondary); margin-bottom: 8px; }
.article-title { font-size: 16px; margin-bottom: 8px; }
.article-title a { color: var(--text-primary); text-decoration: none; }
.article-title a:hover { color: var(--accent); }
.article-summary {
  font-size: 14px; color: var(--text-secondary); display: -webkit-box;
  -webkit-line-clamp: 2; -webkit-box-orient: vertical; overflow: hidden;
}
.load-more { text-align: center; padding: 20px; }
.empty-state { text-align: center; padding: 60px; display: flex; flex-direction: column; align-items: center; gap: 12px; }
</style>
