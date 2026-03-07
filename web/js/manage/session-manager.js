/**
 * <x-session-manager> â€” customer-facing session lifecycle manager.
 *
 * Uses a single <x-multi-date-time-picker> as the core UI for both displaying
 * existing bookings (via `initial-bookings`) and collecting new slot selections
 * (via `available`).
 *
 * â”€â”€ Modes â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€
 *
 *   view              â€” picker shows active bookings (initial-bookings); delete button
 *                       on each triggers cancel-booking. Session action bar below.
 *   reschedule-all    â€” picker shows N fresh empty pickers with available slots;
 *                       confirm emits reschedule-session when all N filled.
 *   add-booking       â€” picker shows active bookings pre-seeded + 1 empty picker;
 *                       confirm emits add-booking with the one slot not in the original set.
 *   cancel-session-confirm â€” confirmation overlay; confirm emits cancel-session.
 *
 * â”€â”€ Attributes (bootstrapping credentials) â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€
 *   session-id    (string)
 *   session-token (string)
 *
 * â”€â”€ Properties (parent-driven data) â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€
 *   session      (object | null)  â€” full sessionDetailResp; sets clears mode, re-renders
 *   availability (object)         â€” { "YYYY-MM": ["ISO", ...] } merged into a flat slot pool
 *   error        (string | null)  â€” error message; cleared when .session is set next
 *
 * â”€â”€ Events fired â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€
 *   session-action      { type, sessionId, sessionToken, â€¦ } â€” see type union below
 *   availability-needed { month: "YYYY-MM", serviceId }
 *
 *   session-action types:
 *     add-booking        { start, timezone }
 *     reschedule-session { slots: string[], timezone }
 *     cancel-booking     { bookingId, timezone }
 *     cancel-session     { timezone }
 */

import '../x-multi-date-time-picker.js';

// â”€â”€ Styles â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

const STYLES = `
  @import '/css/tokens.css';

  :host {
    --color-bg:             #f9fafb;
    --color-surface:        #ffffff;
    --color-border:         #d1d5db;
    --color-text:           #1f2937;
    --color-text-secondary: #6b7280;
    --color-primary:        #0f62fe;
    --color-danger:         #dc2626;
    --color-danger-bg:      #fef2f2;
    --color-success:        #16a34a;
    --color-success-bg:     #f0fdf4;
    --font-family:          Inter, "Segoe UI", system-ui, -apple-system, sans-serif;
    --radius-sm:            6px;
    --radius-md:            10px;
    --shadow-card:          0 1px 4px rgba(0,0,0,.08);

    display: block;
    font-family: var(--font-family);
    font-size: 1rem;
    color: var(--color-text);
    box-sizing: border-box;
  }
  *, *::before, *::after { box-sizing: inherit; }
  [hidden] { display: none !important; }

  .container {
    max-width: 700px;
    margin: 2rem auto;
    padding: 0 1rem;
  }

  /* â”€â”€ Spinners & banners â”€â”€ */
  .spinner-wrap {
    display: flex;
    justify-content: center;
    padding: 3rem 0;
  }
  .spinner {
    width: 40px; height: 40px;
    border: 4px solid var(--color-border);
    border-top-color: var(--color-primary);
    border-radius: 50%;
    animation: spin 0.8s linear infinite;
  }
  @keyframes spin { to { transform: rotate(360deg); } }

  .error-banner {
    padding: 1rem;
    border-radius: var(--radius-sm);
    background: var(--color-danger-bg);
    border: 1px solid var(--color-danger);
    color: var(--color-danger);
    margin-bottom: 1rem;
  }
  .error-banner p { margin: 0; }

  .success-banner {
    padding: 0.75rem 1rem;
    border-radius: var(--radius-sm);
    background: var(--color-success-bg);
    border: 1px solid var(--color-success);
    color: var(--color-success);
    font-weight: 600;
    margin-bottom: 1rem;
  }

  /* â”€â”€ Session header â”€â”€ */
  .session-header {
    margin-bottom: 1.25rem;
  }
  .session-title {
    margin: 0 0 0.2rem;
    font-size: 1.25rem;
    font-weight: 700;
  }
  .session-subtitle {
    margin: 0;
    font-size: 0.9rem;
    color: var(--color-text-secondary);
  }

  /* â”€â”€ Cancelled booking list (below picker in all modes) â”€â”€ */
  .cancelled-list {
    margin: 0.75rem 0 0;
    padding: 0;
    list-style: none;
    display: grid;
    gap: 0.4rem;
  }
  .cancelled-item {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 0.5rem 0.75rem;
    border-radius: var(--radius-sm);
    background: var(--color-bg);
    border: 1px solid var(--color-border);
    font-size: 0.875rem;
    opacity: 0.6;
  }
  .cancelled-badge {
    font-size: 0.72rem;
    font-weight: 600;
    color: var(--color-text-secondary);
  }

  /* â”€â”€ Picker mode hints â”€â”€ */
  .picker-hint {
    font-size: 0.875rem;
    color: var(--color-text-secondary);
    margin: 0 0 0.75rem;
  }

  /* â”€â”€ Action bars â”€â”€ */
  .action-bar {
    display: flex;
    flex-wrap: wrap;
    gap: 0.6rem;
    margin-top: 1.25rem;
  }

  /* â”€â”€ Confirmation overlay â”€â”€ */
  .confirm-overlay {
    margin-top: 1rem;
    padding: 1.25rem 1.5rem;
    border-radius: var(--radius-md);
    background: var(--color-danger-bg);
    border: 1px solid var(--color-danger);
    box-shadow: var(--shadow-card);
  }
  .confirm-title {
    margin: 0 0 0.5rem;
    font-size: 1.05rem;
    font-weight: 700;
    color: var(--color-text);
  }
  .confirm-body {
    margin: 0 0 1rem;
    font-size: 0.9rem;
    color: var(--color-text-secondary);
  }

  /* â”€â”€ Buttons â”€â”€ */
  .btn {
    padding: 0.5rem 1.1rem;
    border-radius: var(--radius-sm);
    border: 1px solid transparent;
    font-size: 0.9rem;
    font-weight: 500;
    font-family: inherit;
    cursor: pointer;
    line-height: 1.4;
    transition: opacity 0.12s;
  }
  .btn:focus-visible {
    outline: 2px solid var(--color-primary);
    outline-offset: 2px;
  }
  .btn:disabled { opacity: 0.4; cursor: not-allowed; }
  .btn--primary {
    background: var(--color-primary);
    border-color: var(--color-primary);
    color: #fff;
  }
  .btn--primary:not(:disabled):hover { opacity: 0.88; }
  .btn--secondary {
    background: transparent;
    border-color: var(--color-border);
    color: var(--color-text);
  }
  .btn--secondary:not(:disabled):hover { background: var(--color-bg); }
  .btn--danger {
    background: var(--color-danger);
    border-color: var(--color-danger);
    color: #fff;
  }
  .btn--danger:not(:disabled):hover { opacity: 0.88; }
`;

// â”€â”€ Component â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

/**
 * XSessionManager â€” customer-facing session lifecycle manager.
 *
 * Attributes:  session-id, session-token
 * Properties:  session, availability, error
 * Events:      session-action, availability-needed
 */
export class XSessionManager extends HTMLElement {
  // â”€â”€ Private state â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

  /** Full sessionDetailResp or null. */
  #session = null;

  /** Error message or null. */
  #error = null;

  /**
   * Flat pool of all available ISO 8601 UTC slot strings.
   * Built by merging months received via .availability setter.
   */
  #flatSlots = [];

  /**
   * Current interaction mode.
   * @type {'view'|'reschedule-all'|'add-booking'|'cancel-session-confirm'}
   */
  #mode = 'view';

  /** ISO slot strings emitted by the picker's slots-changed event. */
  #selectedSlots = [];

  /** Set of "YYYY-MM" keys already fetched, to avoid duplicate requests. */
  #fetchedMonths = new Set();

  /**
   * Original start_at values of active bookings at the time we enter edit mode.
   * Used in add-booking mode to identify the newly added slot.
   */
  #originalStarts = [];

  // â”€â”€ Lifecycle â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

  constructor() {
    super();
    this.attachShadow({ mode: 'open' });
  }

  static get observedAttributes() { return ['session-id', 'session-token']; }

  connectedCallback() {
    this.#render();
    // x-date-time-picker fires x-date-time-picker-month-selected as composed:true.
    // Intercept here to lazy-fetch months not yet loaded.
    this.shadowRoot.addEventListener('x-date-time-picker-month-selected', (e) => {
      const { year, month } = e.detail ?? {};
      if (typeof year !== 'number' || typeof month !== 'number') return;
      const key = `${String(year).padStart(4,'0')}-${String(month).padStart(2,'0')}`;
      this.#fetchAvailabilityMonth(key).catch(err =>
        console.error('[x-session-manager] availability fetch failed', key, err));
    });
  }

  // â”€â”€ Public properties â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

  get session() { return this.#session; }
  /**
   * Set after every successful API call. Clears active mode and re-renders.
   */
  set session(v) {
    this.#session = v ?? null;
    this.#error   = null;
    this.#mode    = 'view';
    this.#selectedSlots  = [];
    this.#originalStarts = [];
    this.#flatSlots      = [];
    this.#fetchedMonths.clear();
    this.#render();
  }

  get error() { return this.#error; }
  /** Set when a parent API call fails. Cleared automatically when .session is set next. */
  set error(v) {
    this.#error = v ?? null;
    this.#render();
  }

  get availability() { return []; /* write-only from outside */ }
  /**
   * Merges a flat array of UTC ISO 8601 slot strings into the internal pool
   * and feeds the updated list to the visible picker immediately.
   * The array is the direct body of GET /api/v1/availability.
   */
  set availability(v) {
    if (!Array.isArray(v)) return;
    // Merge de-duplicated
    const set = new Set([...this.#flatSlots, ...v]);
    this.#flatSlots = [...set].sort();
    // Feed to the visible picker immediately
    const mp = this.shadowRoot?.getElementById('sm-picker');
    if (mp) mp.setAttribute('available', JSON.stringify(this.#flatSlots));
  }

  // â”€â”€ Private getters â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

  get #sessionId()    { return this.getAttribute('session-id')    ?? ''; }
  get #sessionToken() { return this.getAttribute('session-token') ?? ''; }

  get #activeBookings() {
    return (this.#session?.bookings ?? []).filter(
      (b) => b.state !== 'cancelled' && b.state !== 'noshow',
    );
  }

  get #cancelledBookings() {
    return (this.#session?.bookings ?? []).filter(
      (b) => b.state === 'cancelled' || b.state === 'noshow',
    );
  }

  // â”€â”€ Private rendering â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

  #render() {
    const root = this.shadowRoot;
    root.innerHTML = `<style>${STYLES}</style>`;

    const container = document.createElement('div');
    container.className = 'container';

    if (this.#error) {
      container.appendChild(this.#el('div', { class: 'error-banner', role: 'alert' },
        this.#el('p', {}, this.#error)));
    } else if (!this.#session) {
      container.appendChild(this.#buildSpinner());
    } else {
      container.appendChild(this.#buildHeader());
      container.appendChild(this.#buildPickerSection());

      const cancelled = this.#cancelledBookings;
      if (cancelled.length > 0) {
        container.appendChild(this.#buildCancelledList(cancelled));
      }

      if (this.#mode === 'cancel-session-confirm') {
        container.appendChild(this.#buildCancelSessionConfirm());
      } else {
        container.appendChild(this.#buildActionBar());
      }
    }

    root.appendChild(container);
  }

  // â”€â”€ Header â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

  #buildHeader() {
    const s = this.#session;
    const wrap = document.createElement('div');
    wrap.className = 'session-header';

    const h1 = this.#el('h1', { class: 'session-title' }, s.service?.name ?? 'Buchung');
    wrap.appendChild(h1);
    if (s.contact?.first_name) {
      wrap.appendChild(this.#el('p', { class: 'session-subtitle' },
        `${s.contact.first_name} ${s.contact.last_name} Â· ${s.contact.email}`));
    }
    return wrap;
  }

  // â”€â”€ Picker section â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

  #buildPickerSection() {
    const active  = this.#activeBookings;
    const wrap    = document.createElement('div');

    // Optional hint text above the picker
    const hint = this.#pickerHint(active.length);
    if (hint) wrap.appendChild(this.#el('p', { class: 'picker-hint' }, hint));

    const mp = document.createElement('x-multi-date-time-picker');
    mp.id = 'sm-picker';

    switch (this.#mode) {
      case 'view':
        // Show existing active bookings; delete button = cancel intent
        mp.setAttribute('initial-bookings', JSON.stringify({ bookings: active }));
        mp.setAttribute('max-items', String(active.length)); // prevent add-button
        break;

      case 'reschedule-all':
        // Fresh empty pickers â€” one per active booking. User picks N new slots.
        mp.setAttribute('max-items', String(active.length));
        mp.setAttribute('available', JSON.stringify(this.#flatSlots));
        break;

      case 'add-booking':
        // Show existing active bookings pre-seeded + 1 extra empty picker
        mp.setAttribute('initial-bookings', JSON.stringify({ bookings: active }));
        mp.setAttribute('max-items', String(active.length + 1));
        mp.setAttribute('available', JSON.stringify(this.#flatSlots));
        break;

      case 'cancel-session-confirm':
        // Read-only snapshot while confirm dialog is shown
        mp.setAttribute('initial-bookings', JSON.stringify({ bookings: active }));
        mp.setAttribute('max-items', String(active.length));
        mp.setAttribute('disabled', '');
        break;
    }

    // Slot selection changes
    mp.addEventListener('x-multi-date-time-picker-slots-changed', (e) => {
      this.#selectedSlots = Array.isArray(e.detail?.slots) ? e.detail.slots : [];
      this.#syncConfirmButton();
    });

    // Delete button pressed on a picker that has a booking â†’ cancel intent
    mp.addEventListener('x-multi-date-time-picker-item-removed', (e) => {
      const { bookingId } = e.detail ?? {};
      if (bookingId && this.#mode === 'view') {
        this.#emit('session-action', {
          type:         'cancel-booking',
          sessionId:    this.#sessionId,
          sessionToken: this.#sessionToken,
          bookingId,
          timezone:     Intl.DateTimeFormat().resolvedOptions().timeZone,
        });
      }
      // In add/reschedule mode: item removal adjusts the picker count; no action emitted
    });

    wrap.appendChild(mp);

    // Request the initial 6 months of availability whenever a mode that needs it is entered
    if (this.#mode === 'reschedule-all' || this.#mode === 'add-booking') {
      this.#requestInitialAvailability();
    }

    return wrap;
  }

  #pickerHint(activeCount) {
    switch (this.#mode) {
      case 'reschedule-all':
        return `Bitte wÃ¤hle ${activeCount} neue${activeCount === 1 ? 'n Termin' : ' Termine'} aus.`;
      case 'add-booking':
        return 'Bitte wÃ¤hle einen neuen Termin aus.';
      default:
        return null;
    }
  }

  // â”€â”€ Cancelled bookings list (read-only, below picker) â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

  #buildCancelledList(bookings) {
    const ul = document.createElement('ul');
    ul.className = 'cancelled-list';
    for (const b of bookings) {
      const li = document.createElement('li');
      li.className = 'cancelled-item';
      const dateEl = document.createElement('span');
      try {
        const tz     = Intl.DateTimeFormat().resolvedOptions().timeZone;
        const locale = navigator.languages?.[0] ?? 'de-DE';
        const start  = new Date(b.start_at);
        const end    = new Date(b.end_at);
        const dOpts  = { weekday: 'short', day: '2-digit', month: '2-digit', year: 'numeric', hour: '2-digit', minute: '2-digit', timeZone: tz };
        const tOpts  = { hour: '2-digit', minute: '2-digit', timeZone: tz };
        dateEl.textContent = `${start.toLocaleString(locale, dOpts)} â€“ ${end.toLocaleTimeString(locale, tOpts)} Uhr`;
      } catch {
        dateEl.textContent = b.start_at;
      }
      li.appendChild(dateEl);
      const reason = b.cancel_reason === 'admin' ? 'Abgelehnt' : 'Storniert';
      li.appendChild(this.#el('span', { class: 'cancelled-badge' }, reason));
      ul.appendChild(li);
    }
    return ul;
  }

  // â”€â”€ Action bars â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

  #buildActionBar() {
    const bar  = document.createElement('div');
    bar.className = 'action-bar';
    const active = this.#activeBookings;

    switch (this.#mode) {
      case 'view': {
        const canAdd = this.#session?.can_add_booking ?? (this.#session?.state === 'submitted');
        if (canAdd) {
          bar.appendChild(this.#btn('Termin hinzufÃ¼gen', 'primary', () => this.#enterAddBooking()));
        }
        if (active.length > 0) {
          bar.appendChild(this.#btn('Alle verschieben', 'secondary', () => this.#enterRescheduleAll()));
          bar.appendChild(this.#btn('Alle stornieren', 'danger', () => this.#enterCancelSession()));
        }
        break;
      }

      case 'reschedule-all': {
        const confirmBtn = this.#btn('Verschieben bestÃ¤tigen', 'primary', () => this.#submitRescheduleAll());
        confirmBtn.id       = 'sm-confirm';
        confirmBtn.disabled = true;
        bar.appendChild(confirmBtn);
        bar.appendChild(this.#btn('Abbrechen', 'secondary', () => this.#enterView()));
        break;
      }

      case 'add-booking': {
        const confirmBtn = this.#btn('Termin buchen', 'primary', () => this.#submitAddBooking());
        confirmBtn.id       = 'sm-confirm';
        confirmBtn.disabled = true;
        bar.appendChild(confirmBtn);
        bar.appendChild(this.#btn('Abbrechen', 'secondary', () => this.#enterView()));
        break;
      }
    }

    return bar;
  }

  // â”€â”€ Cancel-session confirmation â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

  #buildCancelSessionConfirm() {
    const wrap = document.createElement('div');
    wrap.className = 'confirm-overlay';
    wrap.setAttribute('role', 'alertdialog');
    wrap.setAttribute('aria-label', 'Buchung stornieren');

    wrap.appendChild(this.#el('h2', { class: 'confirm-title' }, 'Gesamte Buchung stornieren'));
    wrap.appendChild(this.#el('p', { class: 'confirm-body' },
      'MÃ¶chtest du alle Termine dieser Buchung stornieren? Diese Aktion kann nicht rÃ¼ckgÃ¤ngig gemacht werden.'));

    const bar = document.createElement('div');
    bar.className = 'action-bar';
    bar.style.marginTop = '0';
    bar.appendChild(this.#btn('Alle stornieren', 'danger', () => this.#submitCancelSession()));
    bar.appendChild(this.#btn('Abbrechen', 'secondary', () => this.#enterView()));
    wrap.appendChild(bar);

    return wrap;
  }

  // â”€â”€ Mode transitions â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

  #enterView() {
    this.#mode           = 'view';
    this.#selectedSlots  = [];
    this.#originalStarts = [];
    this.#render();
  }

  #enterRescheduleAll() {
    this.#mode           = 'reschedule-all';
    this.#selectedSlots  = [];
    this.#originalStarts = this.#activeBookings.map((b) => b.start_at);
    this.#render();
  }

  #enterAddBooking() {
    this.#mode           = 'add-booking';
    this.#selectedSlots  = [];
    this.#originalStarts = this.#activeBookings.map((b) => b.start_at);
    this.#render();
  }

  #enterCancelSession() {
    this.#mode = 'cancel-session-confirm';
    this.#render();
  }

  // â”€â”€ Confirm-button sync â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

  #syncConfirmButton() {
    const btn = this.shadowRoot?.getElementById('sm-confirm');
    if (!btn) return;

    let ready = false;
    switch (this.#mode) {
      case 'reschedule-all': {
        const needed = this.#activeBookings.length;
        ready = this.#selectedSlots.length === needed;
        break;
      }
      case 'add-booking': {
        // At least one slot must be NEW (not in originalStarts)
        const originalSet = new Set(this.#originalStarts);
        const newSlots = this.#selectedSlots.filter((s) => !originalSet.has(s));
        ready = newSlots.length === 1;
        break;
      }
    }
    btn.disabled = !ready;
  }

  // â”€â”€ Action emitters â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

  #submitRescheduleAll() {
    const needed = this.#activeBookings.length;
    if (this.#selectedSlots.length !== needed) return;
    this.#emit('session-action', {
      type:         'reschedule-session',
      sessionId:    this.#sessionId,
      sessionToken: this.#sessionToken,
      slots:        this.#selectedSlots,
      timezone:     Intl.DateTimeFormat().resolvedOptions().timeZone,
    });
  }

  #submitAddBooking() {
    const originalSet = new Set(this.#originalStarts);
    const newSlots    = this.#selectedSlots.filter((s) => !originalSet.has(s));
    if (newSlots.length !== 1) return;
    this.#emit('session-action', {
      type:         'add-booking',
      sessionId:    this.#sessionId,
      sessionToken: this.#sessionToken,
      start:        newSlots[0],
      timezone:     Intl.DateTimeFormat().resolvedOptions().timeZone,
    });
  }

  #submitCancelSession() {
    this.#emit('session-action', {
      type:         'cancel-session',
      sessionId:    this.#sessionId,
      sessionToken: this.#sessionToken,
      timezone:     Intl.DateTimeFormat().resolvedOptions().timeZone,
    });
  }

  // â”€â”€ Availability helpers â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

  /**
   * Fetches the current month plus 5 forward months and merges the slots into
   * #flatSlots, then feeds the updated pool to the visible picker.
   */
  #requestInitialAvailability() {
    const now = new Date();
    const fetches = [];
    for (let i = 0; i < 6; i++) {
      const d   = new Date(now.getFullYear(), now.getMonth() + i, 1);
      const key = `${String(d.getFullYear()).padStart(4,'0')}-${String(d.getMonth()+1).padStart(2,'0')}`;
      fetches.push(this.#fetchAvailabilityMonth(key));
    }
    Promise.all(fetches).catch(err =>
      console.error('[x-session-manager] initial availability fetch failed', err));
  }

  /**
   * Fetches one calendar month from GET /api/v1/availability and merges the
   * returned flat string array into #flatSlots. Deduplicates requests.
   * @param {string} monthKey  "YYYY-MM"
   */
  async #fetchAvailabilityMonth(monthKey) {
    const serviceId = this.#session?.service?.id;
    if (!serviceId) return;
    if (this.#fetchedMonths.has(monthKey)) return;
    this.#fetchedMonths.add(monthKey);

    const url = `api/v1/availability?period=${encodeURIComponent(monthKey)}&service_id=${encodeURIComponent(serviceId)}`;
    const res = await fetch(url);
    if (!res.ok) {
      this.#fetchedMonths.delete(monthKey); // allow retry on transient error
      throw new Error(`availability fetch ${monthKey}: ${res.status}`);
    }
    const slots = await res.json();
    if (!Array.isArray(slots)) return;

    // Merge into flat pool
    const merged = new Set([...this.#flatSlots, ...slots]);
    this.#flatSlots = [...merged].sort();

    // Push to the visible picker
    const mp = this.shadowRoot?.getElementById('sm-picker');
    if (mp) mp.setAttribute('available', JSON.stringify(this.#flatSlots));
  }

  // â”€â”€ Generic helpers â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

  #buildSpinner() {
    const wrap  = document.createElement('div');
    wrap.className = 'spinner-wrap';
    const inner = document.createElement('div');
    inner.className = 'spinner';
    inner.setAttribute('role', 'status');
    inner.setAttribute('aria-label', 'Wird geladen');
    wrap.appendChild(inner);
    return wrap;
  }

  /**
   * Creates an element with optional attributes and a text child.
   * @param {string} tag
   * @param {Object} attrs
   * @param {string} [text]
   */
  #el(tag, attrs = {}, text = '') {
    const el = document.createElement(tag);
    for (const [k, v] of Object.entries(attrs)) el.setAttribute(k, v);
    if (text) el.textContent = text;
    return el;
  }

  /**
   * Creates a styled button.
   * @param {string}   label
   * @param {'primary'|'secondary'|'danger'} variant
   * @param {Function} onClick
   */
  #btn(label, variant, onClick) {
    const btn = document.createElement('button');
    btn.className   = `btn btn--${variant}`;
    btn.textContent = label;
    btn.addEventListener('click', onClick);
    return btn;
  }

  #emit(eventName, detail) {
    this.dispatchEvent(new CustomEvent(eventName, { bubbles: true, detail }));
  }
}

customElements.define('x-session-manager', XSessionManager);
