// AI News Hub v0.5.0 — 前端交互逻辑
// Vanilla JS，调用后端 API 渲染数据
(function () {
  'use strict';

  // --- Config ---
  const API_BASE = '/api/v1';
  const PER_PAGE = 20;

  // --- User Token (v0.7.0) ---
  function getUserToken() {
    let token = localStorage.getItem('user_token');
    if (!token) {
      token = crypto.randomUUID();
      localStorage.setItem('user_token', token);
    }
    return token;
  }

  function authHeaders() {
    return {
      'Content-Type': 'application/json',
      'X-User-Token': getUserToken(),
    };
  }

  // Bookmark state cache
  let bookmarkState = {}; // { articleId: bool }

  // --- State ---
  let currentLang = 'all';
  let currentCategory = '';
  let currentPage = 1;
  let totalPages = 1;
  let totalArticles = 0;
  let isLoading = false;
  let categoryStats = {}; // { categoryName: count }

  // Search state
  let isSearchMode = false;
  let searchDebounceTimer = null;
  let currentSearchQuery = '';

  // --- DOM refs ---
  const articleListEl = document.getElementById('articleList');
  const loadMoreWrap = document.getElementById('loadMoreWrap');
  const loadMoreBtn = document.getElementById('loadMoreBtn');
  const statsFooter = document.getElementById('statsFooter');
  const langSwitcher = document.getElementById('langSwitcher');
  const categoryTabs = document.getElementById('categoryTabs');
  const collectStatusEl = document.getElementById('collectStatus');
  const headerClock = document.getElementById('headerClock');
  const themeToggle = document.getElementById('themeToggle');

  // Search DOM refs
  const searchContainer = document.getElementById('searchContainer');
  const searchToggle = document.getElementById('searchToggle');
  const searchInputWrap = document.getElementById('searchInputWrap');
  const searchInput = document.getElementById('searchInput');
  const searchClear = document.getElementById('searchClear');

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
    initTheme();
    bindEvents();
    startClock();
    // Init user token (ensures it exists in localStorage)
    getUserToken();
    // Fire-and-forget user init
    fetch(API_BASE + '/user/init', {
      method: 'POST',
      headers: authHeaders(),
    }).catch(function() {});
    await loadStats();
    await loadCategories();

    // Check URL params for pre-filled search
    const urlParams = new URLSearchParams(window.location.search);
    const urlSearch = urlParams.get('search');
    if (urlSearch && urlSearch.trim()) {
      currentSearchQuery = urlSearch.trim();
      openSearchInput();
      searchInput.value = currentSearchQuery;
      searchClear.style.display = '';
      enterSearchMode();
      await performSearch(currentSearchQuery);
    } else {
      await loadArticles();
    }
  }

  // --- Theme Toggle ---
  function initTheme() {
    const saved = localStorage.getItem('theme');
    if (saved) {
      document.documentElement.setAttribute('data-theme', saved);
    }
    updateThemeIcon();
  }

  function updateThemeIcon() {
    if (!themeToggle) return;
    const isLight = document.documentElement.getAttribute('data-theme') === 'light';
    themeToggle.textContent = isLight ? '☀️' : '🌙';
  }

  function toggleTheme() {
    const current = document.documentElement.getAttribute('data-theme');
    const next = current === 'light' ? 'dark' : 'light';
    document.documentElement.setAttribute('data-theme', next);
    localStorage.setItem('theme', next);
    updateThemeIcon();
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
    // Theme toggle
    if (themeToggle) {
      themeToggle.addEventListener('click', toggleTheme);
    }

    // Language switcher
    langSwitcher.addEventListener('click', function (e) {
      const btn = e.target.closest('.lang-btn');
      if (!btn || btn.classList.contains('active')) return;

      langSwitcher.querySelectorAll('.lang-btn').forEach(function (b) { b.classList.remove('active'); });
      btn.classList.add('active');

      currentLang = btn.dataset.lang;
      currentPage = 1;
      if (isSearchMode) {
        performSearch(currentSearchQuery);
      } else {
        loadArticles();
      }
    });

    // Load more
    loadMoreBtn.addEventListener('click', function () {
      if (isLoading || currentPage >= totalPages) return;
      currentPage++;
      if (isSearchMode) {
        performSearch(currentSearchQuery, true);
      } else {
        loadArticles(true);
      }
    });

    // --- Search events ---
    // Toggle search input open/close
    if (searchToggle) {
      searchToggle.addEventListener('click', function () {
        if (isSearchInputOpen()) {
          closeSearchInput();
          if (isSearchMode) {
            exitSearch();
          }
        } else {
          openSearchInput();
          searchInput.focus();
        }
      });
    }

    // Search input with debounce
    if (searchInput) {
      searchInput.addEventListener('input', function () {
        const query = searchInput.value.trim();
        searchClear.style.display = query ? '' : 'none';

        if (searchDebounceTimer) {
          clearTimeout(searchDebounceTimer);
        }

        if (!query) {
          exitSearch();
          return;
        }

        searchDebounceTimer = setTimeout(function () {
          currentSearchQuery = query;
          enterSearchMode();
          performSearch(query);
        }, 300);
      });

      // ESC to exit search
      searchInput.addEventListener('keydown', function (e) {
        if (e.key === 'Escape') {
          searchInput.value = '';
          searchClear.style.display = 'none';
          closeSearchInput();
          if (isSearchMode) {
            exitSearch();
          }
        }
      });
    }

    // Clear search button
    if (searchClear) {
      searchClear.addEventListener('click', function () {
        searchInput.value = '';
        searchClear.style.display = 'none';
        closeSearchInput();
        if (isSearchMode) {
          exitSearch();
        }
      });
    }

    // Keyboard shortcut: "/" to focus search (when not in an input)
    document.addEventListener('keydown', function (e) {
      if (e.key === '/' && !isSearchInputOpen()) {
        const tag = document.activeElement.tagName.toLowerCase();
        if (tag !== 'input' && tag !== 'textarea' && !document.activeElement.isContentEditable) {
          e.preventDefault();
          openSearchInput();
          searchInput.focus();
        }
      }
    });
  }

  // --- Search functions ---
  function openSearchInput() {
    if (searchContainer) searchContainer.classList.add('search-open');
  }

  function closeSearchInput() {
    if (searchContainer) searchContainer.classList.remove('search-open');
    if (searchInput) searchInput.blur();
  }

  function isSearchInputOpen() {
    return searchContainer && searchContainer.classList.contains('search-open');
  }

  function enterSearchMode() {
    if (!isSearchMode) {
      isSearchMode = true;
      document.body.classList.add('search-mode');
    }
  }

  function exitSearch() {
    isSearchMode = false;
    currentSearchQuery = '';
    document.body.classList.remove('search-mode');
    searchDebounceTimer = null;
    currentPage = 1;
    loadArticles();
  }

  async function performSearch(query, append) {
    if (isLoading) return;
    isLoading = true;

    if (!append) {
      articleListEl.innerHTML = renderSkeletons(4);
      loadMoreWrap.style.display = 'none';
      statsFooter.textContent = '';
    } else {
      loadMoreBtn.disabled = true;
      loadMoreBtn.classList.add('loading');
      loadMoreBtn.textContent = '搜索中';
    }

    try {
      var params = new URLSearchParams({
        page: currentPage,
        per_page: PER_PAGE,
        search: query,
      });

      if (currentCategory) params.set('category', currentCategory);
      if (currentLang !== 'all') params.set('language', currentLang);

      var res = await fetch(API_BASE + '/articles?' + params, {
        headers: { 'X-User-Token': getUserToken() },
      });
      var data = await res.json();

      if (data.error) {
        if (!append) {
          articleListEl.innerHTML = renderErrorState(data.message);
        }
        return;
      }

      var articles = data.articles || [];
      var snippets = data.snippets || {};
      totalPages = data.total_pages || 1;
      totalArticles = data.total || 0;

      if (append) {
        if (articles.length > 0) {
          articleListEl.insertAdjacentHTML('beforeend', renderSearchResults(articles, snippets, (currentPage - 1) * PER_PAGE));
        }
      } else {
        if (articles.length === 0) {
          articleListEl.innerHTML = renderSearchEmptyState(query);
        } else {
          articleListEl.innerHTML = renderSearchResults(articles, snippets, 0);
        }
      }

      // Fetch bookmark status for search results
      if (articles.length > 0) {
        fetchBookmarkStatus(articles);
      }

      // Load more button
      if (currentPage < totalPages) {
        loadMoreWrap.style.display = '';
        loadMoreBtn.disabled = false;
        loadMoreBtn.classList.remove('loading');
        loadMoreBtn.textContent = '加载更多搜索结果';
      } else {
        loadMoreWrap.style.display = 'none';
      }

      // Stats footer — search mode indicator
      if (totalArticles > 0) {
        statsFooter.innerHTML = '<span class="search-mode-indicator">🔍 搜索结果 · 共 ' + totalArticles + ' 条</span>';
      }

    } catch (err) {
      console.error('Search error:', err);
      if (!append) {
        articleListEl.innerHTML = renderErrorState(null);
      }
    } finally {
      isLoading = false;
      loadMoreBtn.disabled = false;
      loadMoreBtn.classList.remove('loading');
    }
  }

  // --- Render search results ---
  function renderSearchResults(articles, snippets, startIndex) {
    return articles.map(function (a, i) {
      var rank = startIndex + i + 1;
      var catClass = CATEGORY_COLORS[a.category] || 'tag-unknown';
      var langClass = a.language === 'zh' ? 'zh' : 'en';
      var langLabel = a.language === 'zh' ? '🇨🇳' : '🇬🇧';
      var timeStr = formatTime(a.published_at || a.collected_at);
      var detailHref = '/article.html?id=' + a.id;
      var category = a.category || '未分类';

      // Snippet with <mark> highlights (already HTML-safe from backend)
      var snippetText = snippets[a.id] || '';
      // If snippet is empty, use summary (truncated) without highlights
      var summaryHtml = '';
      if (snippetText) {
        summaryHtml = '<p class="card-summary card-snippet">' + snippetText + '</p>';
      } else if (a.summary) {
        summaryHtml = '<p class="card-summary">' + escapeHtml(a.summary) + '</p>';
      }

      // Thumbnail
      var thumbnailHtml = '';
      if (a.image_url) {
        thumbnailHtml = '<div class="card-thumbnail"><img src="' + escapeAttr(a.image_url) + '" alt="" loading="lazy" onerror="this.parentElement.style.display=\'none\'"></div>';
      }

      return '<div class="article-card" data-category="' + escapeAttr(category) + '">'
        + '<div class="card-rank">' + rank + '</div>'
        + thumbnailHtml
        + '<div class="card-body">'
        + '<div class="card-title-row">'
        + '<h3 class="card-title"><a href="' + detailHref + '">' + escapeHtml(a.title) + '</a></h3>'
        + '<a class="card-url-link" href="' + escapeAttr(a.url) + '" target="_blank" rel="noopener" title="原文链接">🔗</a>'
        + '</div>'
        + summaryHtml
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

  function renderSearchEmptyState(query) {
    return '<div class="empty-state">'
      + '<div class="empty-icon">🔍</div>'
      + '<p>未找到 "<strong>' + escapeHtml(query) + '</strong>" 相关结果</p>'
      + '<p class="empty-sub">尝试使用不同的关键词搜索</p>'
      + '</div>';
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
      if (isSearchMode) {
        performSearch(currentSearchQuery);
      } else {
        loadArticles();
      }

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

      var res = await fetch(API_BASE + '/articles?' + params, {
        headers: { 'X-User-Token': getUserToken() },
      });
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

      // Fetch bookmark status for current page articles
      if (articles.length > 0) {
        fetchBookmarkStatus(articles);
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
      var isBookmarked = !!bookmarkState[a.id];
      var bookmarkIcon = isBookmarked ? '❤️' : '🤍';
      var bookmarkClass = isBookmarked ? 'bookmark-btn bookmarked' : 'bookmark-btn';

      // Thumbnail
      var thumbnailHtml = '';
      if (a.image_url) {
        thumbnailHtml = '<div class="card-thumbnail"><img src="' + escapeAttr(a.image_url) + '" alt="" loading="lazy" onerror="this.parentElement.style.display=\'none\'"></div>';
      }

      return '<div class="article-card" data-category="' + escapeAttr(category) + '">'
        + '<div class="card-rank">' + rank + '</div>'
        + thumbnailHtml
        + '<div class="card-body">'
        + '<div class="card-title-row">'
        + '<h3 class="card-title"><a href="' + detailHref + '">' + escapeHtml(a.title) + '</a></h3>'
        + '<button class="' + bookmarkClass + '" data-article-id="' + a.id + '" onclick="toggleCardBookmark(this)" title="收藏">💡</button>'
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

  // --- Bookmark functions (v0.7.0) ---

  async function fetchBookmarkStatus(articles) {
    if (!articles || articles.length === 0) return;
    var ids = articles.map(function(a) { return a.id; }).join(',');
    try {
      var res = await fetch(API_BASE + '/bookmarks/status?ids=' + ids, {
        headers: { 'X-User-Token': getUserToken() },
      });
      var data = await res.json();
      if (data.bookmarks) {
        // Merge into state
        for (var key in data.bookmarks) {
          bookmarkState[key] = data.bookmarks[key];
        }
        updateBookmarkIcons();
      }
    } catch (err) {
      // Silent fail
    }
  }

  function updateBookmarkIcons() {
    var buttons = document.querySelectorAll('.bookmark-btn');
    buttons.forEach(function(btn) {
      var id = btn.dataset.articleId;
      var isBookmarked = !!bookmarkState[id];
      btn.textContent = isBookmarked ? '❤️' : '🤍';
      if (isBookmarked) {
        btn.classList.add('bookmarked');
      } else {
        btn.classList.remove('bookmarked');
      }
    });
  }

  // Global toggle function for card bookmark buttons
  window.toggleCardBookmark = function(btn) {
    var articleId = btn.dataset.articleId;
    var isBookmarked = !!bookmarkState[articleId];

    if (isBookmarked) {
      // Unbookmark
      fetch(API_BASE + '/bookmarks/' + articleId, {
        method: 'DELETE',
        headers: authHeaders(),
      }).then(function(res) { return res.json(); }).then(function(data) {
        if (!data.error) {
          bookmarkState[articleId] = false;
          btn.textContent = '🤍';
          btn.classList.remove('bookmarked');
        }
      }).catch(function() {});
    } else {
      // Bookmark
      fetch(API_BASE + '/bookmarks', {
        method: 'POST',
        headers: authHeaders(),
        body: JSON.stringify({ article_id: parseInt(articleId) }),
      }).then(function(res) { return res.json(); }).then(function(data) {
        if (!data.error) {
          bookmarkState[articleId] = true;
          btn.textContent = '❤️';
          btn.classList.add('bookmarked', 'bookmark-pop');
          setTimeout(function() { btn.classList.remove('bookmark-pop'); }, 500);
        }
      }).catch(function() {});
    }
  };

})();
