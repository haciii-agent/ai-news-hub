// AI News Hub v1.3.0 — Store: 共享状态 + API + 工具函数
// 所有页面共享这一个文件，替换原来的 auth.js + app.js 的重复代码
// 使用: const S = window.aiNewsHub; 在页面 script 中调用

(function (global) {
  'use strict';

  const API_BASE = '/api/v1';
  const JWT_KEY = 'jwt_token';
  const JWT_EXPIRY_KEY = 'jwt_expiry';
  const ANON_TOKEN_KEY = 'user_token';
  const THEME_KEY = 'theme';

  // ─── Token ─────────────────────────────────────────────────────────────────

  function getJWT() { return localStorage.getItem(JWT_KEY); }
  function setJWT(token, expiresIn) {
    localStorage.setItem(JWT_KEY, token);
    localStorage.setItem(JWT_EXPIRY_KEY, String(Date.now() + (expiresIn || 604800) * 1000));
  }
  function clearJWT() {
    localStorage.removeItem(JWT_KEY);
    localStorage.removeItem(JWT_EXPIRY_KEY);
    _currentUser = null;
    _isLoggedIn = false;
    updateNavUI();
  }
  function isJWTExpired() {
    const e = localStorage.getItem(JWT_EXPIRY_KEY);
    return !e || Date.now() > parseInt(e, 10);
  }
  function isLoggedIn() { return !!getJWT() && !isJWTExpired(); }
  function getAnonToken() {
    let t = localStorage.getItem(ANON_TOKEN_KEY);
    if (!t) { t = crypto.randomUUID(); localStorage.setItem(ANON_TOKEN_KEY, t); }
    return t;
  }

  // ─── In-memory state ───────────────────────────────────────────────────────

  let _currentUser = null;
  let _isLoggedIn = false;
  let _collectStatus = null;

  // ─── Auth headers ─────────────────────────────────────────────────────────

  function authHeaders() {
    const h = { 'Content-Type': 'application/json' };
    if (isLoggedIn()) h['Authorization'] = 'Bearer ' + getJWT();
    else h['X-User-Token'] = getAnonToken();
    return h;
  }

  // ─── API fetch ─────────────────────────────────────────────────────────────

  async function apiFetch(path, options = {}) {
    const res = await fetch(API_BASE + path, {
      ...options,
      headers: { ...authHeaders(), ...(options.headers || {}) },
    });
    const data = await res.json().catch(() => ({}));
    if (res.status === 401 && isJWTExpired()) { clearJWT(); }
    return data;
  }

  // ─── Auth API ─────────────────────────────────────────────────────────────

  async function login(loginVal, password) {
    const res = await fetch(API_BASE + '/auth/login', {
      method: 'POST', headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ login: loginVal, password }),
    });
    return res.json();
  }

  async function register(username, email, password) {
    const res = await fetch(API_BASE + '/auth/register', {
      method: 'POST', headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username, email, password }),
    });
    return res.json();
  }

  async function fetchMe() {
    if (!isLoggedIn()) return { anonymous: true };
    return apiFetch('/user/profile');
  }

  async function logout() {
    clearJWT();
    showToast('已退出登录');
  }

  async function initAuth() {
    if (isLoggedIn()) {
      try {
        const d = await fetchMe();
        if (d.error || d.anonymous || d.guest) { clearJWT(); }
        else { _currentUser = d.user || d; _isLoggedIn = true; }
      } catch { clearJWT(); }
    } else {
      _isLoggedIn = false;
      // Fire-and-forget: registers anon token with backend
      fetch(API_BASE + '/user/init', { method: 'POST', headers: authHeaders() }).catch(() => {});
    }
    updateNavUI();
  }

  // ─── Nav UI (更新所有页面的导航栏) ─────────────────────────────────────────

  function updateNavUI() {
    document.querySelectorAll('.nav-user-area').forEach(el => {
      if (!el) return;
      if (_isLoggedIn && _currentUser) {
        const name = escapeHtml(_currentUser.username || '...');
        const isAdmin = _currentUser.role === 'admin';
        el.innerHTML = `<div class="user-menu">
          <button class="user-avatar" onclick="aiNewsHub.toggleMenu()">${name}</button>
          <div class="user-dropdown" id="userDropdown" style="display:none">
            <a href="/profile.html">👤 个人中心</a>
            ${isAdmin ? '<a href="/admin.html">🔧 管理后台</a>' : ''}
            <button onclick="aiNewsHub.logout()">🚪 退出登录</button>
          </div></div>`;
      } else {
        el.innerHTML = '<a href="/login.html" class="header-nav-link">登录</a>'
          + '<a href="/login.html?tab=register" class="header-nav-link">注册</a>';
      }
    });

    // Close dropdowns on outside click
    document.removeEventListener('click', _docClickHandler);
    document.addEventListener('click', _docClickHandler);
  }

  const _docClickHandler = (e) => {
    if (!e.target.closest('.user-menu')) {
      document.querySelectorAll('.user-dropdown').forEach(d => { d.style.display = 'none'; });
    }
  };

  function toggleMenu() {
    const dd = document.getElementById('userDropdown');
    if (dd) dd.style.display = dd.style.display === 'none' ? '' : 'none';
  }

  // ─── Toast ────────────────────────────────────────────────────────────────

  function showToast(msg, duration = 2000) {
    let t = document.getElementById('toast');
    if (!t) {
      t = document.createElement('div');
      t.id = 'toast';
      t.style.cssText = 'position:fixed;bottom:60px;left:50%;transform:translateX(-50%);'
        + 'background:#333;color:#fff;padding:8px 20px;border-radius:20px;'
        + 'z-index:9999;font-size:14px;opacity:0;transition:opacity .3s;pointer-events:none';
      document.body.appendChild(t);
    }
    t.textContent = msg;
    t.style.opacity = '1';
    clearTimeout(t._timer);
    t._timer = setTimeout(() => { t.style.opacity = '0'; }, duration);
  }

  // ─── Theme ────────────────────────────────────────────────────────────────

  function initTheme() {
    const saved = localStorage.getItem(THEME_KEY) || 'light';
    applyTheme(saved);
  }

  function applyTheme(theme) {
    document.documentElement.setAttribute('data-theme', theme);
    localStorage.setItem(THEME_KEY, theme);
    // Update theme toggle buttons
    document.querySelectorAll('.theme-toggle').forEach(btn => {
      btn.textContent = theme === 'dark' ? '☀️' : '🌙';
      btn.title = theme === 'dark' ? '切换亮色' : '切换暗色';
    });
  }

  function toggleTheme() {
    const next = (document.documentElement.getAttribute('data-theme') || 'light') === 'light' ? 'dark' : 'light';
    applyTheme(next);
  }

  // ─── Clock ────────────────────────────────────────────────────────────────

  function startClock(elId) {
    function tick() {
      const el = elId ? document.getElementById(elId) : document.querySelector('.header-clock');
      if (!el) { setTimeout(tick, 1000); return; }
      el.textContent = new Date().toLocaleString('zh-CN', {
        month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit'
      });
    }
    tick();
    setInterval(tick, 30000);
  }

  // ─── Collect Status ────────────────────────────────────────────────────────

  async function fetchCollectStatus() {
    try {
      _collectStatus = await apiFetch('/collect/status');
      updateCollectBadge();
    } catch {}
  }

  function updateCollectBadge() {
    if (!_collectStatus) return;
    document.querySelectorAll('.collect-status-badge').forEach(el => {
      el.textContent = _collectStatus.total_collected + '篇';
      el.className = 'collect-status-badge ' + (_collectStatus.status || '');
    });
  }

  // ─── Helpers ─────────────────────────────────────────────────────────────

  function escapeHtml(str) {
    if (!str) return '';
    const d = document.createElement('div');
    d.textContent = str;
    return d.innerHTML;
  }

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

  const KNOWN_CATEGORIES = [
    '综合资讯', 'AI/ML', '科技前沿', '商业动态',
    '开源生态', '学术研究', '政策监管', '产品发布',
  ];

  function getCategoryClass(cat) {
    return CATEGORY_COLORS[cat] || 'tag-general';
  }

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

  function formatFullTime(ts) {
    if (!ts) return '';
    return new Date(ts * 1000).toLocaleString('zh-CN');
  }

  // Record read history
  function recordRead(article) {
    const h = { ...authHeaders() };
    delete h['Content-Type'];
    fetch(API_BASE + '/history', {
      method: 'POST', headers: h,
      body: JSON.stringify({ article_id: article.id, title: article.title, source: article.source }),
    }).catch(() => {});
  }

  // ─── Expose ───────────────────────────────────────────────────────────────

  const api = {
    get: (path) => apiFetch(path),
    post: (path, body) => apiFetch(path, { method: 'POST', body: JSON.stringify(body) }),
    put: (path, body) => apiFetch(path, { method: 'PUT', body: JSON.stringify(body) }),
    del: (path) => apiFetch(path, { method: 'DELETE' }),
  };

  // ─── Auth compatibility shim (for existing pages using Auth.*) ──────────────
  // Deprecated: use aiNewsHub.* instead. Will be removed in v2.0.
  const _authCompat = {
    init: () => initAuth(),
    isLoggedIn: () => isLoggedIn(),
    getJWT: () => getJWT(),
    setJWT: (t, e) => setJWT(t, e),
    clearJWT: () => clearJWT(),
    logout: () => logout(),
    authHeaders: () => authHeaders(),
    login: (u, p) => login(u, p),
    register: (n, e, p) => register(n, e, p),
    fetchMe: () => fetchMe(),
    getCurrentUser: () => _currentUser,
    checkUsername: (n) => apiFetch('/auth/check?username=' + encodeURIComponent(n)),
    checkEmail: (e) => apiFetch('/auth/check?email=' + encodeURIComponent(e)),
    toggleMenu: () => toggleMenu(),
    updateNavUI: () => updateNavUI(),
    updatePassword: (o, n) => apiFetch('/user/password', { method: 'PUT', body: JSON.stringify({ old_password: o, new_password: n }) }),
    getAnonToken: () => getAnonToken(),
    clearAnonToken: () => { localStorage.removeItem(ANON_TOKEN_KEY); },
    getJWTExpiry: () => localStorage.getItem(JWT_EXPIRY_KEY),
  };
  global.Auth = _authCompat;

  global.aiNewsHub = {
    // Token
    getJWT, setJWT, clearJWT, isLoggedIn, getAnonToken,
    // Auth
    login, register, fetchMe, logout, initAuth,
    // State getters
    getCurrentUser: () => _currentUser,
    isUserLoggedIn: () => _isLoggedIn,
    getCollectStatus: () => _collectStatus,
    // API
    api, apiFetch, authHeaders,
    // UI
    showToast, updateNavUI, toggleMenu,
    // Theme
    initTheme, applyTheme, toggleTheme,
    // Clock
    startClock,
    // Collect
    fetchCollectStatus, updateCollectBadge,
    // Helpers
    escapeHtml, formatTime, formatFullTime, getCategoryClass,
    CATEGORY_COLORS, KNOWN_CATEGORIES,
    recordRead,
    API_BASE,
  };

})(window);
