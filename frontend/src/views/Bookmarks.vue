<template>
  <main class="bookmarks-page">
    <h1 class="page-title">📌 我的收藏</h1>

    <n-spin :show="loading && !articles.length">
      <div v-if="!auth.loggedIn" class="empty-state">
        <p>登录后可查看收藏内容</p>
        <router-link to="/login" class="n-button n-button--primary">去登录</router-link>
      </div>
      <div v-else-if="!loading && !articles.length" class="empty-state">
        <p>暂无收藏内容</p>
        <router-link to="/" class="n-button n-button--primary">去首页看看</router-link>
      </div>

      <div v-else class="article-list">
        <div v-for="article in articles" :key="article.id" class="article-card">
          <div class="article-meta">
            <span class="article-source">{{ article.source }}</span>
            <span>·</span>
            <span>{{ formatTime(article.published_at || article.collected_at) }}</span>
          </div>
          <h2 class="article-title">
            <router-link :to="'/article/' + article.id" target="_blank">{{ article.title }}</router-link>
            <a v-if="article.url" class="url-link" :href="article.url" target="_blank" rel="noopener">🔗</a>
          </h2>
          <p v-if="article.summary" class="article-summary">{{ article.summary }}</p>
          <div class="article-footer">
            <n-tag v-if="article.category" size="small">{{ article.category }}</n-tag>
            <n-button size="small" type="warning" @click="unbookmark(article)">📌 已收藏</n-button>
          </div>
        </div>
      </div>

      <div v-if="hasMore" class="load-more">
        <n-button @click="loadMore" :loading="loading">加载更多</n-button>
      </div>
    </n-spin>

    <div v-if="totalArticles > 0" class="stats-footer">
      共 {{ totalArticles }} 篇 · 第 {{ currentPage }}/{{ totalPages }} 页
    </div>
  </main>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useMessage } from 'naive-ui'
import { useAuthStore } from '@/stores/auth'
import { bookmarksApi } from '@/api'
import type { Article } from '@/api'

const PER_PAGE = 20
const message = useMessage()
const auth = useAuthStore()

const articles = ref<Article[]>([])
const currentPage = ref(1)
const totalPages = ref(1)
const totalArticles = ref(0)
const loading = ref(true)
const removing = ref(new Set<number>())

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

async function loadBookmarks(append = false) {
  if (!append) loading.value = true
  try {
    const res = await bookmarksApi.list({ page: currentPage.value, per_page: PER_PAGE })
    const data = res.data
    if (data?.error) { message.error(data.message); return }
    const list: Article[] = data?.articles || []
    if (append) articles.value.push(...list)
    else articles.value = list
    totalPages.value = data?.total_pages || 1
    totalArticles.value = data?.total || 0
    hasMore.value = currentPage.value < totalPages.value
  } catch { message.error('加载失败') }
  finally { loading.value = false }
}

async function unbookmark(article: Article) {
  if (removing.value.has(article.id)) return
  removing.value.add(article.id)
  try {
    await bookmarksApi.remove(article.id)
    articles.value = articles.value.filter(a => a.id !== article.id)
    totalArticles.value = Math.max(0, totalArticles.value - 1)
    message.success('已取消收藏')
  } catch { message.error('操作失败') }
  finally { removing.value.delete(article.id) }
}

function loadMore() {
  currentPage.value++
  loadBookmarks(true)
}

onMounted(async () => {
  await auth.init()
  if (auth.loggedIn) await loadBookmarks()
  else loading.value = false
})
</script>

<style scoped>
.bookmarks-page {
  max-width: var(--max-width);
  margin: 0 auto;
  padding: 20px 16px;
}

.page-title {
  margin-bottom: 20px;
  font-size: 22px;
}

.article-list {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.article-card {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  padding: 16px;
  transition: box-shadow 0.2s;
}
.article-card:hover { box-shadow: var(--shadow-hover); }

.article-meta {
  display: flex;
  gap: 6px;
  font-size: 13px;
  color: var(--text-secondary);
  margin-bottom: 8px;
}

.article-title {
  font-size: 16px;
  margin-bottom: 8px;
  display: flex;
  align-items: center;
  gap: 8px;
}
.article-title a { color: var(--text-primary); text-decoration: none; }
.article-title a:hover { color: var(--accent); }
.url-link { font-size: 14px; }

.article-summary {
  font-size: 14px;
  color: var(--text-secondary);
  margin-bottom: 10px;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.article-footer {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.load-more { text-align: center; padding: 20px; }

.empty-state {
  text-align: center;
  padding: 60px;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
}

.stats-footer {
  text-align: center;
  padding: 16px;
  color: var(--text-secondary);
  font-size: 14px;
}
</style>
