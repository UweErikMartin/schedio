/**
 * <x-booking-card> — displays a single booking summary with action buttons.
 *
 * JS properties:
 *   booking   — Booking object from the API
 *   currency  — ISO 4217 code (default "EUR")
 *
 * Events dispatched:
 *   reschedule-requested — user clicked "Termin verschieben"
 *   cancel-requested     — user clicked "Termin stornieren"
 */

const STATE_LABELS = {
  reserved:  'Vorgemerkt',
  confirmed: 'Bestätigt',
  cancelled: 'Storniert',
  noshow:    'Nicht erschienen',
};

const STATE_COLORS = {
  reserved:  'var(--color-tentative, #7c3aed)',
  confirmed: 'var(--color-success,   #22c55e)',
  cancelled: 'var(--color-muted,     #6b7280)',
  noshow:    'var(--color-muted,     #6b7280)',
};

const STYLES = `
  :host { display: block; font-family: var(--font-body, sans-serif); }

  .card {
    border: 1px solid var(--color-border, #e5e7eb);
    border-radius: 8px;
    padding: 1.25rem 1.5rem;
    margin-bottom: 1rem;
    background: var(--color-surface, #fff);
  }

  .header {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    margin-bottom: 1rem;
  }

  .service-name {
    font-size: 1.1rem;
    font-weight: 700;
    color: var(--color-text, #111827);
    margin: 0;
  }

  .state-badge {
    padding: 0.25rem 0.75rem;
    border-radius: 9999px;
    font-size: 0.8rem;
    font-weight: 600;
    color: #fff;
    white-space: nowrap;
  }

  .details {
    display: grid;
    gap: 0.4rem;
    font-size: 0.95rem;
    color: var(--color-text-secondary, #374151);
    margin-bottom: 1rem;
  }

  .detail-row {
    display: flex;
    gap: 0.5rem;
  }

  .label {
    font-weight: 600;
    min-width: 120px;
    color: var(--color-text, #111827);
  }

  .actions {
    display: flex;
    gap: 0.75rem;
    flex-wrap: wrap;
    margin-top: 0.5rem;
  }

  .btn-primary {
    padding: 0.5rem 1.25rem;
    border-radius: 6px;
    border: none;
    background: var(--color-primary, #4f46e5);
    color: #fff;
    cursor: pointer;
    font-size: 0.9rem;
  }
  .btn-primary:hover { opacity: 0.88; }

  .btn-danger {
    padding: 0.5rem 1.25rem;
    border-radius: 6px;
    border: 1px solid var(--color-danger, #ef4444);
    background: transparent;
    color: var(--color-danger, #ef4444);
    cursor: pointer;
    font-size: 0.9rem;
  }
  .btn-danger:hover { background: var(--color-danger-bg, #fef2f2); }
`;

class XBookingCard extends HTMLElement {
  #booking = null;
  #currency = 'EUR';

  constructor() {
    super();
    this.attachShadow({ mode: 'open' });
  }

  get booking() { return this.#booking; }
  set booking(val) {
    this.#booking = val;
    if (this.isConnected) this.#render();
  }

  get currency() { return this.#currency; }
  set currency(val) {
    this.#currency = val || 'EUR';
    if (this.isConnected) this.#render();
  }

  connectedCallback() {
    this.#render();
  }

  #fmt(dt) {
    return new Intl.DateTimeFormat(navigator.language, {
      dateStyle: 'medium',
      timeStyle: 'short',
    }).format(new Date(dt));
  }

  #fmtPrice(price) {
    return new Intl.NumberFormat(navigator.language, {
      style: 'currency',
      currency: this.#currency,
    }).format(price ?? 0);
  }

  #render() {
    const root = this.shadowRoot;
    root.innerHTML = '';

    const style = document.createElement('style');
    style.textContent = STYLES;
    root.appendChild(style);

    if (!this.#booking) return;

    const b  = this.#booking;
    const st = b.state || 'reserved';

    const card = document.createElement('div');
    card.className = 'card';

    // ── Header ──────────────────────────────────────────────────────────────
    const header = document.createElement('div');
    header.className = 'header';

    const title = document.createElement('h2');
    title.className = 'service-name';
    title.textContent = b.service?.name ?? '';

    const badge = document.createElement('span');
    badge.className = 'state-badge';
    badge.textContent = STATE_LABELS[st] ?? st;
    badge.style.background = STATE_COLORS[st] ?? 'var(--color-muted, #6b7280)';

    header.appendChild(title);
    header.appendChild(badge);
    card.appendChild(header);

    // ── Details ──────────────────────────────────────────────────────────────
    const details = document.createElement('div');
    details.className = 'details';

    const rows = [
      ['Beginn',   this.#fmt(b.start_at)],
      ['Ende',     this.#fmt(b.end_at)],
      ['Ort',      b.location || '—'],
      ['Kontakt',  b.contact ? `${b.contact.first_name} ${b.contact.last_name}` : '—'],
      ['Preis',    this.#fmtPrice(b.service?.price)],
    ];
    for (const [label, value] of rows) {
      const row = document.createElement('div');
      row.className = 'detail-row';
      row.innerHTML = `<span class="label">${label}:</span><span>${value}</span>`;
      details.appendChild(row);
    }
    card.appendChild(details);

    // ── Action buttons ────────────────────────────────────────────────────────
    const actionable = st === 'reserved' || st === 'confirmed';
    if (actionable) {
      const actions = document.createElement('div');
      actions.className = 'actions';

      const rescheduleBtn = document.createElement('button');
      rescheduleBtn.className = 'btn-primary';
      rescheduleBtn.textContent = 'Termin verschieben';
      rescheduleBtn.addEventListener('click', () => {
        this.dispatchEvent(new CustomEvent('reschedule-requested', { bubbles: true, composed: true }));
      });

      const cancelBtn = document.createElement('button');
      cancelBtn.className = 'btn-danger';
      cancelBtn.textContent = 'Termin stornieren';
      cancelBtn.addEventListener('click', () => {
        this.dispatchEvent(new CustomEvent('cancel-requested', { bubbles: true, composed: true }));
      });

      actions.appendChild(rescheduleBtn);
      actions.appendChild(cancelBtn);
      card.appendChild(actions);
    }

    root.appendChild(card);
  }
}

customElements.define('x-booking-card', XBookingCard);
