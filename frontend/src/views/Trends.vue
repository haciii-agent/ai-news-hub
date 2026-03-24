<template>
  <div class="trends-view">
    <div class="page-title-bar">
      <h1 class="page-title">📈 趋势分析</h1>
    </div>
    <div class="trend-tabs">
      <n-button :type="tab === 'hot' ? 'primary' : 'default'" @click="switchTab('hot')">🔥 热点新闻</n-button>
      <n-button :type="tab === 'timeline' ? 'primary' : 'default'" @click="switchTab('timeline')">📅 时间线</n-button>
      <n-button :type="tab === 'stories' ? 'primary' : 'default'" @click="switchTab('stories')">📖 故事线</n-button>
    </div>
    <n-spin :show="loading">
      <div v-if="!items.length" class="empty-state">
        <n-empty description="暂无数据" />
      </div>
      <div v-else class="article-list">
        <article v-for="item in items" :key="item.id" class="article-card">
          <div class="article-meta">
            <span class="article-source">{{ item.source }}</span>
            <span class="article-sep">·</span>
            <span class="article-time">{{ formatTime(item.published_at) }}</span>
            <n-tag v-if="item.hot_score" type="warning" size="small">🔥 {{ item.hot_score }}</n-tag>
          </div>
          <h2 class="article-title">
            <router-link :to="'/article/' + item.id">{{ item.title }}</router-link>
          </h2>
          <p v-if="item.summary" class="article-summary">{{ item.summary }}</p>
        </article>
      </div>
    </n-spin>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { getTrendsHot, getTrendsTimeline, getTrendsStories } from '../api'

const tab = ref('hot')
const loading = ref(false)
const items = ref<any[]>([])

async function load() {
  loading.value = true
  try {
    if (tab.value === 'hot') items.value = (await getTrendsHot()) ?? []
    else if (tab.value === 'timeline') items.value = (await getTrendsTimeline()) ?? []
    else items.value = (await getTrendsStories()) ?? []
  } finally { loading.value = false }
}

function switchTab(t: string) { tab.value = t; load() }

function formatTime(d: string) {
  if (!d) return ''
  const diff = Date.now() - new Date(d).getTime()
  if (diff < 86400000) return Math.floor(diff / 3600000) + '小时前'
  return new Date(d).toLocaleDateString('zh-CN')
}

onMounted(load)
</script>

<style scoped>
.page-title-bar { text-align: center; }
.trend-tabs { display: flex; gap: 8px; justify-content: center; margin: 16px 0; }
</style>
