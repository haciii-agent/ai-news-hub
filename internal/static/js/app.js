// AI News Hub v0.3.0 — 前端交互逻辑
// Vanilla JS，调用后端 API 渲染数据
(function () {
  'use strict';

  // --- Config ---
  const API_BASE = '/api/v1';
  const PER_PAGE = 20;

  // --- State ---
  let currentLang = 'all';
  let currentCategory = '';
  let currentPage = 1;
  let totalPages = 1;
  let totalArticles = 0;
  let isLoading = false;
  let categoryStats = {}; // { categoryName: count }

  // --- DOM refs ---
  const articleListEl = document.getElementById('articleList');
  const loadMoreWrap = document.getElementById('loadMoreWrap');
  const loadMoreBtn = document.getElementById('loadMoreBtn');
  const statsFooter = document.getElementById('statsFooter');
  const langSwitcher = document.getElementById('langSwitcher');
  const categoryTabs = document.getElementById('categoryTabs');
  const collectStatusEl = document.getElementById('collectStatus');
  const headerClock = document.getElementById('headerClock');

  // Stats bar elements
  const statTotal = document.getElementById('statTotal');
  const statZh = document.getElementById('statZh');
  const statEn = document.getElementById('statEn');
  const statCategories = document.getElementById('statCategories');
  const statLastCollect = document.getElementById('statLastCollect');

  // --- Category tag color mapping ---
  const CATEGORY_COLORS = {
    'AI/ML': 'tag-ai_ml',
    '科技前沿': 'tag-tech_frontier',
    '商业动态': 'tag-business',
    '开源生态': 'tag-open_source',
    '学术研究': 'tag-research',
    '政策监管': 'tag-policy',
    '产品发布': 'tag-product',
    '综合资讯': 'tag-general',
  };

  // All known display categories in order
  const KNOWN_CATEGORIES = [
    '综合资讯', 'AI/ML', '科技前沿', '商业动态',
    '开源生态', '学术研究', '政策监管', '产品发布',
  ];

  // --- Init ---
  init();

  async function init() {
    bindEvents();
    startClock();
    await loadStats();
    await loadCategories();
    await loadArticles();
  }

  // --- Real-time Clock ---
  function startClock() {
    if (!headerClock) return;

    function updateClock() {
      const now = new Date();
      const h = String(now.getHours()).padStart(2, '0');
      const m = String(now.getMinutes()).padStart(2, '0');
      const s = String(now.getSeconds()).padStart(2, '0');
      headerClock.textContent = h + ':' + m + ':' + s;
    }

    updateClock();
    setInterval(updateClock, 1000);
  }

  // --- Events ---
  function bindEvents() {
    // Language switcher
    langSwitcher.addEventListener('click', function (e) {
      const btn = e.target.closest('.lang-btn');
      if (!btn || btn.classList.contains('active')) return;

      langSwitcher.querySelectorAll('.lang-btn').forEach(function (b) { b.classList.remove('active'); });
      btn.classList.add('active');

      currentLang = btn.dataset.lang;
      currentPage = 1;
      loadArticles();
    });

    // Load more
    loadMoreBtn.addEventListener('click', function () {
      if (isLoading || currentPage >= totalPages) return;
      currentPage++;
      loadArticles(true);
    });
  }

  // --- Load Stats ---
  async function loadStats() {
    try {
      const res = await fetch(API_BASE + '/stats');
      const data = await res.json();

      // Update stats bar
      if (statTotal) statTotal.textContent = data.total_articles || 0;

      const langCounts = data.article_count_by_language || {};
      if (statZh) statZh.textContent = langCounts.zh || 0;
      if (statEn) statEn.textContent = langCounts.en || 0;
      if (statCategories) statCategories.textContent = data.total_categories || 0;

      // Last collect time
      const lc = data.latest_collect;
      if (lc && lc.finished_at) {
        const timeAgo = formatTime(lc.finished_at);
        if (statLastCollect) statLastCollect.textContent = '最近采集: ' + timeAgo;

        // Update collect status badge
        updateCollectStatusBadge(lc.status);
      } else if (lc && lc.status === 'never_run') {
        if (statLastCollect) statLastCollect.textContent = '最近采集: 尚未采集';
        updateCollectStatusBadge('never_run');
      } else {
        if (statLastCollect) statLastCollect.textContent = '最近采集: 尚未采集';
        updateCollectStatusBadge('never_run');
      }
    } catch (err) {
      // Silent fail for stats
    }
  }

  // --- Update collect status badge ---
  function updateCollectStatusBadge(status) {
    if (!collectStatusEl) return;

    const statusConfig = {
      success: { icon: '✓', text: '采集正常', cls: 'success' },
      partial: { icon: '⚠', text: '部分源失败', cls: 'partial' },
      failed: { icon: '✗', text: '采集失败', cls: 'failed' },
      never_run: { icon: '○', text: '尚未采集', cls: 'never_run' },
    };

    const cfg = statusConfig[status] || statusConfig.never_run;
    collectStatusEl.className = 'collect-status-badge ' + cfg.cls;
    collectStatusEl.innerHTML = '<span class="status-icon">' + cfg.icon + '</span>' + cfg.text;
    collectStatusEl.style.display = '';
  }

  // --- Load categories ---
  async function loadCategories() {
    try {
      const res = await fetch(API_BASE + '/categories');
      const data = await res.json();

      if (data.categories && Array.isArray(data.categories)) {
        data.categories.forEach(function (cat) {
          categoryStats[cat.category] = cat.count;
        });
      }
    } catch (err) {
      console.warn('Failed to load categories:', err);
    }

    renderCategoryTabs();
  }

  // --- Render category tabs ---
  function renderCategoryTabs() {
    var html = '<button class="cat-tab active" data-category="">全部</button>';

    KNOWN_CATEGORIES.forEach(function (cat) {
      var count = categoryStats[cat] || 0;
      if (count > 0 || Object.keys(categoryStats).length === 0) {
        html += '<button class="cat-tab" data-category="' + escapeAttr(cat) + '">'
          + escapeHtml(cat)
          + (count > 0 ? '<span class="cat-count">' + count + '</span>' : '')
          + '</button>';
      }
    });

    categoryTabs.innerHTML = html;

    // Bind tab clicks
    categoryTabs.addEventListener('click', function (e) {
      var tab = e.target.closest('.cat-tab');
      if (!tab || tab.classList.contains('active')) return;

      categoryTabs.querySelectorAll('.cat-tab').forEach(function (t) { t.classList.remove('active'); });
      tab.classList.add('active');

      currentCategory = tab.dataset.category;
      currentPage = 1;
      loadArticles();

      // Smooth scroll to top
      window.scrollTo({ top: 0, behavior: 'smooth' });
    });
  }

  // --- Load articles ---
  async function loadArticles(append) {
    if (isLoading) return;
    isLoading = true;

    if (!append) {
      articleListEl.innerHTML = renderSkeletons(4);
      loadMoreWrap.style.display = 'none';
      statsFooter.textContent = '';
    } else {
      loadMoreBtn.disabled = true;
      loadMoreBtn.classList.add('loading');
      loadMoreBtn.textContent = '加载中';
    }

    try {
      var params = new URLSearchParams({
        page: currentPage,
        per_page: PER_PAGE,
        sort: 'time',
      });

      if (currentCategory) params.set('category', currentCategory);
      if (currentLang !== 'all') params.set('language', currentLang);

      var res = await fetch(API_BASE + '/articles?' + params);
      var data = await res.json();

      if (data.error) {
        if (!append) {
          articleListEl.innerHTML = renderErrorState(data.message);
        }
        return;
      }

      var articles = data.articles || [];
      totalPages = data.total_pages || 1;
      totalArticles = data.total || 0;

      if (append) {
        if (articles.length > 0) {
          articleListEl.insertAdjacentHTML('beforeend', renderArticles(articles, (currentPage - 1) * PER_PAGE));
        }
      } else {
        if (articles.length === 0) {
          articleListEl.innerHTML = renderEmptyState();
        } else {
          articleListEl.innerHTML = renderArticles(articles, 0);
        }
      }

      // Load more button
      if (currentPage < totalPages) {
        loadMoreWrap.style.display = '';
        loadMoreBtn.disabled = false;
        loadMoreBtn.classList.remove('loading');
        loadMoreBtn.textContent = '加载更多 (' + totalArticles + ' 篇)';
      } else {
        loadMoreWrap.style.display = 'none';
      }

      // Stats footer
      if (totalArticles > 0) {
        statsFooter.textContent = '共 ' + totalArticles + ' 篇 · 第 ' + currentPage + '/' + totalPages + ' 页';
      }

    } catch (err) {
      console.error('Load articles error:', err);
      if (!append) {
        articleListEl.innerHTML = renderErrorState(null);
      }
    } finally {
      isLoading = false;
      loadMoreBtn.disabled = false;
      loadMoreBtn.classList.remove('loading');
    }
  }

  // --- Render articles HTML ---
  function renderArticles(articles, startIndex) {
    return articles.map(function (a, i) {
      var rank = startIndex + i + 1;
      var catClass = CATEGORY_COLORS[a.category] || 'tag-unknown';
      var langClass = a.language === 'zh' ? 'zh' : 'en';
      var langLabel = a.language === 'zh' ? '🇨🇳' : '🇬🇧';
      var timeStr = formatTime(a.published_at || a.collected_at);
      var detailHref = '/article.html?id=' + a.id;
      var summary = a.summary ? escapeHtml(a.summary) : '';
      var category = a.category || '未分类';

      return '<div class="article-card" data-category="' + escapeAttr(category) + '">'
        + '<div class="card-rank">' + rank + '</div>'
        + '<div class="card-body">'
        + '<div class="card-title-row">'
        + '<h3 class="card-title"><a href="' + detailHref + '">' + escapeHtml(a.title) + '</a></h3>'
        + '<a class="card-url-link" href="' + escapeAttr(a.url) + '" target="_blank" rel="noopener" title="原文链接">🔗</a>'
        + '</div>'
        + (summary ? '<p class="card-summary">' + summary + '</p>' : '')
        + '<div class="card-meta">'
        + '<span class="card-source">' + escapeHtml(a.source) + '</span>'
        + '<span class="card-dot">·</span>'
        + '<span class="card-time">' + timeStr + '</span>'
        + '<span class="card-lang ' + langClass + '">' + langLabel + '</span>'
        + '<span class="tag ' + catClass + '">' + escapeHtml(category) + '</span>'
        + '</div>'
        + '</div>'
        + '</div>';
    }).join('');
  }

  // --- State renderers ---
  function renderEmptyState() {
    return '<div class="empty-state">'
      + '<div class="empty-icon">📭</div>'
      + '<p>暂无文章</p>'
      + '<p class="empty-sub">切换分类或语言查看更多内容</p>'
      + '</div>';
  }

  function renderErrorState(message) {
    var msg = message || '加载失败，请稍后再试';
    return '<div class="error-state">'
      + '<div class="error-icon">🔌</div>'
      + '<p>' + escapeHtml(msg) + '</p>'
      + '<p class="error-retry" onclick="location.reload()">点击重试</p>'
      + '</div>';
  }

  // --- Skeleton placeholders ---
  function renderSkeletons(count) {
    var html = '';
    for (var i = 0; i < count; i++) {
      html += '<div class="skeleton-card"><div class="skeleton-rank"></div><div class="skeleton-body"><div class="skeleton-title"></div><div class="skeleton-summary"></div><div class="skeleton-summary"></div><div class="skeleton-meta"></div></div></div>';
    }
    return html;
  }

  // --- Time formatting ---
  function formatTime(dateStr) {
    if (!dateStr) return '';
    try {
      var d = new Date(dateStr);
      if (isNaN(d.getTime())) return dateStr;
      var now = new Date();
      var diff = now - d;
      if (diff < 60000) return '刚刚';
      if (diff < 3600000) return Math.floor(diff / 60000) + ' 分钟前';
      if (diff < 86400000) return Math.floor(diff / 3600000) + ' 小时前';
      if (diff < 604800000) return Math.floor(diff / 86400000) + ' 天前';
      return d.toLocaleDateString('zh-CN', { month: 'short', day: 'numeric' });
    } catch (e) {
      return dateStr;
    }
  }

  // --- Utilities ---
  function escapeHtml(str) {
    if (!str) return '';
    var map = { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#039;' };
    return String(str).replace(/[&<>"']/g, function (c) { return map[c]; });
  }

  function escapeAttr(str) {
    return escapeHtml(str);
  }

})();
