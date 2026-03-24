<template>
  <main class="home-page">
    <!-- Stats Bar -->
    <div class="stats-bar">
      <n-space size="large" align="center">
        <span>共 <strong>{{ stats.total }}</strong> 篇</span>
        <span>中文 <strong>{{ stats.zhTotal || stats.zh }}</strong></span>
        <span>EN <strong>{{ stats.enTotal || stats.en }}</strong></span>
        <span>分类 <strong>{{ categories.length }}</strong></span>
        <span>上次采集 <strong>{{ stats.lastCollect }}</strong></span>
      </n-space>
    </div>

    <!-- Category Tabs -->
    <div class="category-tabs">
      <n-space>
        <n-badge :value="stats.total" :max="9999" :show="stats.total > 0">
          <n-button :type="currentCategory === '' ? 'primary' : 'default'" size="small" @click="selectCategory('')">全部</n-button>
        </n-badge>
        <n-badge v-for="cat in categories" :key="cat.category" :value="cat.count" :max="9999" :show="cat.count > 0">
          <n-button :type="currentCategory === cat.category ? 'primary' : 'default'" size="small" @click="selectCategory(cat.category)">
            {{ cat.category }}
          </n-button>
        </n-badge>
      </n-space>
    </div>

    <!-- Search Bar -->
    <div class="search-bar">
      <n-input
        v-model:value="searchQuery"
        placeholder="搜索新闻标题或摘要..."
        clearable
        @keyup.enter="doSearch"
        @clear="clearSearch"
      >
        <template #prefix>🔍</template>
      </n-input>
      <n-button type="primary" @click="doSearch">搜索</n-button>
    </div>

    <!-- Article List -->
    <div class="article-list">
      <n-spin :show="loading && !articles.length">
        <div v-if="!loading && !articles.length && !isSearchMode" class="empty-state">
          <p>暂无文章</p>
          <n-button @click="loadArticles()">重试</n-button>
        </div>
        <div v-else-if="!loading && !articles.length && isSearchMode" class="empty-state">
          <p>没有找到相关文章</p>
        </div>

        <div v-for="article in articles" :key="article.id" class="article-card">
          <div class="article-meta">
            <span class="article-source">{{ article.source }}</span>
            <span class="article-sep">·</span>
            <span class="article-time">{{ formatTime(article.published_at) }}</span>
            <n-tag :type="article.language === 'zh' ? 'info' : 'default'" size="small">
              {{ article.language === 'zh' ? '🇨🇳 中文' : '🇬🇧 EN' }}
            </n-tag>
          </div>

          <h2 class="article-title">
            <router-link :to="'/article/' + article.id" target="_blank" @click="recordRead(article)">
              {{ article.title }}
            </router-link>
          </h2>

          <p v-if="article.summary" class="article-summary">{{ article.summary }}</p>

          <div class="article-footer">
            <div class="article-tags">
              <n-tag
                v-for="cat in getCats(article.categories)"
                :key="cat"
                size="small"
                :type="getCatType(cat)"
              >{{ cat }}</n-tag>
            </div>
            <div class="article-actions">
              <n-button
                quaternary
                size="small"
                @click="toggleBookmark(article)"
                :type="bookmarkState[article.id] ? 'warning' : 'default'"
              >
                {{ bookmarkState[article.id] ? '📌' : '📍' }}
              </n-button>
            </div>
          </div>
        </div>

        <div v-if="hasMore" class="load-more">
          <n-button v-if="!loading" @click="loadMore" type="primary" ghost>加载更多</n-button>
          <n-button v-else type="primary" loading>加载中...</n-button>
        </div>
      </n-spin>
    </div>

    <div v-if="articles.length" class="stats-footer">
      已显示 {{ displayedCount }} 篇
    </div>
  </main>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { useMessage } from 'naive-ui'
import { articlesApi, bookmarksApi, categoriesApi, historyApi, getDashboardStats } from '@/api'
import type { Article } from '@/api'

const PER_PAGE = 20

const message = useMessage()
const articles = ref<Article[]>([])
const loading = ref(false)
const currentPage = ref(1)
const totalPages = ref(1)
const currentCategory = ref('')
const searchQuery = ref('')
const isSearchMode = ref(false)
const categories = ref<{ category: string; count: number }[]>([])
const stats = reactive({ total: 0, zh: 0, en: 0, zhTotal: 0, enTotal: 0, lastCollect: '—' })
const displayedCount = ref(0)
const hasMore = ref(false)
const bookmarkState = reactive<Record<number, boolean>>({})

function getCats(categoriesStr?: string) {
  return (categoriesStr || '').split(',').filter(Boolean)
}

function getCatType(cat: string) {
  const map: Record<string, 'error' | 'info' | 'success' | 'warning' | 'default'> = {
    'AI/ML': 'error',
    '科技前沿': 'info',
    '商业动态': 'warning',
    '开源生态': 'success',
    '学术研究': 'warning',
    '政策监管': 'default',
    '产品发布': 'info',
  }
  return map[cat] || 'default'
}

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

async function loadStats() {
  try {
    const data = await getDashboardStats()
    stats.total = data.total_articles ?? 0
    stats.lastCollect = data.last_collect_time
      ? new Date(data.last_collect_time).toLocaleDateString('zh-CN', { month: 'short', day: 'numeric' })
      : '—'
  } catch {}
}

async function loadCategories() {
  try {
    const res = await categoriesApi.list()
    const data = res.data
    if (data?.categories) {
      categories.value = data.categories
    }
  } catch {}
}

async function loadArticles(append = false) {
  if (!append) loading.value = true
  try {
    const res = await articlesApi.list({
      page: currentPage.value,
      per_page: PER_PAGE,
      category: currentCategory.value || undefined,
      q: searchQuery.value || undefined,
    })
    const list: Article[] = res.data?.articles || []
    if (append) articles.value.push(...list)
    else articles.value = list

    // Language stats from backend
    if (!append) {
      const langStats = res.data?.lang_stats
      if (langStats) {
        stats.zhTotal = langStats.zh ?? 0
        stats.enTotal = langStats.en ?? 0
      }
      if (list.length) {
        stats.zh = list.filter(a => a.language === 'zh').length
        stats.en = list.filter(a => a.language === 'en').length
      }
    }

    hasMore.value = list.length >= PER_PAGE
    displayedCount.value = articles.value.length
  } catch (e) {
    message.error('加载文章失败')
  } finally {
    loading.value = false
  }
}

function selectCategory(cat: string) {
  currentCategory.value = cat
  currentPage.value = 1
  articles.value = []
  loadArticles()
}

function doSearch() {
  isSearchMode.value = !!searchQuery.value.trim()
  currentPage.value = 1
  articles.value = []
  loadArticles()
}

function clearSearch() {
  isSearchMode.value = false
  currentPage.value = 1
  articles.value = []
  loadArticles()
}

function loadMore() {
  currentPage.value++
  loadArticles(true)
}

async function toggleBookmark(article: Article) {
  try {
    if (bookmarkState[article.id]) {
      bookmarkState[article.id] = false
      message.success('已取消收藏')
    } else {
      await bookmarksApi.add(article.id, article.title, article.source || '')
      bookmarkState[article.id] = true
      message.success('已收藏')
    }
  } catch {
    message.error('操作失败，请先登录')
  }
}

function recordRead(_article: Article) {
  // fire-and-forget
}

onMounted(async () => {
  await Promise.all([loadStats(), loadCategories(), loadArticles()])

  const params = new URLSearchParams(window.location.search)
  if (params.get('q')) {
    searchQuery.value = params.get('q')!
    isSearchMode.value = true
    loadArticles()
  }
})
</script>

<style scoped>
.home-page {
  max-width: var(--max-width);
  margin: 0 auto;
  padding: 0 16px 40px;
}

.stats-bar {
  padding: 12px 0;
  color: var(--text-secondary);
  font-size: 14px;
}
.stats-bar strong { color: var(--text-primary); }

.category-tabs {
  margin-bottom: 16px;
  flex-wrap: wrap;
  gap: 6px;
}
.category-tabs :deep(.n-badge) {
  display: inline-flex;
}

.search-bar {
  display: flex;
  gap: 8px;
  margin-bottom: 20px;
}
.search-bar .n-input { flex: 1; }

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
.article-card:hover {
  box-shadow: var(--shadow-hover);
}

.article-meta {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  color: var(--text-secondary);
  margin-bottom: 8px;
}
.article-source { font-weight: 500; }
.article-sep { opacity: 0.5; }

.article-title {
  font-size: 17px;
  line-height: 1.4;
  margin-bottom: 8px;
}
.article-title a {
  color: var(--text-primary);
  text-decoration: none;
}
.article-title a:hover { color: var(--accent); }

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
  align-items: center;
  justify-content: space-between;
}

.article-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}

.load-more {
  text-align: center;
  padding: 20px;
}

.empty-state {
  text-align: center;
  padding: 60px;
  color: var(--text-secondary);
}

.stats-footer {
  text-align: center;
  padding: 16px;
  color: var(--text-secondary);
  font-size: 14px;
}
</style>
