/**
 * <x-booking-manager> â€” orchestrator for the customer booking-management page.
 *
 * Session mode (preferred): attributes session-id + session-token
 *   Loads the whole session via GET /api/v1/sessions/{id}?session_token=
 *   Shows all bookings and allows rescheduling / cancelling the whole session
 *   using <x-multi-date-time-picker>.
 *
 * Booking mode (legacy): attributes booking-id + token
 *   Loads a single booking via GET /api/v1/bookings/{id}?token=
 *
 * Internal states:
 *   loading | error | view | reschedule | cancel-confirm | cancelled | success
 *   session-view | session-reschedule | session-cancelled | session-success
 */

import './booking-card.js';
import './reschedule-picker.js';
import './cancel-confirm.js';
import '../x-multi-date-time-picker.js';

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

  .session-card {
    border: 1px solid var(--color-border, #e5e7eb);
    border-radius: 8px;
    padding: 1.25rem 1.5rem;
    margin-bottom: 1rem;
    background: var(--color-surface, #fff);
  }

  .session-card h2 {
    margin: 0 0 0.5rem;
    font-size: 1.1rem;
    font-weight: 700;
    color: var(--color-text, #111827);
  }

  .session-meta {
    font-size: 0.9rem;
    color: var(--color-text-secondary, #6b7280);
    margin-bottom: 1rem;
  }

  .booking-list {
    list-style: none;
    margin: 0 0 1rem;
    padding: 0;
    display: grid;
    gap: 0.5rem;
  }

  .booking-item {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 0.6rem 0.75rem;
    border-radius: 6px;
    background: var(--color-surface-secondary, #f9fafb);
    border: 1px solid var(--color-border, #e5e7eb);
    font-size: 0.9rem;
  }

  .booking-item-date { color: var(--color-text, #111827); font-weight: 500; }

  .state-badge {
    padding: 0.2rem 0.6rem;
    border-radius: 9999px;
    font-size: 0.75rem;
    font-weight: 600;
    color: #fff;
  }

  .state-reserved  { background: var(--color-tentative, #7c3aed); }
  .state-confirmed { background: var(--color-success,   #22c55e); }
  .state-cancelled { background: var(--color-muted,     #6b7280); }

  .session-hint {
    font-size: 0.875rem;
    color: var(--color-text-secondary, #6b7280);
    margin: 0 0 1rem;
  }

  .actions {
    display: flex;
    gap: 0.75rem;
    flex-wrap: wrap;
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
  button:disabled { opacity: 0.4; cursor: not-allowed; }

  .btn-secondary {
    background: transparent;
    color: var(--color-text, #111827);
    border-color: var(--color-border, #d1d5db);
  }
  .btn-secondary:hover { background: var(--color-surface-secondary, #f9fafb); }

  .btn-danger {
    background: var(--color-danger, #ef4444);
    border-color: var(--color-danger, #ef4444);
  }

  .reschedule-wrap {
    border: 1px solid var(--color-border, #e5e7eb);
    border-radius: 8px;
    padding: 1.25rem 1.5rem;
    margin-bottom: 1rem;
    background: var(--color-surface, #fff);
  }

  .reschedule-wrap h3 {
    margin: 0 0 0.75rem;
    font-size: 1rem;
    font-weight: 700;
    color: var(--color-text, #111827);
  }

  .inline-error {
    padding: 0.6rem 0.75rem;
    border-radius: 6px;
    background: var(--color-danger-bg, #fef2f2);
    border: 1px solid var(--color-danger, #ef4444);
    color: var(--color-danger, #ef4444);
    font-size: 0.875rem;
    margin-bottom: 0.75rem;
  }
`;

const STATE_LABELS = {
  reserved:  'Vorgemerkt',
  confirmed: 'BestÃ¤tigt',
  cancelled: 'Storniert',
  noshow:    'Nicht erschienen',
};

class XBookingManager extends HTMLElement {
  static get observedAttributes() {
    return ['booking-id', 'token', 'session-id', 'session-token'];
  }

  // â”€â”€ Booking-mode state â”€â”€
  #state = 'loading';
  #booking = null;
  #errorMsg = '';
  #networkError = false;

  // â”€â”€ Session-mode state â”€â”€
  #session = null;
  #selectedSlots = [];
  #rescheduleError = '';
  #submitting = false;

  constructor() {
    super();
    this.attachShadow({ mode: 'open' });
  }

  get booking() { return this.#booking; }

  get #bookingId()    { return this.getAttribute('booking-id')    || ''; }
  get #token()        { return this.getAttribute('token')         || ''; }
  get #sessionId()    { return this.getAttribute('session-id')    || ''; }
  get #sessionToken() { return this.getAttribute('session-token') || ''; }
  get #isSessionMode() { return !!this.#sessionId; }

  attributeChangedCallback() {
    if (this.isConnected) this.#load();
  }

  connectedCallback() {
    this.#render();
    this.#load();
  }

  async #load() {
    if (this.#isSessionMode) {
      await this.#loadSession();
    } else {
      await this.#loadBooking();
    }
  }

  // â”€â”€ Booking mode loading â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

  async #loadBooking() {
    if (!this.#bookingId || !this.#token) return;
    this.#setState('loading');
    try {
      const res = await fetch(
        `/api/v1/bookings/${encodeURIComponent(this.#bookingId)}?token=${encodeURIComponent(this.#token)}`
      );
      if (res.status === 401 || res.status === 403 || res.status === 404) {
        this.#errorMsg = 'UngÃ¼ltiger oder abgelaufener Link.';
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

  // â”€â”€ Session mode loading â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

  async #loadSession() {
    if (!this.#sessionId || !this.#sessionToken) return;
    this.#setState('loading');
    try {
      const res = await fetch(
        `/api/v1/sessions/${encodeURIComponent(this.#sessionId)}?session_token=${encodeURIComponent(this.#sessionToken)}`
      );
      if (res.status === 401 || res.status === 403 || res.status === 404) {
        this.#errorMsg = 'UngÃ¼ltiger oder abgelaufener Link.';
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
      this.#session = await res.json();
      this.#networkError = false;
      this.#setState('session-view');
    } catch {
      this.#errorMsg = 'Verbindung fehlgeschlagen. Bitte Seite neu laden.';
      this.#networkError = true;
      this.#setState('error');
    }
  }

  // â”€â”€ State transitions â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

  #setState(state) {
    this.#state = state;
    this.#render();
    // After rendering the reschedule view, asynchronously load availability.
    if (state === 'session-reschedule') {
      const mp = this.shadowRoot?.getElementById('session-multi-picker');
      if (mp) {
        this.#fetchSessionAvailability(mp).catch((err) =>
          console.error('[x-booking-manager] fetchSessionAvailability failed', err)
        );
      }
    }
  }

  // â”€â”€ Rendering â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

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

      // â”€â”€ Booking mode states â”€â”€
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

      // â”€â”€ Session mode states â”€â”€
      case 'session-view':
        container.appendChild(this.#buildSessionView());
        break;

      case 'session-reschedule':
        container.appendChild(this.#buildSessionView(true));
        container.appendChild(this.#buildSessionReschedule());
        break;

      case 'session-success': {
        const banner = document.createElement('div');
        banner.className = 'banner';
        banner.textContent = 'Ihre Termine wurden erfolgreich neu vergeben.';
        container.appendChild(banner);
        container.appendChild(this.#buildSessionView());
        break;
      }

      case 'session-cancelled': {
        const banner = document.createElement('div');
        banner.className = 'banner';
        banner.textContent = 'Ihre Buchung wurde storniert.';
        container.appendChild(banner);
        break;
      }
    }

    root.appendChild(container);
  }

  // â”€â”€ Booking-mode builders â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

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

  // â”€â”€ Session-mode builders â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

  #buildSessionView(readonly = false) {
    const s = this.#session;
    const wrap = document.createElement('div');
    wrap.className = 'session-card';

    const h2 = document.createElement('h2');
    h2.textContent = s?.service?.name || 'Buchung';
    wrap.appendChild(h2);

    const meta = document.createElement('p');
    meta.className = 'session-meta';
    if (s?.contact?.first_name) {
      meta.textContent = `${s.contact.first_name} ${s.contact.last_name} Â· ${s.contact.email}`;
    }
    wrap.appendChild(meta);

    // Booking lines
    const ul = document.createElement('ul');
    ul.className = 'booking-list';
    const bookings = Array.isArray(s?.bookings) ? s.bookings : [];
    for (const b of bookings) {
      const li = document.createElement('li');
      li.className = 'booking-item';

      const dateSpan = document.createElement('span');
      dateSpan.className = 'booking-item-date';
      try {
        const start = new Date(b.start_at);
        const end   = new Date(b.end_at);
        const locale = navigator.languages?.[0] ?? 'de-DE';
        const tz = Intl.DateTimeFormat().resolvedOptions().timeZone;
        const opts = { weekday: 'short', day: '2-digit', month: '2-digit', year: 'numeric',
                       hour: '2-digit', minute: '2-digit', timeZone: tz };
        dateSpan.textContent =
          `${start.toLocaleString(locale, opts)} â€“ ${end.toLocaleTimeString(locale, { hour: '2-digit', minute: '2-digit', timeZone: tz })} Uhr`;
      } catch {
        dateSpan.textContent = b.start_at;
      }
      li.appendChild(dateSpan);

      const badge = document.createElement('span');
      badge.className = `state-badge state-${b.state}`;
      badge.textContent = STATE_LABELS[b.state] ?? b.state;
      li.appendChild(badge);

      ul.appendChild(li);
    }
    wrap.appendChild(ul);

    if (!readonly) {
      const activeCount = bookings.filter((b) => b.state !== 'cancelled').length;
      if (activeCount > 0) {
        const actions = document.createElement('div');
        actions.className = 'actions';

        const rescheduleBtn = document.createElement('button');
        rescheduleBtn.textContent = 'Termine verschieben';
        rescheduleBtn.addEventListener('click', () => {
          this.#selectedSlots = [];
          this.#rescheduleError = '';
          this.#setState('session-reschedule');
        });
        actions.appendChild(rescheduleBtn);

        const cancelBtn = document.createElement('button');
        cancelBtn.className = 'btn-danger';
        cancelBtn.textContent = 'Alle stornieren';
        cancelBtn.addEventListener('click', () => this.#submitSessionCancel());
        actions.appendChild(cancelBtn);

        wrap.appendChild(actions);
      }
    }

    return wrap;
  }

  #buildSessionReschedule() {
    const activeBookings = (this.#session?.bookings ?? []).filter((b) => b.state !== 'cancelled');
    const needed = activeBookings.length;

    const wrap = document.createElement('div');
    wrap.className = 'reschedule-wrap';

    const h3 = document.createElement('h3');
    h3.textContent = 'Neue Termine wÃ¤hlen';
    wrap.appendChild(h3);

    const hint = document.createElement('p');
    hint.className = 'session-hint';
    hint.textContent = `Bitte wÃ¤hle ${needed} neue${needed === 1 ? 'n Termin' : ' Termine'} aus.`;
    wrap.appendChild(hint);

    if (this.#rescheduleError) {
      const errEl = document.createElement('div');
      errEl.className = 'inline-error';
      errEl.textContent = this.#rescheduleError;
      wrap.appendChild(errEl);
    }

    const mp = document.createElement('x-multi-date-time-picker');
    mp.id = 'session-multi-picker';
    mp.setAttribute('data-availability-endpoint', 'api/v1/availability');
    mp.addEventListener('x-multi-date-time-picker-slots-changed', (e) => {
      this.#selectedSlots = Array.isArray(e.detail?.slots) ? e.detail.slots : [];
      const confirmBtn = this.shadowRoot?.getElementById('session-confirm-btn');
      if (confirmBtn) confirmBtn.disabled = this.#selectedSlots.length !== needed || this.#submitting;
    });
    wrap.appendChild(mp);

    const actions = document.createElement('div');
    actions.className = 'actions';
    actions.style.marginTop = '1rem';

    const confirmBtn = document.createElement('button');
    confirmBtn.id = 'session-confirm-btn';
    confirmBtn.disabled = true;
    confirmBtn.textContent = 'BestÃ¤tigen';
    confirmBtn.addEventListener('click', () => this.#submitSessionReschedule());
    actions.appendChild(confirmBtn);

    const cancelBtn = document.createElement('button');
    cancelBtn.className = 'btn-secondary';
    cancelBtn.textContent = 'Abbrechen';
    cancelBtn.addEventListener('click', () => this.#setState('session-view'));
    actions.appendChild(cancelBtn);

    wrap.appendChild(actions);
    return wrap;
  }

  // â”€â”€ Session-mode API calls â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

  /**
   * Fetches 6 months of availability (current + 5 forward) for the session's
   * service and feeds the merged flat slot array into the multi-picker.
   * @param {HTMLElement} mp  the x-multi-date-time-picker element
   */
  async #fetchSessionAvailability(mp) {
    const serviceId = this.#session?.service?.id;
    if (!serviceId || !mp) return;

    const now = new Date();
    const allSlots = new Set();
    for (let offset = 0; offset < 6; offset++) {
      const d = new Date(now.getFullYear(), now.getMonth() + offset, 1);
      const monthKey = `${String(d.getFullYear()).padStart(4, '0')}-${String(d.getMonth() + 1).padStart(2, '0')}`;
      try {
        const res = await fetch(
          `api/v1/availability?period=${encodeURIComponent(monthKey)}&service_id=${encodeURIComponent(serviceId)}`
        );
        if (!res.ok) continue;
        const payload = await res.json();
        if (Array.isArray(payload)) payload.forEach(s => allSlots.add(s));
      } catch {
        // Non-fatal; picker shows no dates for this month.
      }
    }
    mp.setAttribute('available', JSON.stringify([...allSlots].sort()));
  }

  async #submitSessionReschedule() {
    if (this.#submitting) return;
    this.#submitting = true;
    this.#rescheduleError = '';
    const confirmBtn = this.shadowRoot?.getElementById('session-confirm-btn');
    if (confirmBtn) {
      confirmBtn.disabled = true;
      confirmBtn.textContent = 'Wird gespeichertâ€¦';
    }

    try {
      const res = await fetch(
        `/api/v1/sessions/${encodeURIComponent(this.#sessionId)}/reschedule?session_token=${encodeURIComponent(this.#sessionToken)}`,
        {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            slots:    this.#selectedSlots,
            timezone: Intl.DateTimeFormat().resolvedOptions().timeZone,
          }),
        }
      );
      if (!res.ok) {
        const text = await res.text().catch(() => res.statusText);
        this.#rescheduleError = res.status === 409
          ? 'Ein oder mehrere Zeitslots sind nicht mehr verfÃ¼gbar. Bitte wÃ¤hle andere Termine.'
          : `Fehler beim Verschieben (${res.status}): ${text}`;
        this.#submitting = false;
        this.#setState('session-reschedule');
        return;
      }
      // Reload session data to show updated booking times.
      await this.#loadSession();
      this.#setState('session-success');
    } catch {
      this.#rescheduleError = 'Verbindung fehlgeschlagen. Bitte erneut versuchen.';
      this.#submitting = false;
      this.#setState('session-reschedule');
    } finally {
      this.#submitting = false;
    }
  }

  async #submitSessionCancel() {
    try {
      const tz = encodeURIComponent(Intl.DateTimeFormat().resolvedOptions().timeZone);
      const res = await fetch(
        `/api/v1/sessions/${encodeURIComponent(this.#sessionId)}?session_token=${encodeURIComponent(this.#sessionToken)}&tz=${tz}`,
        { method: 'DELETE' }
      );
      if (!res.ok) {
        const text = await res.text().catch(() => res.statusText);
        this.#errorMsg = `Stornierung fehlgeschlagen (${res.status}): ${text}`;
        this.#networkError = false;
        this.#setState('error');
        return;
      }
      this.#setState('session-cancelled');
    } catch {
      this.#errorMsg = 'Verbindung fehlgeschlagen. Bitte Seite neu laden.';
      this.#networkError = true;
      this.#setState('error');
    }
  }
}

customElements.define('x-booking-manager', XBookingManager);
