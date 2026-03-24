<template>
  <main class="dashboard-page">
    <div class="page-header">
      <h1 class="page-title">📊 数据看板</h1>
    </div>

    <n-spin :show="loading">
      <!-- Stats Cards -->
      <div class="stats-grid">
        <div class="stat-card">
          <div class="stat-label">总文章数</div>
          <div class="stat-value">{{ stats.total_articles ?? 0 }}</div>
        </div>
        <div class="stat-card">
          <div class="stat-label">今日新增</div>
          <div class="stat-value">{{ stats.today_new ?? 0 }}</div>
        </div>
        <div class="stat-card">
          <div class="stat-label">文章分类</div>
          <div class="stat-value">{{ stats.total_categories ?? 0 }}</div>
        </div>
        <div class="stat-card">
          <div class="stat-label">数据来源</div>
          <div class="stat-value">{{ stats.total_sources ?? 0 }}</div>
        </div>
      </div>

      <!-- Collect Status -->
      <div class="collect-info">
        <n-alert v-if="stats.last_collect_status === 'success'" type="success" :show-icon="true">
          最近采集成功 · {{ formatTime(stats.last_collect_time) }} · 新增 {{ stats.latest_collect?.total_new ?? 0 }} 篇
        </n-alert>
        <n-alert v-else type="warning" :show-icon="true">
          最近采集{{ stats.last_collect_status || '未知' }} · {{ formatTime(stats.last_collect_time) }}
          <span v-if="stats.latest_collect?.errors_count"> · {{ stats.latest_collect.errors_count }} 个错误</span>
        </n-alert>
      </div>

      <!-- Tab Navigation -->
      <div class="tabs-section">
        <n-tabs v-model:value="activeTab" type="line">
          <n-tab-pane name="categories" tab="🏷️ 分类">
            <div class="tab-inner">
              <div v-if="categories.length === 0" class="empty-state">暂无数据</div>
              <div v-else class="category-bars">
                <div v-for="cat in categories" :key="cat.name" class="cat-bar-row">
                  <span class="cat-name">{{ cat.name }}</span>
                  <div class="cat-bar-wrap">
                    <div class="cat-bar" :style="{ width: cat.percentage + '%' }"></div>
                  </div>
                  <span class="cat-count">{{ cat.count }}</span>
                  <span class="cat-pct">{{ cat.percentage }}%</span>
                </div>
              </div>
            </div>
          </n-tab-pane>

          <n-tab-pane name="sources" tab="📡 来源">
            <div class="tab-inner">
              <div v-if="sources.length === 0" class="empty-state">暂无数据</div>
              <div v-else class="sources-table">
                <table class="data-table">
                  <thead>
                    <tr><th>来源</th><th>类型</th><th>文章数</th><th>成功率</th><th>最后成功</th><th>状态</th></tr>
                  </thead>
                  <tbody>
                    <tr v-for="s in sources" :key="s.name">
                      <td>{{ s.name }}</td>
                      <td><n-tag size="small">{{ s.type }}</n-tag></td>
                      <td><strong>{{ s.article_count }}</strong></td>
                      <td>{{ Math.round((s.success_rate ?? 0) * 100) }}%</td>
                      <td>{{ formatTime(s.last_success) }}</td>
                      <td>
                        <n-tag :type="s.status === 'ok' ? 'success' : 'error'" size="small">
                          {{ s.status === 'ok' ? '正常' : '异常' }}
                        </n-tag>
                      </td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </div>
          </n-tab-pane>

          <n-tab-pane name="recent" tab="🆕 最新文章">
            <div class="tab-inner">
              <div v-if="recent.length === 0" class="empty-state">暂无数据</div>
              <div v-else class="recent-list">
                <div v-for="a in recent" :key="a.id" class="recent-item">
                  <span class="recent-cat" :class="getCatClass(a.category)">{{ a.category || '其他' }}</span>
                  <router-link :to="'/article/' + a.id" class="recent-title">{{ a.title }}</router-link>
                  <span class="recent-meta">{{ a.source }} · {{ formatTime(a.published_at) }}</span>
                </div>
              </div>
            </div>
          </n-tab-pane>
        </n-tabs>
      </div>
    </n-spin>
  </main>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { getDashboardStats, getDashboardCategories, getDashboardSources, getDashboardRecent } from '../api'

const loading = ref(false)
const activeTab = ref('categories')
const stats = ref<any>({})
const categories = ref<any[]>([])
const sources = ref<any[]>([])
const recent = ref<any[]>([])

function formatTime(ts?: string) {
  if (!ts) return '—'
  const d = new Date(ts)
  const diff = (Date.now() - d.getTime()) / 1000
  if (diff < 3600) return '不到1小时前'
  if (diff < 86400) return Math.floor(diff / 3600) + '小时前'
  if (diff < 604800) return Math.floor(diff / 86400) + '天前'
  return d.toLocaleDateString('zh-CN', { month: 'short', day: 'numeric' })
}

function getCatClass(cat: string) {
  const map: Record<string, string> = {
    'AI/ML': 'cat-ai', '科技前沿': 'cat-tech', '产品发布': 'cat-prod',
    '商业动态': 'cat-biz', '开源生态': 'cat-oss', '政策监管': 'cat-policy', '学术研究': 'cat-acad',
  }
  return map[cat] || 'cat-default'
}

onMounted(async () => {
  loading.value = true
  try {
    const [s, c, src, r] = await Promise.all([
      getDashboardStats(),
      getDashboardCategories(),
      getDashboardSources(),
      getDashboardRecent(),
    ])
    stats.value = s ?? {}
    categories.value = c ?? []
    sources.value = src ?? []
    recent.value = r ?? []
  } finally {
    loading.value = false
  }
})
</script>

<style scoped>
.dashboard-page {
  max-width: 1100px;
  margin: 0 auto;
  padding: 0 20px 40px;
}

.page-header { padding: 20px 0 12px; }

/* Stats Cards */
.stats-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 12px;
  margin-bottom: 12px;
}

.stat-card {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  padding: 16px 20px;
}

.stat-label { font-size: 13px; color: var(--text-secondary); margin-bottom: 6px; }
.stat-value { font-size: 28px; font-weight: 700; color: var(--text-primary); }

/* Collect Status */
.collect-info { margin-bottom: 12px; }

/* Tabs Section */
.tabs-section {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  overflow: hidden;
}

.tab-inner { padding: 20px; }

/* Category bars */
.category-bars { display: flex; flex-direction: column; gap: 10px; }
.cat-bar-row { display: flex; align-items: center; gap: 10px; }
.cat-name { width: 90px; font-size: 14px; color: var(--text-secondary); flex-shrink: 0; }
.cat-bar-wrap { flex: 1; background: var(--border); border-radius: 4px; height: 8px; overflow: hidden; }
.cat-bar { height: 100%; background: var(--accent); border-radius: 4px; transition: width 0.3s; }
.cat-count { width: 40px; text-align: right; font-size: 14px; font-weight: 600; }
.cat-pct { width: 42px; font-size: 12px; color: var(--text-secondary); }

/* Sources table */
.data-table { width: 100%; border-collapse: collapse; font-size: 14px; }
.data-table th { text-align: left; padding: 10px 12px; border-bottom: 1px solid var(--border); color: var(--text-secondary); font-weight: 500; }
.data-table td { padding: 10px 12px; border-bottom: 1px solid var(--border); }
.data-table tr:last-child td { border-bottom: none; }
.data-table tr:hover td { background: var(--bg-secondary); }

/* Recent list */
.recent-list { display: flex; flex-direction: column; }
.recent-item { display: flex; align-items: baseline; gap: 10px; padding: 10px 0; border-bottom: 1px solid var(--border); }
.recent-item:last-child { border-bottom: none; }
.recent-cat { font-size: 11px; padding: 1px 6px; border-radius: 4px; flex-shrink: 0; }
.cat-ai { background: #fef2f2; color: #dc2626; }
.cat-tech { background: #eff6ff; color: #2563eb; }
.cat-prod { background: #f0fdf4; color: #16a34a; }
.cat-biz { background: #fffbeb; color: #d97706; }
.cat-oss { background: #f5f3ff; color: #7c3aed; }
.cat-policy { background: #f9fafb; color: #6b7280; }
.cat-acad { background: #fff7ed; color: #ea580c; }
.cat-default { background: var(--surface); color: var(--text-secondary); }
.recent-title { flex: 1; font-size: 14px; color: var(--text-primary); text-decoration: none; }
.recent-title:hover { color: var(--accent); }
.recent-meta { font-size: 12px; color: var(--text-secondary); flex-shrink: 0; white-space: nowrap; }

.empty-state { text-align: center; padding: 40px; color: var(--text-secondary); }
</style>
