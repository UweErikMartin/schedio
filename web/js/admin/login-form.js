const STYLES = `
  :host {
    display: flex;
    align-items: center;
    justify-content: center;
    min-height: 100vh;
    background: #f4f7fb;
    font-family: Inter, "Segoe UI", system-ui, -apple-system, sans-serif;
    font-size: 1rem;
    color: #1f2937;
  }

  *, *::before, *::after { box-sizing: border-box; }

  .card {
    background: #ffffff;
    border: 1px solid #d1d5db;
    border-radius: 12px;
    padding: 2rem 2.25rem;
    width: 100%;
    max-width: 400px;
    box-shadow: 0 2px 12px rgba(0,0,0,0.07);
  }

  h1 {
    margin: 0 0 1.75rem;
    font-size: 1.375rem;
    font-weight: 700;
    color: #1f2937;
  }

  .field {
    display: grid;
    gap: 0.35rem;
    margin-bottom: 1rem;
  }

  label {
    font-size: 0.9rem;
    font-weight: 600;
    color: #374151;
  }

  input[type="email"],
  input[type="password"],
  input[type="text"] {
    width: 100%;
    padding: 0.625rem 0.75rem;
    border: 1px solid #d1d5db;
    border-radius: 8px;
    font: inherit;
    font-size: 0.9375rem;
    background: #fff;
    color: #1f2937;
    outline: none;
    transition: border-color 150ms;
  }
  input:focus { border-color: #0f62fe; box-shadow: 0 0 0 3px rgba(15,98,254,0.15); }
  input:disabled { background: #f9fafb; color: #9ca3af; }

  .password-wrapper {
    position: relative;
    display: flex;
    align-items: center;
  }
  .password-wrapper input { padding-right: 2.75rem; }
  .toggle-pw {
    position: absolute;
    right: 0.5rem;
    background: none;
    border: none;
    cursor: pointer;
    padding: 0.35rem;
    color: #6b7280;
    font-size: 1rem;
    line-height: 1;
    border-radius: 4px;
  }
  .toggle-pw:hover { color: #1f2937; }

  .btn-submit {
    margin-top: 1.25rem;
    width: 100%;
    padding: 0.7rem 1rem;
    background: #0f62fe;
    color: #fff;
    border: none;
    border-radius: 8px;
    font: inherit;
    font-size: 1rem;
    font-weight: 600;
    cursor: pointer;
    transition: background 150ms;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 0.5rem;
    min-height: 2.75rem;
  }
  .btn-submit:hover:not(:disabled) { background: #0b4fd1; }
  .btn-submit:disabled { background: #93c0fd; cursor: default; }

  .spinner {
    width: 1.1rem;
    height: 1.1rem;
    border: 2px solid rgba(255,255,255,0.4);
    border-top-color: #fff;
    border-radius: 50%;
    animation: spin 0.7s linear infinite;
  }
  @keyframes spin { to { transform: rotate(360deg); } }

  .error-banner {
    margin-top: 1rem;
    padding: 0.75rem 1rem;
    background: #fef2f2;
    border: 1px solid #fecaca;
    border-radius: 8px;
    color: #dc2626;
    font-size: 0.9rem;
    display: none;
  }
  .error-banner.visible { display: block; }
`;

const TEMPLATE = document.createElement('template');
TEMPLATE.innerHTML = `
  <style>${STYLES}</style>
  <div class="card">
    <h1>Anmelden</h1>
    <form id="form" novalidate>
      <div class="field">
        <label for="email">E-Mail-Adresse</label>
        <input
          id="email"
          type="email"
          autocomplete="email"
          required
          placeholder="admin@example.com"
        />
      </div>
      <div class="field">
        <label for="password">Passwort</label>
        <div class="password-wrapper">
          <input
            id="password"
            type="password"
            autocomplete="current-password"
            required
          />
          <button type="button" class="toggle-pw" aria-label="Passwort anzeigen" aria-pressed="false">👁</button>
        </div>
      </div>
      <div class="error-banner" id="error" role="alert"></div>
      <button type="submit" class="btn-submit" id="submit-btn">
        <span id="btn-label">Anmelden</span>
      </button>
    </form>
  </div>
`;

/**
 * `<x-login-form>` — admin login form.
 *
 * Dispatches `login-success` with `{ user: { email, role } }` on success.
 */
class XLoginForm extends HTMLElement {
  #root;
  #state = 'idle';
  #form;
  #emailInput;
  #passwordInput;
  #submitBtn;
  #btnLabel;
  #errorBanner;
  #togglePw;

  connectedCallback() {
    this.#root = this.attachShadow({ mode: 'open' });
    this.#root.appendChild(TEMPLATE.content.cloneNode(true));

    this.#form        = this.#root.getElementById('form');
    this.#emailInput  = this.#root.getElementById('email');
    this.#passwordInput = this.#root.getElementById('password');
    this.#submitBtn   = this.#root.getElementById('submit-btn');
    this.#btnLabel    = this.#root.getElementById('btn-label');
    this.#errorBanner = this.#root.getElementById('error');
    this.#togglePw    = this.#root.querySelector('.toggle-pw');

    this.#form.addEventListener('submit', e => { e.preventDefault(); this.#submit(); });
    this.#togglePw.addEventListener('click', () => this.#togglePassword());
  }

  #togglePassword() {
    const isText = this.#passwordInput.type === 'text';
    this.#passwordInput.type = isText ? 'password' : 'text';
    this.#togglePw.setAttribute('aria-pressed', String(!isText));
    this.#togglePw.setAttribute('aria-label', isText ? 'Passwort anzeigen' : 'Passwort verbergen');
  }

  #setState(state, errorMsg = '') {
    this.#state = state;
    const busy = state === 'submitting';
    this.#emailInput.disabled = busy;
    this.#passwordInput.disabled = busy;
    this.#submitBtn.disabled = busy;
    this.#btnLabel.textContent = busy ? '' : 'Anmelden';

    // Spinner inside button during submission
    let spinner = this.#submitBtn.querySelector('.spinner');
    if (busy) {
      if (!spinner) {
        spinner = document.createElement('span');
        spinner.className = 'spinner';
        this.#submitBtn.insertBefore(spinner, this.#btnLabel);
      }
    } else if (spinner) {
      spinner.remove();
    }

    // Error banner
    const showError = state === 'error-credentials' || state === 'error-server';
    this.#errorBanner.textContent = errorMsg;
    this.#errorBanner.classList.toggle('visible', showError);
  }

  async #submit() {
    // Trigger native validation
    if (!this.#form.reportValidity()) return;

    const email    = this.#emailInput.value.trim();
    const password = this.#passwordInput.value;

    this.#setState('submitting');

    try {
      const res = await fetch('/auth/login', {
        method:  'POST',
        headers: { 'Content-Type': 'application/json' },
        body:    JSON.stringify({ email, password }),
      });

      if (res.ok) {
        const user = await res.json();
        this.#setState('idle');
        this.dispatchEvent(new CustomEvent('login-success', {
          detail:   { user },
          bubbles:  true,
          composed: true,
        }));
      } else if (res.status === 401) {
        this.#setState('error-credentials', 'Ungültige E-Mail-Adresse oder Passwort.');
      } else {
        this.#setState('error-server', 'Serverfehler. Bitte versuche es später erneut.');
      }
    } catch {
      this.#setState('error-server', 'Netzwerkfehler. Bitte prüfe deine Verbindung.');
    }
  }
}

customElements.define('x-login-form', XLoginForm);
