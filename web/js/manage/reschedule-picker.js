/**
 * <x-reschedule-picker> — inline date-time picker for rescheduling a booking.
 *
 * Attributes:
 *   booking-id  — ID of the booking to reschedule
 *   token       — HMAC management token
 *   service-id  — forwarded to <x-date-time-picker> for availability queries
 *
 * Events dispatched:
 *   reschedule-confirmed({ newSlot: string }) — API call succeeded
 *   reschedule-cancelled                      — user clicked "Abbrechen"
 */

import '../x-date-time-picker.js';

const STYLES = `
  :host { display: block; font-family: var(--font-body, sans-serif); }

  .wrap {
    border: 1px solid var(--color-border, #e5e7eb);
    border-radius: 8px;
    padding: 1.25rem 1.5rem;
    margin-bottom: 1rem;
    background: var(--color-surface, #fff);
  }

  h3 {
    margin: 0 0 1rem;
    font-size: 1rem;
    font-weight: 700;
    color: var(--color-text, #111827);
  }

  .actions {
    display: flex;
    gap: 0.75rem;
    margin-top: 1rem;
    flex-wrap: wrap;
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
  .btn-primary:disabled { opacity: 0.4; cursor: not-allowed; }
  .btn-primary:not(:disabled):hover { opacity: 0.88; }

  .btn-secondary {
    padding: 0.5rem 1.25rem;
    border-radius: 6px;
    border: 1px solid var(--color-border, #d1d5db);
    background: transparent;
    color: var(--color-text, #111827);
    cursor: pointer;
    font-size: 0.9rem;
  }
  .btn-secondary:hover { background: var(--color-surface-secondary, #f9fafb); }

  .error {
    padding: 0.75rem 1rem;
    border-radius: 6px;
    background: var(--color-danger-bg, #fef2f2);
    border: 1px solid var(--color-danger, #ef4444);
    color: var(--color-danger, #ef4444);
    font-size: 0.9rem;
    margin-bottom: 0.75rem;
  }

  .spinner {
    display: inline-block;
    width: 16px;
    height: 16px;
    border: 2px solid rgba(255,255,255,0.4);
    border-top-color: #fff;
    border-radius: 50%;
    animation: spin 0.7s linear infinite;
    vertical-align: middle;
    margin-right: 0.4rem;
  }

  @keyframes spin { to { transform: rotate(360deg); } }
`;

class XReschedulePicker extends HTMLElement {
  static get observedAttributes() {
    return ['booking-id', 'token', 'service-id'];
  }

  #selectedSlot = null;  // UTC RFC-3339 string from the server, submitted as-is to the API
  #loading = false;
  #errorMsg = '';
  #activeYear = null;
  #activeMonth = null;

  constructor() {
    super();
    this.attachShadow({ mode: 'open' });
  }

  get #bookingId() { return this.getAttribute('booking-id') || ''; }
  get #token()     { return this.getAttribute('token')      || ''; }
  get #serviceId() { return this.getAttribute('service-id') || ''; }

  connectedCallback() {
    this.#render();
    this.#wirePicker();
  }

  attributeChangedCallback() {
    if (this.isConnected) {
      this.#render();
      this.#wirePicker();
    }
  }

  #render() {
    const root = this.shadowRoot;
    root.innerHTML = '';

    const style = document.createElement('style');
    style.textContent = STYLES;
    root.appendChild(style);

    const wrap = document.createElement('div');
    wrap.className = 'wrap';

    const heading = document.createElement('h3');
    heading.textContent = 'Neuen Termin wählen';
    wrap.appendChild(heading);

    if (this.#errorMsg) {
      const err = document.createElement('div');
      err.className = 'error';
      err.textContent = this.#errorMsg;
      wrap.appendChild(err);
    }

    // Date-time picker
    const dp = document.createElement('x-date-time-picker');
    dp.id = 'dt-picker';
    dp.setAttribute('data-availability-endpoint', 'api/v1/availability');
    wrap.appendChild(dp);

    // Action buttons
    const actions = document.createElement('div');
    actions.className = 'actions';

    const confirmBtn = document.createElement('button');
    confirmBtn.id = 'confirm-btn';
    confirmBtn.className = 'btn-primary';
    confirmBtn.disabled = !this.#selectedSlot || this.#loading;
    if (this.#loading) {
      confirmBtn.innerHTML = '<span class="spinner"></span>Wird gespeichert…';
    } else {
      confirmBtn.textContent = 'Bestätigen';
    }
    confirmBtn.addEventListener('click', () => this.#submit());

    const cancelBtn = document.createElement('button');
    cancelBtn.className = 'btn-secondary';
    cancelBtn.textContent = 'Abbrechen';
    cancelBtn.addEventListener('click', () => {
      this.dispatchEvent(new CustomEvent('reschedule-cancelled', { bubbles: true, composed: true }));
    });

    actions.appendChild(confirmBtn);
    actions.appendChild(cancelBtn);
    wrap.appendChild(actions);

    root.appendChild(wrap);

    // Wire date-time picker events after render
    this.#wirePicker();
  }

  #wirePicker() {
    const dp = this.shadowRoot.getElementById('dt-picker');
    if (!dp) return;

    dp.addEventListener('x-date-time-picker-initialized', async (e) => {
      this.#activeYear  = Number(e.detail?.year);
      this.#activeMonth = Number(e.detail?.month);
      await this.#fetchAvailability();
    });

    dp.addEventListener('x-date-time-picker-month-selected', async (e) => {
      this.#activeYear  = Number(e.detail?.year);
      this.#activeMonth = Number(e.detail?.month);
      await this.#fetchAvailability();
    });

    dp.addEventListener('x-date-time-picker-date-selected', (e) => {
      const slots = Array.isArray(e.detail?.timeSlots) ? e.detail.timeSlots : [];
      const date  = e.detail?.date || '';
      if (slots.length > 0 && date) {
        // slots[0] is the server's original UTC RFC-3339 string — use it directly.
        const time = slots[0];
        dp.setAttribute('selected-time', time);
        this.#selectedSlot = time;
      } else {
        this.#selectedSlot = null;
      }
      const btn = this.shadowRoot.getElementById('confirm-btn');
      if (btn) btn.disabled = !this.#selectedSlot || this.#loading;
    });

    // Track manual time-slot selection when the user picks a different hour.
    // selected-time attribute is the server's UTC RFC-3339 string — use it directly.
    const observer = new MutationObserver(() => {
      const t = (dp.getAttribute('selected-time') || '').trim();
      if (t) {
        this.#selectedSlot = t;
      }
      const btn = this.shadowRoot.getElementById('confirm-btn');
      if (btn) btn.disabled = !this.#selectedSlot || this.#loading;
    });
    observer.observe(dp, { attributes: true, attributeFilter: ['selected-time'] });
  }

  async #fetchAvailability() {
    if (!this.#activeYear || !this.#activeMonth || !this.#serviceId) return;
    const dp = this.shadowRoot.getElementById('dt-picker');
    if (!dp) return;
    const monthKey = `${String(this.#activeYear).padStart(4, '0')}-${String(this.#activeMonth).padStart(2, '0')}`;
    try {
      const res = await fetch(
        `api/v1/availability?period=${encodeURIComponent(monthKey)}&service_id=${encodeURIComponent(this.#serviceId)}`
      );
      if (!res.ok) return;
      const payload = await res.json();
      const monthDates =
        payload?.months?.[monthKey] && typeof payload.months[monthKey] === 'object'
          ? payload.months[monthKey]
          : {};
      dp.setAttribute('available-dates', JSON.stringify({
        month: monthKey,
        dates: Object.keys(monthDates).sort(),
        timeSlots: monthDates,
      }));
    } catch {
      // Availability fetch failure is non-fatal; the picker shows no dates.
    }
  }

  async #submit() {
    if (!this.#selectedSlot) return;
    this.#loading  = true;
    this.#errorMsg = '';
    this.#updateLoadingState();

    try {
      const res = await fetch(
        `/api/v1/bookings/${encodeURIComponent(this.#bookingId)}/reschedule?token=${encodeURIComponent(this.#token)}`,
        {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            new_slot: this.#selectedSlot,
            timezone: Intl.DateTimeFormat().resolvedOptions().timeZone,
          }),
        }
      );
      if (!res.ok) {
        const text = await res.text().catch(() => res.statusText);
        this.#errorMsg = res.status === 409
          ? 'Dieser Zeitslot ist leider nicht mehr verfügbar.'
          : `Fehler beim Verschieben (${res.status}): ${text}`;
        this.#loading = false;
        this.#render();
        return;
      }
      const updatedBooking = await res.json().catch(() => null);
      this.dispatchEvent(new CustomEvent('reschedule-confirmed', {
        detail: { newSlot: this.#selectedSlot, booking: updatedBooking },
        bubbles: true,
        composed: true,
      }));
    } catch {
      this.#errorMsg = 'Verbindung fehlgeschlagen. Bitte erneut versuchen.';
      this.#loading = false;
      this.#render();
    }
  }

  #updateLoadingState() {
    const btn = this.shadowRoot.getElementById('confirm-btn');
    if (btn) {
      btn.disabled = true;
      btn.innerHTML = '<span class="spinner"></span>Wird gespeichert…';
    }
    const cancelBtn = this.shadowRoot.querySelector('.btn-secondary');
    if (cancelBtn) cancelBtn.disabled = true;
  }
}

customElements.define('x-reschedule-picker', XReschedulePicker);
