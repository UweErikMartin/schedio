/**
 * <x-cancel-confirm> — cancellation confirmation dialog.
 *
 * Attributes:
 *   booking-id  — ID of the booking to cancel
 *   token       — HMAC management token
 *
 * Events dispatched:
 *   cancellation-confirmed — DELETE API call succeeded
 *   cancellation-cancelled — user clicked "Abbrechen"
 */

const STYLES = `
  :host { display: block; font-family: var(--font-body, sans-serif); }

  .overlay {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.45);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 1000;
  }

  .dialog {
    background: var(--color-surface, #fff);
    border-radius: 10px;
    padding: 2rem;
    max-width: 440px;
    width: 90%;
    box-shadow: 0 10px 40px rgba(0, 0, 0, 0.2);
  }

  h3 {
    margin: 0 0 0.75rem;
    font-size: 1.1rem;
    color: var(--color-text, #111827);
  }

  p {
    margin: 0 0 1.25rem;
    color: var(--color-text-secondary, #374151);
    font-size: 0.95rem;
    line-height: 1.6;
  }

  .error {
    padding: 0.75rem 1rem;
    border-radius: 6px;
    background: var(--color-danger-bg, #fef2f2);
    border: 1px solid var(--color-danger, #ef4444);
    color: var(--color-danger, #ef4444);
    font-size: 0.9rem;
    margin-bottom: 1rem;
  }

  .actions {
    display: flex;
    gap: 0.75rem;
    flex-wrap: wrap;
  }

  .btn-danger {
    padding: 0.5rem 1.25rem;
    border-radius: 6px;
    border: none;
    background: var(--color-danger, #ef4444);
    color: #fff;
    cursor: pointer;
    font-size: 0.9rem;
  }
  .btn-danger:disabled { opacity: 0.4; cursor: not-allowed; }
  .btn-danger:not(:disabled):hover { opacity: 0.88; }

  .btn-secondary {
    padding: 0.5rem 1.25rem;
    border-radius: 6px;
    border: 1px solid var(--color-border, #d1d5db);
    background: transparent;
    color: var(--color-text, #111827);
    cursor: pointer;
    font-size: 0.9rem;
  }
  .btn-secondary:disabled { opacity: 0.4; cursor: not-allowed; }
  .btn-secondary:not(:disabled):hover { background: var(--color-surface-secondary, #f9fafb); }

  .spinner {
    display: inline-block;
    width: 14px;
    height: 14px;
    border: 2px solid rgba(255,255,255,0.4);
    border-top-color: #fff;
    border-radius: 50%;
    animation: spin 0.7s linear infinite;
    vertical-align: middle;
    margin-right: 0.4rem;
  }

  @keyframes spin { to { transform: rotate(360deg); } }
`;

class XCancelConfirm extends HTMLElement {
  static get observedAttributes() {
    return ['booking-id', 'token'];
  }

  #loading = false;
  #errorMsg = '';

  constructor() {
    super();
    this.attachShadow({ mode: 'open' });
  }

  get #bookingId() { return this.getAttribute('booking-id') || ''; }
  get #token()     { return this.getAttribute('token')      || ''; }

  connectedCallback() {
    this.#render();
  }

  attributeChangedCallback() {
    if (this.isConnected) this.#render();
  }

  #render() {
    const root = this.shadowRoot;
    root.innerHTML = '';

    const style = document.createElement('style');
    style.textContent = STYLES;
    root.appendChild(style);

    const overlay = document.createElement('div');
    overlay.className = 'overlay';

    const dialog = document.createElement('div');
    dialog.className = 'dialog';
    dialog.setAttribute('role', 'dialog');
    dialog.setAttribute('aria-modal', 'true');
    dialog.setAttribute('aria-labelledby', 'cancel-title');

    const heading = document.createElement('h3');
    heading.id = 'cancel-title';
    heading.textContent = 'Termin stornieren';

    const text = document.createElement('p');
    text.textContent =
      'Möchten Sie diesen Termin wirklich stornieren? Diese Aktion kann nicht rückgängig gemacht werden.';

    dialog.appendChild(heading);
    dialog.appendChild(text);

    if (this.#errorMsg) {
      const err = document.createElement('div');
      err.className = 'error';
      err.textContent = this.#errorMsg;
      dialog.appendChild(err);
    }

    const actions = document.createElement('div');
    actions.className = 'actions';

    const confirmBtn = document.createElement('button');
    confirmBtn.id = 'confirm-btn';
    confirmBtn.className = 'btn-danger';
    confirmBtn.disabled = this.#loading;
    if (this.#loading) {
      confirmBtn.innerHTML = '<span class="spinner"></span>Wird storniert…';
    } else {
      confirmBtn.textContent = 'Stornieren bestätigen';
    }
    confirmBtn.addEventListener('click', () => this.#cancel());

    const abortBtn = document.createElement('button');
    abortBtn.className = 'btn-secondary';
    abortBtn.disabled = this.#loading;
    abortBtn.textContent = 'Abbrechen';
    abortBtn.addEventListener('click', () => {
      this.dispatchEvent(new CustomEvent('cancellation-cancelled', { bubbles: true, composed: true }));
    });

    actions.appendChild(confirmBtn);
    actions.appendChild(abortBtn);
    dialog.appendChild(actions);
    overlay.appendChild(dialog);
    root.appendChild(overlay);
  }

  async #cancel() {
    this.#loading  = true;
    this.#errorMsg = '';
    this.#render();

    try {
      const tz = encodeURIComponent(Intl.DateTimeFormat().resolvedOptions().timeZone);
      const res = await fetch(
        `/api/v1/bookings/${encodeURIComponent(this.#bookingId)}?token=${encodeURIComponent(this.#token)}&tz=${tz}`,
        { method: 'DELETE' }
      );
      if (!res.ok) {
        const text = await res.text().catch(() => res.statusText);
        this.#errorMsg = `Stornierung fehlgeschlagen (${res.status}): ${text}`;
        this.#loading  = false;
        this.#render();
        return;
      }
      this.dispatchEvent(new CustomEvent('cancellation-confirmed', { bubbles: true, composed: true }));
    } catch {
      this.#errorMsg = 'Verbindung fehlgeschlagen. Bitte erneut versuchen.';
      this.#loading  = false;
      this.#render();
    }
  }
}

customElements.define('x-cancel-confirm', XCancelConfirm);
