import './login-form.js';
import './admin-nav.js';
import './admin-dashboard.js';
import './settings-form.js';

const STYLES = `
  :host {
    display: block;
    font-family: Inter, "Segoe UI", system-ui, -apple-system, sans-serif;
    font-size: 1rem;
    color: #1f2937;
  }

  *, *::before, *::after { box-sizing: border-box; }

  /* Full-page spinner while checking auth */
  .auth-check {
    display: flex;
    align-items: center;
    justify-content: center;
    min-height: 100vh;
    background: #f4f7fb;
  }
  .spinner {
    width: 2.5rem;
    height: 2.5rem;
    border: 3px solid #d1d5db;
    border-top-color: #0f62fe;
    border-radius: 50%;
    animation: spin 0.8s linear infinite;
  }
  @keyframes spin { to { transform: rotate(360deg); } }

  /* App shell: nav sidebar + content area */
  .shell {
    display: flex;
    min-height: 100vh;
    background: #f4f7fb;
  }

  .content {
    flex: 1 1 auto;
    min-width: 0;
    overflow-y: auto;
  }
`;

/**
 * `<x-admin-app>` — top-level orchestrator for the admin SPA.
 *
 * Internal states: `checking-auth` | `logged-out` | `logged-in`.
 * Handles session verification, client-side routing, login and logout.
 */
class XAdminApp extends HTMLElement {
  #root;
  #state  = 'checking-auth';
  #user   = null;
  #route  = '';
  #popHandler;

  connectedCallback() {
    this.#root = this.attachShadow({ mode: 'open' });
    this.#root.innerHTML = `<style>${STYLES}</style><div class="auth-check"><span class="spinner"></span></div>`;

    this.#route = window.location.pathname;

    // Listen for browser back/forward navigation
    this.#popHandler = () => { this.#route = window.location.pathname; this.#renderPanel(); };
    window.addEventListener('popstate', this.#popHandler);

    this.#checkAuth();
  }

  disconnectedCallback() {
    window.removeEventListener('popstate', this.#popHandler);
  }

  // ── Auth helpers ──────────────────────────────────────────────────────────

  async #checkAuth() {
    try {
      const res = await fetch('/auth/me', { credentials: 'same-origin' });
      if (res.ok) {
        this.#user = await res.json();
        this.#setState('logged-in');
      } else {
        this.#setState('logged-out');
      }
    } catch {
      this.#setState('logged-out');
    }
  }

  async #logout() {
    try {
      await fetch('/auth/logout', { method: 'POST', credentials: 'same-origin' });
    } catch { /* ignore */ }
    this.#user = null;
    this.#setState('logged-out');
  }

  // ── State management ─────────────────────────────────────────────────────

  #setState(state) {
    this.#state = state;
    switch (state) {
      case 'checking-auth': this.#renderChecking(); break;
      case 'logged-out':    this.#renderLogin(); break;
      case 'logged-in':     this.#renderShell(); break;
    }
  }

  // ── Render helpers ────────────────────────────────────────────────────────

  #renderChecking() {
    this.#root.innerHTML = `
      <style>${STYLES}</style>
      <div class="auth-check"><span class="spinner"></span></div>
    `;
  }

  #renderLogin() {
    this.#root.innerHTML = `<style>${STYLES}</style><x-login-form></x-login-form>`;
    this.#root.querySelector('x-login-form').addEventListener('login-success', e => {
      this.#user = e.detail.user;
      this.#navigate('/admin/dashboard');
      this.#setState('logged-in');
    });
  }

  #renderShell() {
    this.#root.innerHTML = `
      <style>${STYLES}</style>
      <div class="shell">
        <x-admin-nav
          active-route="${this.#route}"
          user-email="${this.#user?.email ?? ''}"
        ></x-admin-nav>
        <div class="content" id="panel"></div>
      </div>
    `;

    const nav = this.#root.querySelector('x-admin-nav');
    nav.addEventListener('nav-clicked', e => this.#navigate(e.detail.href));
    nav.addEventListener('nav-logout',  () => this.#logout());

    this.#renderPanel();
  }

  #renderPanel() {
    const panel = this.#root.getElementById('panel');
    if (!panel) return;

    // Update nav active route without a full re-render
    const nav = this.#root.querySelector('x-admin-nav');
    if (nav) nav.setAttribute('active-route', this.#route);

    const route = this.#route;
    let tag;
    if (route === '/admin' || route === '/admin/' || route === '/admin/dashboard') {
      tag = 'x-admin-dashboard';
    } else if (route === '/admin/settings') {
      tag = 'x-settings-form';
    } else {
      // Default for unknown routes
      tag = 'x-admin-dashboard';
    }

    // Avoid unnecessary DOM churn if the panel is already the right element
    if (panel.firstElementChild?.tagName?.toLowerCase() === tag) return;

    panel.innerHTML = '';
    panel.appendChild(document.createElement(tag));
  }

  #navigate(href) {
    if (window.location.pathname !== href) {
      history.pushState(null, '', href);
    }
    this.#route = href;
    this.#renderPanel();
  }
}

customElements.define('x-admin-app', XAdminApp);
