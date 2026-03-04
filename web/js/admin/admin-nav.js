const STYLES = `
  :host {
    display: flex;
    flex-direction: column;
    width: 220px;
    min-width: 220px;
    background: #1e293b;
    color: #e2e8f0;
    font-family: Inter, "Segoe UI", system-ui, -apple-system, sans-serif;
    font-size: 0.9375rem;
    min-height: 100vh;
    padding: 0;
  }

  *, *::before, *::after { box-sizing: border-box; }

  .brand {
    padding: 1.25rem 1.25rem 1rem;
    font-size: 1.125rem;
    font-weight: 700;
    color: #f8fafc;
    letter-spacing: -0.01em;
    border-bottom: 1px solid rgba(255,255,255,0.08);
  }

  nav {
    flex: 1 1 auto;
    padding: 0.75rem 0;
  }

  .nav-item {
    display: flex;
    align-items: center;
    gap: 0.625rem;
    padding: 0.65rem 1.25rem;
    color: #94a3b8;
    text-decoration: none;
    cursor: pointer;
    border: none;
    background: none;
    font: inherit;
    font-size: 0.9375rem;
    width: 100%;
    text-align: left;
    border-radius: 0;
    transition: background 120ms, color 120ms;
    outline: none;
  }
  .nav-item:hover { background: rgba(255,255,255,0.07); color: #f1f5f9; }
  .nav-item[aria-current="page"],
  .nav-item.active {
    background: rgba(15,98,254,0.25);
    color: #60a5fa;
    font-weight: 600;
  }
  .nav-icon {
    width: 1.1rem;
    flex-shrink: 0;
    opacity: 0.8;
  }

  .footer {
    padding: 1rem 1.25rem;
    border-top: 1px solid rgba(255,255,255,0.08);
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
  }
  .user-email {
    font-size: 0.8125rem;
    color: #64748b;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .btn-logout {
    padding: 0.5rem 0.75rem;
    background: transparent;
    border: 1px solid rgba(255,255,255,0.15);
    border-radius: 6px;
    color: #94a3b8;
    font: inherit;
    font-size: 0.875rem;
    cursor: pointer;
    transition: background 120ms, color 120ms;
  }
  .btn-logout:hover { background: rgba(255,255,255,0.07); color: #f1f5f9; }
`;

const NAV_ITEMS = [
  { label: 'Dashboard',     href: '/admin/dashboard', icon: '▦' },
  { label: 'Dienste',       href: '/admin/services',  icon: '⊞' },
  { label: 'Einstellungen', href: '/admin/settings',  icon: '⚙' },
];

const TEMPLATE = document.createElement('template');

/**
 * `<x-admin-nav>` — sidebar navigation for the admin SPA.
 *
 * Observed attributes: `active-route`, `user-email`.
 * Events: `nav-clicked` (detail: { href }), `nav-logout` (no detail).
 */
class XAdminNav extends HTMLElement {
  static get observedAttributes() { return ['active-route', 'user-email']; }

  #root;

  connectedCallback() {
    this.#root = this.attachShadow({ mode: 'open' });
    this.#render();
  }

  attributeChangedCallback() {
    if (this.#root) this.#render();
  }

  get activeRoute() { return this.getAttribute('active-route') || ''; }
  get userEmail()   { return this.getAttribute('user-email')   || ''; }

  #isActive(href) {
    const route = this.activeRoute;
    return route === href || (href === '/admin/dashboard' && (route === '/admin' || route === '/admin/dashboard'));
  }

  #render() {
    const itemsHTML = NAV_ITEMS.map(item => {
      const active = this.#isActive(item.href);
      return `<button
        class="nav-item${active ? ' active' : ''}"
        ${active ? 'aria-current="page"' : ''}
        data-href="${item.href}"
        type="button"
      ><span class="nav-icon" aria-hidden="true">${item.icon}</span>${item.label}</button>`;
    }).join('');

    this.#root.innerHTML = `
      <style>${STYLES}</style>
      <div class="brand">schedio</div>
      <nav>${itemsHTML}</nav>
      <div class="footer">
        ${this.userEmail ? `<div class="user-email" title="${this.userEmail}">${this.userEmail}</div>` : ''}
        <button class="btn-logout" id="logout-btn" type="button">Abmelden</button>
      </div>
    `;

    this.#root.querySelectorAll('.nav-item').forEach(btn => {
      btn.addEventListener('click', () => {
        this.dispatchEvent(new CustomEvent('nav-clicked', {
          detail:   { href: btn.dataset.href },
          bubbles:  true,
          composed: true,
        }));
      });
    });

    this.#root.getElementById('logout-btn').addEventListener('click', () => {
      this.dispatchEvent(new CustomEvent('nav-logout', {
        bubbles:  true,
        composed: true,
      }));
    });
  }
}

customElements.define('x-admin-nav', XAdminNav);
