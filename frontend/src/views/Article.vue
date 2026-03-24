<template>
  <main class="article-page">
    <div class="article-container">
      <n-button @click="$router.back()" quaternary>← 返回列表</n-button>

      <n-spin :show="loading">
        <div v-if="error" class="error-state">
          <p>{{ error }}</p>
          <n-button @click="loadArticle">重试</n-button>
        </div>

        <div v-else-if="article" class="article-detail-card">
          <h1 class="article-title">{{ article.title }}</h1>

          <div class="article-meta">
            <span>📌 {{ article.source }}</span>
            <span>·</span>
            <span>🕐 {{ formatTime(article.published_at || article.collected_at) }}</span>
            <n-tag :type="article.language === 'zh' ? 'info' : 'default'" size="small">
              {{ article.language === 'zh' ? '🇨🇳 中文' : '🇬🇧 EN' }}
            </n-tag>
            <n-tag v-if="article.category" size="small">{{ article.category }}</n-tag>
            <span v-if="article.importance_score > 0" class="score-badge">
              {{ article.importance_score >= 80 ? '🔥' : article.importance_score >= 60 ? '⚡' : '📌' }}
              重要度 {{ Math.round(article.importance_score) }}/100
            </span>
          </div>

          <img v-if="article.image_url" class="hero-image" :src="article.image_url" :alt="article.title" loading="lazy" />

          <div v-if="article.ai_summary" class="ai-summary">
            <div class="ai-summary-header">🤖 AI 智能摘要</div>
            <div class="ai-summary-body">{{ article.ai_summary }}</div>
          </div>

          <div v-if="article.summary && article.summary !== article.ai_summary" class="article-summary">
            <strong>摘要：</strong>{{ article.summary }}
          </div>

          <div v-if="article.content_html" class="article-content" v-html="article.content_html"></div>

          <!-- Interaction Bar -->
          <div class="interaction-bar">
            <span>❤️ {{ interactions.likes_count }} 人点赞</span>
            <span>💬 {{ commentsData.total || 0 }} 条评论</span>
            <n-button :type="interactions.is_liked ? 'error' : 'default'" size="small" @click="toggleLike">
              {{ interactions.is_liked ? '❤️ 已赞' : '🤍 点赞' }}
            </n-button>
            <n-button size="small" @click="copyLink">📋 复制链接</n-button>
            <a :href="article.url" target="_blank" rel="noopener" class="n-button n-button--small">🔗 原文</a>
          </div>

          <!-- Comments -->
          <div class="comments-section">
            <h3>💬 评论区</h3>
            <div class="comment-form">
              <n-input
                v-model:value="commentText"
                placeholder="发表评论..."
                type="textarea"
                :maxlength="500"
                @keydown.enter.ctrl.prevent="submitComment"
              />
              <n-button type="primary" :loading="submitting" @click="submitComment">发表</n-button>
            </div>

            <div v-if="!commentsData.comments?.length" class="comments-empty">暂无评论</div>
            <div v-else class="comments-list">
              <div v-for="c in commentsData.comments" :key="c.id" class="comment-item">
                <div class="comment-avatar">👤</div>
                <div class="comment-body">
                  <div class="comment-meta">
                    <span class="comment-author">匿名用户</span>
                    <span class="comment-time">{{ formatTime(c.created_at) }}</span>
                    <n-button v-if="c.user_id === currentUserId" text type="error" size="tiny" @click="deleteComment(c.id)">删除</n-button>
                  </div>
                  <div class="comment-text">{{ c.content }}</div>
                </div>
              </div>
            </div>
          </div>

          <!-- Actions -->
          <div class="detail-actions">
            <n-button type="primary" @click="togglePreview">
              {{ previewOpen ? '收起正文' : '📖 查看正文' }}
            </n-button>
            <n-button @click="toggleBookmark" :type="article.is_bookmarked ? 'warning' : 'default'">
              {{ article.is_bookmarked ? '❤️ 已收藏' : '🤍 收藏' }}
            </n-button>
          </div>

          <!-- Content Preview -->
          <div v-show="previewOpen" class="content-preview">
            <div class="preview-header">
              <span>📖 正文预览</span>
              <span v-if="sourceDomain" class="preview-source">来源: {{ sourceDomain }}</span>
            </div>
            <div v-if="previewLoading" class="preview-loading">
              <n-spin size="large" />正在提取正文…
            </div>
            <div v-else-if="previewData?.html" class="preview-body" v-html="previewData.html"></div>
            <div v-else class="preview-error">未能提取到正文内容，请直接访问原文</div>
            <div class="preview-footer">⚠️ 此内容由 AI 自动提取，如有错漏请以原文为准</div>
          </div>
        </div>
      </n-spin>
    </div>
  </main>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useMessage } from 'naive-ui'
import { articlesApi } from '@/api'

const route = useRoute()
const router = useRouter()
const message = useMessage()

const article = ref<any>(null)
const loading = ref(true)
const error = ref('')
const commentText = ref('')
const submitting = ref(false)
const previewOpen = ref(false)
const previewLoading = ref(false)
const previewData = ref<any>(null)
const currentUserId = ref(0)

const interactions = reactive({ likes_count: 0, is_liked: false })
const commentsData = reactive<any>({ comments: [], total: 0 })

const sourceDomain = computed(() => {
  if (!article.value?.url) return ''
  try { return new URL(article.value.url).hostname } catch { return '' }
})

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

async function loadArticle() {
  const id = Number(route.params.id)
  if (!id) { error.value = '缺少文章 ID'; loading.value = false; return }
  loading.value = true
  error.value = ''
  try {
    const res = await articlesApi.getContent(id)
    if (res.data?.error) { error.value = res.data.message || '加载失败'; return }
    article.value = res.data
    document.title = (article.value.title || '文章') + ' — AI News Hub'
  } catch { error.value = '网络错误' }
  finally { loading.value = false }
}

async function toggleLike() {
  try {
    if (interactions.is_liked) {
      await articlesApi.unlike(Number(route.params.id))
      interactions.is_liked = false
      interactions.likes_count = Math.max(0, interactions.likes_count - 1)
      message.success('取消点赞')
    } else {
      await articlesApi.like(Number(route.params.id))
      interactions.is_liked = true
      interactions.likes_count++
      message.success('已点赞')
    }
  } catch { message.error('操作失败') }
}

async function submitComment() {
  const content = commentText.value.trim()
  if (!content || content.length > 500) { message.warning('评论内容长度需在 1-500 字符之间'); return }
  submitting.value = true
  try {
    await articlesApi.addComment(Number(route.params.id), content)
    commentText.value = ''
    message.success('评论成功')
    // Reload comments
    const res = await articlesApi.getComments(Number(route.params.id))
    Object.assign(commentsData, res.data)
  } catch { message.error('网络错误') }
  finally { submitting.value = false }
}

async function deleteComment(commentId: number) {
  try {
    await articlesApi.deleteComment(Number(route.params.id), commentId)
    commentsData.comments = commentsData.comments.filter((c: any) => c.id !== commentId)
    message.success('已删除')
  } catch { message.error('删除失败') }
}

async function toggleBookmark() {
  message.info('请前往收藏页面管理')
}

function copyLink() {
  navigator.clipboard?.writeText(window.location.href).catch(() => {})
  message.success('链接已复制')
}

async function togglePreview() {
  previewOpen.value = !previewOpen.value
  if (previewOpen.value && !previewData.value) {
    previewLoading.value = true
    try {
      const res = await articlesApi.getContent(Number(route.params.id))
      previewData.value = res.data
    } catch { previewData.value = {} }
    finally { previewLoading.value = false }
  }
}

onMounted(async () => {
  await loadArticle()
})
</script>

<style scoped>
.article-page {
  max-width: var(--max-width);
  margin: 0 auto;
  padding: 20px 16px;
}

.article-container {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.article-detail-card {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  padding: 24px;
}

.article-title {
  font-size: 24px;
  line-height: 1.4;
  margin-bottom: 12px;
}

.article-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
  color: var(--text-secondary);
  font-size: 14px;
  margin-bottom: 16px;
}

.hero-image {
  width: 100%;
  max-height: 400px;
  object-fit: cover;
  border-radius: var(--radius-sm);
  margin-bottom: 16px;
}

.ai-summary {
  background: var(--border);
  border-radius: var(--radius-sm);
  padding: 16px;
  margin-bottom: 16px;
}
.ai-summary-header { font-weight: 600; margin-bottom: 8px; }
.ai-summary-body { font-size: 15px; line-height: 1.6; }

.article-summary {
  font-size: 14px;
  color: var(--text-secondary);
  margin-bottom: 16px;
}

.article-content {
  font-size: 15px;
  line-height: 1.8;
  margin-bottom: 20px;
}

.interaction-bar {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  align-items: center;
  padding: 16px 0;
  border-top: 1px solid var(--border);
  border-bottom: 1px solid var(--border);
  margin-bottom: 20px;
}

.comments-section {
  margin-bottom: 20px;
}
.comments-section h3 { margin-bottom: 12px; }

.comment-form {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-bottom: 16px;
}

.comments-empty {
  text-align: center;
  color: var(--text-secondary);
  padding: 20px;
}

.comment-item {
  display: flex;
  gap: 10px;
  padding: 12px 0;
  border-bottom: 1px solid var(--border);
}
.comment-avatar { font-size: 24px; }
.comment-meta {
  display: flex;
  gap: 8px;
  align-items: center;
  font-size: 13px;
  margin-bottom: 4px;
}
.comment-author { font-weight: 600; }
.comment-text { font-size: 14px; }

.detail-actions {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
  margin-bottom: 16px;
}

.content-preview {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  padding: 16px;
}
.preview-header {
  display: flex;
  justify-content: space-between;
  font-weight: 600;
  margin-bottom: 12px;
}
.preview-loading { text-align: center; padding: 40px; }
.preview-footer { margin-top: 12px; font-size: 12px; color: var(--text-secondary); }

.error-state {
  text-align: center;
  padding: 60px;
  color: var(--text-secondary);
}
</style>
