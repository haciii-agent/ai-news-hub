// AI News Hub — 前端交互逻辑
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
    await loadCategories();
    await loadArticles();
  }

  // --- Events ---
  function bindEvents() {
    // Language switcher
    langSwitcher.addEventListener('click', function (e) {
      const btn = e.target.closest('.lang-btn');
      if (!btn || btn.classList.contains('active')) return;

      langSwitcher.querySelectorAll('.lang-btn').forEach(b => b.classList.remove('active'));
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

  // --- Load categories ---
  async function loadCategories() {
    try {
      const res = await fetch(`${API_BASE}/categories`);
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
    // Build tab list: "全部" + known categories that have articles
    let html = '<button class="cat-tab active" data-category="">全部</button>';

    KNOWN_CATEGORIES.forEach(function (cat) {
      const count = categoryStats[cat] || 0;
      // Only show categories with articles, or all if no stats yet
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
      const tab = e.target.closest('.cat-tab');
      if (!tab || tab.classList.contains('active')) return;

      categoryTabs.querySelectorAll('.cat-tab').forEach(t => t.classList.remove('active'));
      tab.classList.add('active');

      currentCategory = tab.dataset.category;
      currentPage = 1;
      loadArticles();
    });
  }

  // --- Load articles ---
  async function loadArticles(append) {
    if (isLoading) return;
    isLoading = true;

    if (!append) {
      articleListEl.innerHTML = renderSkeletons(3);
      loadMoreWrap.style.display = 'none';
      statsFooter.textContent = '';
    } else {
      loadMoreBtn.disabled = true;
      loadMoreBtn.textContent = '加载中…';
    }

    try {
      const params = new URLSearchParams({
        page: currentPage,
        per_page: PER_PAGE,
        sort: 'time',
      });

      if (currentCategory) params.set('category', currentCategory);
      if (currentLang !== 'all') params.set('language', currentLang);

      const res = await fetch(`${API_BASE}/articles?${params}`);
      const data = await res.json();

      if (data.error) {
        if (!append) {
          articleListEl.innerHTML = '<div class="error-state"><p>' + escapeHtml(data.message) + '</p></div>';
        }
        return;
      }

      const articles = data.articles || [];
      totalPages = data.total_pages || 1;
      totalArticles = data.total || 0;

      if (append) {
        articleListEl.insertAdjacentHTML('beforeend', articles.length > 0 ? renderArticles(articles, (currentPage - 1) * PER_PAGE) : '');
      } else {
        if (articles.length === 0) {
          articleListEl.innerHTML = '<div class="empty-state"><div class="empty-icon">📭</div><p>暂无文章</p></div>';
        } else {
          articleListEl.innerHTML = renderArticles(articles, 0);
        }
      }

      // Load more button
      if (currentPage < totalPages) {
        loadMoreWrap.style.display = '';
        loadMoreBtn.disabled = false;
        loadMoreBtn.textContent = '加载更多 (' + totalArticles + ' 篇)';
      } else {
        loadMoreWrap.style.display = 'none';
      }

      // Stats footer
      if (totalArticles > 0) {
        statsFooter.textContent = '共 ' + totalArticles + ' 篇文章 · 第 ' + currentPage + '/' + totalPages + ' 页';
      }

    } catch (err) {
      console.error('Load articles error:', err);
      if (!append) {
        articleListEl.innerHTML = '<div class="error-state"><p>加载失败，请刷新重试</p></div>';
      }
    } finally {
      isLoading = false;
      loadMoreBtn.disabled = false;
    }
  }

  // --- Render articles HTML ---
  function renderArticles(articles, startIndex) {
    return articles.map(function (a, i) {
      const rank = startIndex + i + 1;
      const catClass = CATEGORY_COLORS[a.category] || 'tag-unknown';
      const langClass = a.language === 'zh' ? 'zh' : 'en';
      const langLabel = a.language === 'zh' ? '中' : 'EN';
      const timeStr = formatTime(a.published_at || a.collected_at);
      const detailHref = '/article.html?id=' + a.id;
      const summary = a.summary ? escapeHtml(a.summary) : '';

      return '<div class="article-card">'
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
        + '<span class="tag ' + catClass + '">' + escapeHtml(a.category || '未分类') + '</span>'
        + '</div>'
        + '</div>'
        + '</div>';
    }).join('');
  }

  // --- Skeleton placeholders ---
  function renderSkeletons(count) {
    let html = '';
    for (let i = 0; i < count; i++) {
      html += '<div class="skeleton-card"><div class="skeleton-rank"></div><div class="skeleton-body"><div class="skeleton-title"></div><div class="skeleton-summary"></div><div class="skeleton-summary"></div><div class="skeleton-meta"></div></div></div>';
    }
    return html;
  }

  // --- Time formatting ---
  function formatTime(dateStr) {
    if (!dateStr) return '';
    try {
      const d = new Date(dateStr);
      if (isNaN(d.getTime())) return dateStr;
      const now = new Date();
      const diff = now - d;
      if (diff < 60000) return '刚刚';
      if (diff < 3600000) return Math.floor(diff / 60000) + ' 分钟前';
      if (diff < 86400000) return Math.floor(diff / 3600000) + ' 小时前';
      if (diff < 604800000) return Math.floor(diff / 86400000) + ' 天前';
      return d.toLocaleDateString('zh-CN', { month: 'short', day: 'numeric' });
    } catch {
      return dateStr;
    }
  }

  // --- Utilities ---
  function escapeHtml(str) {
    if (!str) return '';
    const map = { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#039;' };
    return String(str).replace(/[&<>"']/g, function (c) { return map[c]; });
  }

  function escapeAttr(str) {
    return escapeHtml(str);
  }

})();
