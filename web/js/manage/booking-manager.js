/**
 * <x-booking-manager> — orchestrator for the customer booking-management page.
 *
 * Attributes:
 *   booking-id  — booking ID from the ?id= URL parameter
 *   token       — HMAC-signed management token from the ?token= URL parameter
 *
 * Internal states: loading | error | view | reschedule | cancel-confirm | cancelled | success
 */

import './booking-card.js';
import './reschedule-picker.js';
import './cancel-confirm.js';

const STYLES = `
  :host { display: block; font-family: var(--font-body, sans-serif); }

  .container {
    max-width: 640px;
    margin: 2rem auto;
    padding: 0 1rem;
  }

  .spinner-wrap {
    display: flex;
    justify-content: center;
    padding: 3rem 0;
  }

  .spinner {
    width: 40px;
    height: 40px;
    border: 4px solid var(--color-border, #ddd);
    border-top-color: var(--color-primary, #4f46e5);
    border-radius: 50%;
    animation: spin 0.8s linear infinite;
  }

  @keyframes spin { to { transform: rotate(360deg); } }

  .error-banner {
    padding: 1rem;
    border-radius: 6px;
    background: var(--color-danger-bg, #fef2f2);
    border: 1px solid var(--color-danger, #ef4444);
    color: var(--color-danger, #ef4444);
    margin-bottom: 1rem;
  }

  .error-banner p { margin: 0 0 0.5rem; }
  .error-banner button { margin-top: 0.5rem; }

  .banner {
    padding: 1rem;
    border-radius: 6px;
    background: var(--color-success-bg, #f0fdf4);
    border: 1px solid var(--color-success, #22c55e);
    color: var(--color-success, #22c55e);
    margin-bottom: 1rem;
    font-weight: 600;
  }

  button {
    padding: 0.5rem 1.25rem;
    border-radius: 6px;
    border: 1px solid var(--color-primary, #4f46e5);
    background: var(--color-primary, #4f46e5);
    color: #fff;
    cursor: pointer;
    font-size: 0.95rem;
  }
  button:hover { opacity: 0.88; }
`;

class XBookingManager extends HTMLElement {
  static get observedAttributes() {
    return ['booking-id', 'token'];
  }

  #state = 'loading';
  #booking = null;
  #errorMsg = '';
  #networkError = false;

  constructor() {
    super();
    this.attachShadow({ mode: 'open' });
  }

  get booking() { return this.#booking; }

  attributeChangedCallback() {
    if (this.isConnected) this.#load();
  }

  connectedCallback() {
    this.#render();
    this.#load();
  }

  get #bookingId() { return this.getAttribute('booking-id') || ''; }
  get #token()     { return this.getAttribute('token')      || ''; }

  async #load() {
    if (!this.#bookingId || !this.#token) return;
    this.#setState('loading');
    try {
      const res = await fetch(
        `/api/v1/bookings/${encodeURIComponent(this.#bookingId)}?token=${encodeURIComponent(this.#token)}`
      );
      if (res.status === 401 || res.status === 403 || res.status === 404) {
        this.#errorMsg = 'Ungültiger oder abgelaufener Link.';
        this.#networkError = false;
        this.#setState('error');
        return;
      }
      if (!res.ok) {
        this.#errorMsg = `Fehler beim Laden der Buchung (${res.status}).`;
        this.#networkError = true;
        this.#setState('error');
        return;
      }
      this.#booking = await res.json();
      this.#networkError = false;
      this.#setState('view');
    } catch {
      this.#errorMsg = 'Verbindung fehlgeschlagen. Bitte Seite neu laden.';
      this.#networkError = true;
      this.#setState('error');
    }
  }

  #setState(state) {
    this.#state = state;
    this.#render();
  }

  #render() {
    const root = this.shadowRoot;
    root.innerHTML = '';

    const style = document.createElement('style');
    style.textContent = STYLES;
    root.appendChild(style);

    const container = document.createElement('div');
    container.className = 'container';

    switch (this.#state) {
      case 'loading':
        container.innerHTML = '<div class="spinner-wrap"><div class="spinner"></div></div>';
        break;

      case 'error':
        container.appendChild(this.#buildError());
        break;

      case 'view':
        container.appendChild(this.#buildCard());
        break;

      case 'reschedule':
        container.appendChild(this.#buildCard());
        container.appendChild(this.#buildReschedulePicker());
        break;

      case 'cancel-confirm':
        container.appendChild(this.#buildCard());
        container.appendChild(this.#buildCancelConfirm());
        break;

      case 'success': {
        const banner = document.createElement('div');
        banner.className = 'banner';
        banner.textContent = 'Der Termin wurde erfolgreich verschoben.';
        container.appendChild(banner);
        container.appendChild(this.#buildCard());
        break;
      }

      case 'cancelled': {
        const banner = document.createElement('div');
        banner.className = 'banner';
        banner.textContent = 'Ihr Termin wurde storniert.';
        container.appendChild(banner);
        break;
      }
    }

    root.appendChild(container);
  }

  #buildError() {
    const wrap = document.createElement('div');
    wrap.className = 'error-banner';
    const p = document.createElement('p');
    p.textContent = this.#errorMsg;
    wrap.appendChild(p);
    if (this.#networkError) {
      const btn = document.createElement('button');
      btn.textContent = 'Erneut versuchen';
      btn.addEventListener('click', () => this.#load());
      wrap.appendChild(btn);
    }
    return wrap;
  }

  #buildCard() {
    const card = document.createElement('x-booking-card');
    card.booking = this.#booking;
    card.addEventListener('reschedule-requested', () => this.#setState('reschedule'));
    card.addEventListener('cancel-requested',     () => this.#setState('cancel-confirm'));
    return card;
  }

  #buildReschedulePicker() {
    const picker = document.createElement('x-reschedule-picker');
    picker.setAttribute('booking-id', this.#bookingId);
    picker.setAttribute('token',      this.#token);
    picker.setAttribute('service-id', this.#booking?.service?.id || '');
    picker.addEventListener('reschedule-confirmed', (e) => {
      // Prefer the full updated booking returned by the API (contains correct end_at);
      // fall back to patching start_at only if the response was unavailable.
      if (e.detail?.booking) {
        this.#booking = e.detail.booking;
      } else if (this.#booking && e.detail?.newSlot) {
        this.#booking = { ...this.#booking, start_at: e.detail.newSlot };
      }
      this.#setState('success');
    });
    picker.addEventListener('reschedule-cancelled', () => this.#setState('view'));
    return picker;
  }

  #buildCancelConfirm() {
    const dlg = document.createElement('x-cancel-confirm');
    dlg.setAttribute('booking-id', this.#bookingId);
    dlg.setAttribute('token',      this.#token);
    dlg.addEventListener('cancellation-confirmed', () => {
      if (this.#booking) {
        this.#booking = { ...this.#booking, state: 'cancelled' };
      }
      this.#setState('cancelled');
    });
    dlg.addEventListener('cancellation-cancelled', () => this.#setState('view'));
    return dlg;
  }
}

customElements.define('x-booking-manager', XBookingManager);
