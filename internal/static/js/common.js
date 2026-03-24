// AI News Hub v1.3.0 — Common: 共享 Header/Footer 渲染
// 每个页面只需引入 store.js + common.js，然后写自己的页面逻辑即可
// 不再有重复的 navUserArea / footer-copyright / theme-toggle 代码

(function (global) {
  'use strict';

  const S = global.aiNewsHub;

  // ─── Header ─────────────────────────────────────────────────────────────────

  function renderHeader() {
    const header = document.querySelector('.header');
    if (!header) return;

    // Build header right elements (excluding logo and clock which are static in HTML)
    const right = header.querySelector('.header-right');
    if (!right) return;

    // Remove any existing dynamic elements (except static nav links)
    right.querySelectorAll('.dynamic-el').forEach(el => el.remove());

    function el(tag, attrs, children) {
      const e = document.createElement(tag);
      if (attrs) Object.assign(e, attrs);
      if (children) children.forEach(c => e.appendChild(typeof c === 'string' ? document.createTextNode(c) : c));
      return e;
    }

    // Collect status badge
    const badge = el('div', { className: 'collect-status-badge', id: 'collectStatusBadge', style: 'display:none' });

    // Theme toggle
    const themeBtn = el('button', {
      className: 'theme-toggle dynamic-el', id: 'themeToggle',
      onclick: () => S.toggleTheme(),
    });
    themeBtn.title = '切换主题';
    themeBtn.textContent = (document.documentElement.getAttribute('data-theme') || 'light') === 'dark' ? '☀️' : '🌙';

    // Nav user area (placeholder — will be updated by updateNavUI)
    const navUser = el('div', { className: 'nav-user-area dynamic-el', id: 'navUserArea' });

    // Prepend badge before nav links (insert at position 0 relative to right.children)
    right.insertBefore(badge, right.firstChild);
    right.appendChild(themeBtn);
    right.appendChild(navUser);
  }

  // ─── Footer ─────────────────────────────────────────────────────────────────

  function renderFooter() {
    document.querySelectorAll('.footer-copyright').forEach(el => {
      el.textContent = 'AI News Hub v1.3.0 · 用心搭建 ⚡';
    });
  }

  // ─── Init page ─────────────────────────────────────────────────────────────

  function initPage() {
    S.initTheme();
    S.initAuth();
    S.startClock();
    S.fetchCollectStatus();
  }

  // ─── Shared event bindings ──────────────────────────────────────────────────

  function bindGlobalEvents() {
    // Keyboard shortcuts
    document.addEventListener('keyup', (e) => {
      if (e.key === '/' && !e.target.matches('input,textarea')) {
        e.preventDefault();
        const si = document.getElementById('searchInput') || document.querySelector('.search-input');
        if (si) { si.focus(); si.scrollIntoView({ behavior: 'smooth', block: 'center' }); }
      }
    });
  }

  // ─── Export ───────────────────────────────────────────────────────────────

  global.aiNewsCommon = {
    renderHeader,
    renderFooter,
    initPage,
    bindGlobalEvents,
  };

})(window);
