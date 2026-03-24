// AI News Hub v1.3.0 — Shared Vue 3 Component Definitions (CDN, no build step)
// 每个页面加载 Vue → 创建 app → 注册需要的组件 → mount
// 使用方法: const app = createApp({...}); registerComponents(app); app.mount('#app');

const { ref, reactive, computed, onMounted } = Vue;

// ─── 通用组件 ────────────────────────────────────────────────────────────────

// AppHeader 导航栏
const AppHeader = {
  name: 'AppHeader',
  template: document.getElementById('appHeaderTemplate') ? document.getElementById('appHeaderTemplate').innerHTML : '',
  setup() {
    // 如果没有 template，从 DOM 读取
  },
};

// 注册全局组件到指定 app 实例
function registerComponents(app) {
  // AppHeader
  app.component('AppHeader', {
    name: 'AppHeader',
    template: `
    <header class="header">
      <div class="header-left">
        <a href="/" class="header-logo">
          <span class="logo-icon">📰</span>
          <span>AI News <span class="logo-text-accent">Hub</span></span>
        </a>
        <span class="header-clock" id="headerClock">{{ clock }}</span>
      </div>
      <div class="header-right">
        <div v-if="collectStatus" class="collect-status-badge" :class="collectStatus.status" id="collectStatus">
          {{ collectStatus.total_collected }}篇
        </div>
        <a href="/recommendations.html" class="header-nav-link" title="推荐">🎯</a>
        <a href="/dashboard.html" class="header-nav-link" title="数据看板">📊</a>
        <a href="/trends.html" class="header-nav-link" title="趋势分析">📈</a>
        <a href="/bookmarks.html" class="header-nav-link" title="收藏">📌</a>
        <a href="/history.html" class="header-nav-link" title="阅读历史">📖</a>
        <button class="theme-toggle" id="themeToggle" @click="toggleTheme" :title="store.theme === 'dark' ? '切换亮色' : '切换暗色'">
          {{ store.theme === 'dark' ? '☀️' : '🌙' }}
        </button>
        <div class="nav-user-area" id="navUserArea">
          <template v-if="store.isLoggedIn && store.currentUser">
            <div class="user-menu">
              <button class="user-avatar" @click="menuOpen = !menuOpen">{{ store.currentUser.username }}</button>
              <div class="user-dropdown" v-show="menuOpen" id="userDropdown">
                <a href="/profile.html">👤 个人中心</a>
                <a href="/admin.html" v-if="store.currentUser.role === 'admin'">🔧 管理后台</a>
                <button @click="handleLogout">🚪 退出登录</button>
              </div>
            </div>
          </template>
          <template v-else>
            <a href="/login.html" class="header-nav-link">登录</a>
            <a href="/login.html?tab=register" class="header-nav-link">注册</a>
          </template>
        </div>
      </div>
    </header>`,
    setup() {
      const S = window.Store;
      const clock = ref('');
      const menuOpen = ref(false);
      const collectStatus = computed(() => S.store.collectStatus);

      function updateClock() {
        const now = new Date();
        clock.value = now.toLocaleString('zh-CN', { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' });
      }
      updateClock();
      const timer = setInterval(updateClock, 30000);

      function toggleTheme() { S.toggleTheme(); }
      async function handleLogout() {
        menuOpen.value = false;
        await S.logout();
      }

      onMounted(() => {
        S.fetchCollectStatus();
        document.addEventListener('click', (e) => {
          if (!e.target.closest('.user-menu')) menuOpen.value = false;
        });
      });

      return { store: S.store, clock, menuOpen, collectStatus, toggleTheme, handleLogout };
    }
  });

  // FooterCopyright
  app.component('FooterCopyright', {
    name: 'FooterCopyright',
    template: `<div class="footer-copyright">AI News Hub v1.3.0 · 用心搭建 ⚡</div>`,
  });

  // ArticleCard
  app.component('ArticleCard', {
    name: 'ArticleCard',
    props: ['article'],
    emits: ['bookmark'],
    template: `
    <article class="article-card" :data-id="article.id">
      <div class="article-meta">
        <span class="article-source">{{ article.source }}</span>
        <span class="article-sep">·</span>
        <span class="article-time">{{ formatTime(article.published_at) }}</span>
        <span v-if="article.language === 'zh'" class="lang-badge">中文</span>
        <span v-else class="lang-badge lang-en">EN</span>
      </div>
      <h2 class="article-title">
        <a :href="'/article.html?id=' + article.id" target="_blank" @click="recordRead">{{ article.title }}</a>
      </h2>
      <p class="article-summary" v-if="article.summary">{{ article.summary }}</p>
      <div class="article-footer">
        <div class="article-tags">
          <span v-for="cat in cats" :key="cat" class="article-tag" :class="getCatClass(cat)">{{ cat }}</span>
        </div>
        <div class="article-actions">
          <button class="action-btn bookmark-btn" :class="{ bookmarked: article.is_bookmarked }"
            @click="$emit('bookmark', article)" :title="article.is_bookmarked ? '取消收藏' : '收藏'">
            {{ article.is_bookmarked ? '📌' : '📍' }}
          </button>
        </div>
      </div>
    </article>`,
    setup(props) {
      const S = window.Store;
      const cats = computed(() => (props.article.categories || '').split(',').filter(c => c.trim()));
      function formatTime(ts) {
        if (!ts) return '';
        const d = new Date(ts * 1000);
        const now = new Date();
        const diff = (now - d) / 1000;
        if (diff < 3600) return Math.floor(diff / 60) + '分钟前';
        if (diff < 86400) return Math.floor(diff / 3600) + '小时前';
        if (diff < 604800) return Math.floor(diff / 86400) + '天前';
        return d.toLocaleDateString('zh-CN', { month: 'short', day: 'numeric' });
      }
      function getCatClass(cat) {
        const m = S.CATEGORY_COLORS[cat] || 'tag-general';
        return m.replace('tag-', '');
      }
      function recordRead() {
        const h = { ...S.authHeaders() };
        delete h['Content-Type'];
        fetch(S.API_BASE + '/history', {
          method: 'POST', headers: h,
          body: JSON.stringify({ article_id: props.article.id, title: props.article.title, source: props.article.source }),
        }).catch(() => {});
      }
      return { cats, formatTime, getCatClass, recordRead };
    }
  });
}

// 导出供外部使用
window.registerComponents = registerComponents;
window.VueRefs = { ref, reactive, computed, onMounted };
