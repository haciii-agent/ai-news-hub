// AI News Hub v1.2.0 — Auth Module
// Token management, API calls, nav UI
const Auth = (function() {
  'use strict';

  const JWT_KEY = 'jwt_token';
  const JWT_EXPIRY_KEY = 'jwt_expiry';
  const ANON_TOKEN_KEY = 'user_token';
  const API_BASE = '/api/v1';

  let currentUser = null;

  // --- Helpers ---
  function escapeHtml(str) {
    if (!str) return '';
    var d = document.createElement('div');
    d.textContent = str;
    return d.innerHTML;
  }

  // --- Token Management ---
  function getJWT() {
    return localStorage.getItem(JWT_KEY);
  }

  function setJWT(token, expiresIn) {
    localStorage.setItem(JWT_KEY, token);
    var expiry = Date.now() + (expiresIn || 604800) * 1000;
    localStorage.setItem(JWT_EXPIRY_KEY, String(expiry));
  }

  function clearJWT() {
    localStorage.removeItem(JWT_KEY);
    localStorage.removeItem(JWT_EXPIRY_KEY);
    currentUser = null;
  }

  function isJWTExpired() {
    var expiry = localStorage.getItem(JWT_EXPIRY_KEY);
    if (!expiry) return true;
    return Date.now() > parseInt(expiry, 10);
  }

  function getAnonToken() {
    var token = localStorage.getItem(ANON_TOKEN_KEY);
    if (!token) {
      token = crypto.randomUUID();
      localStorage.setItem(ANON_TOKEN_KEY, token);
    }
    return token;
  }

  function clearAnonToken() {
    localStorage.removeItem(ANON_TOKEN_KEY);
  }

  // --- Auth State ---
  function isLoggedIn() {
    return !!getJWT() && !isJWTExpired();
  }

  function getCurrentUser() {
    return currentUser;
  }

  // --- Request Headers ---
  function authHeaders() {
    var headers = { 'Content-Type': 'application/json' };
    if (isLoggedIn()) {
      headers['Authorization'] = 'Bearer ' + getJWT();
    } else {
      headers['X-User-Token'] = getAnonToken();
    }
    return headers;
  }

  // --- API Calls ---
  async function login(loginVal, password) {
    var res = await fetch(API_BASE + '/auth/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'X-User-Token': getAnonToken() },
      body: JSON.stringify({ login: loginVal, password: password })
    });
    var data = await res.json();
    if (data.error) throw new Error(data.message || '登录失败');
    setJWT(data.token.access_token, data.token.expires_in);
    currentUser = data.user;
    clearAnonToken();
    return data;
  }

  async function register(username, email, password) {
    var res = await fetch(API_BASE + '/auth/register', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'X-User-Token': getAnonToken() },
      body: JSON.stringify({ username: username, email: email, password: password })
    });
    var data = await res.json();
    if (data.error) throw new Error(data.message || '注册失败');
    setJWT(data.token.access_token, data.token.expires_in);
    currentUser = data.user;
    clearAnonToken();
    return data;
  }

  function logout() {
    if (isLoggedIn()) {
      fetch(API_BASE + '/auth/logout', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', 'Authorization': 'Bearer ' + getJWT() }
      }).catch(function() {});
    }
    clearJWT();
    updateNavUI();
    var currentPath = window.location.pathname;
    if (currentPath === '/profile.html' || currentPath === '/admin.html') {
      window.location.href = '/';
    }
  }

  async function fetchMe() {
    var res = await fetch(API_BASE + '/auth/me', {
      headers: authHeaders()
    });
    return await res.json();
  }

  async function checkUsername(username) {
    var res = await fetch(API_BASE + '/auth/check-username?username=' + encodeURIComponent(username));
    return await res.json();
  }

  async function checkEmail(email) {
    var res = await fetch(API_BASE + '/auth/check-email?email=' + encodeURIComponent(email));
    return await res.json();
  }

  async function updatePassword(oldPwd, newPwd) {
    var res = await fetch(API_BASE + '/user/password', {
      method: 'PUT',
      headers: authHeaders(),
      body: JSON.stringify({ old_password: oldPwd, new_password: newPwd })
    });
    var data = await res.json();
    if (data.error) throw new Error(data.message || '修改密码失败');
    return data;
  }

  // --- Nav UI ---
  function updateNavUI() {
    var navUser = document.getElementById('navUserArea');
    if (!navUser) return;

    if (isLoggedIn() && currentUser) {
      var name = currentUser.username || '...';
      var isAdmin = currentUser.role === 'admin';
      navUser.innerHTML =
        '<div class="user-menu">'
        + '<button class="user-avatar" onclick="Auth.toggleMenu()">' + escapeHtml(name) + '</button>'
        + '<div class="user-dropdown" id="userDropdown" style="display:none">'
        + '<a href="/profile.html">👤 个人中心</a>'
        + (isAdmin ? '<a href="/admin.html">🔧 管理后台</a>' : '')
        + '<button onclick="Auth.logout()">🚪 退出登录</button>'
        + '</div></div>';
    } else {
      navUser.innerHTML =
        '<a href="/login.html" class="header-nav-link">登录</a>'
        + '<a href="/login.html?tab=register" class="header-nav-link">注册</a>';
    }
  }

  function toggleMenu() {
    var dd = document.getElementById('userDropdown');
    if (dd) {
      dd.style.display = dd.style.display === 'none' ? '' : 'none';
    }
  }

  // --- Init ---
  async function init() {
    if (isLoggedIn()) {
      try {
        var data = await fetchMe();
        if (data.anonymous || data.guest || data.error) {
          clearJWT();
        } else {
          currentUser = data.user || data;
        }
      } catch (e) {
        clearJWT();
      }
    }
    updateNavUI();
    document.addEventListener('click', function(e) {
      if (!e.target.closest('.user-menu')) {
        var dd = document.getElementById('userDropdown');
        if (dd) dd.style.display = 'none';
      }
    });
  }

  return {
    init: init,
    isLoggedIn: isLoggedIn,
    getJWT: getJWT,
    setJWT: setJWT,
    clearJWT: clearJWT,
    authHeaders: authHeaders,
    login: login,
    register: register,
    logout: logout,
    fetchMe: fetchMe,
    checkUsername: checkUsername,
    checkEmail: checkEmail,
    updatePassword: updatePassword,
    updateNavUI: updateNavUI,
    toggleMenu: toggleMenu,
    getCurrentUser: getCurrentUser,
    getAnonToken: getAnonToken,
    clearAnonToken: clearAnonToken
  };
})();
