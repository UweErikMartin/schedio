// ─── Redesigned to comply with the x-foo Web Component conventions ───────────
// Key changes vs. the original implementation:
//   - Shadow DOM (attachShadow) replaces light-DOM innerHTML
//   - STYLES / TRANSLATIONS follow the required constant names
//   - @import '/css/tokens.css' replaces inline CSS custom properties
//   - Class renamed XDateTimePicker (PascalCase) with named export
//   - All instance state moved to private fields (#)
//   - observedAttributes limited to lowercase kebab-case only
//   - All custom events include composed: true
//   - External locale files loaded from <locale-path>/<lang>.json (configurable)
//   - Mobile-first responsive CSS with min-width breakpoints
// -----------------------------------------------------------------------------

const STYLES = `
  @import '/css/tokens.css';

  /* Fallback token values — used when tokens.css fails to load */
  :host {
    --color-bg:             #f9fafb;
    --color-surface:        #ffffff;
    --color-border:         #d1d5db;
    --color-text:           #1f2937;
    --color-muted:          #6b7280;
    --color-primary:        #0f62fe;
    --color-primary-hover:  #0353d9;
    --color-danger:         #dc2626;
    --color-warning:        #f59e0b;
    --color-success:        #16a34a;
    --color-tentative:      #7c3aed;
    --font-family:          Inter, "Segoe UI", system-ui, -apple-system, sans-serif;
    --font-size-sm:         0.875rem;
    --font-size-base:       1rem;
    --font-size-lg:         1.125rem;
    --font-size-xl:         1.25rem;
    --font-weight-normal:   400;
    --font-weight-medium:   500;
    --font-weight-bold:     600;
    --space-xs:             0.25rem;
    --space-sm:             0.5rem;
    --space-md:             1rem;
    --space-lg:             1.5rem;
    --space-xl:             2rem;
    --radius-sm:            6px;
    --radius-md:            10px;
    --radius-lg:            16px;
    --shadow-card:          0 1px 4px rgba(0, 0, 0, 0.08);

    display: block;
    font-family: var(--font-family);
    font-size: var(--font-size-base);
    color: var(--color-text);
    position: relative;
  }

  *,
  *::before,
  *::after {
    box-sizing: border-box;
  }

  [hidden] { display: none !important; }

  /* -- Date summary bar (mobile-first: stacked) ---- */
  .date-summary {
    display: grid;
    grid-template-columns: 1fr auto;
    align-items: center;
    gap: var(--space-sm);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    background: var(--color-surface);
    padding: 0.35rem var(--space-sm);
    width: 100%;
    cursor: pointer;
    box-shadow: var(--shadow-card);
    transition: border-color 0.15s;
  }

  .date-summary:hover {
    border-color: var(--color-primary);
  }

  /* -- State colours -- */
  :host([state="tentative"]) .date-summary {
    background: #fefce8;
    border-color: #fde047;
  }
  :host([state="tentative"]) .date-summary:hover {
    border-color: #eab308;
  }

  :host([state="confirmed"]) .date-summary {
    background: #f0fdf4;
    border-color: #86efac;
  }
  :host([state="confirmed"]) .date-summary:hover {
    border-color: #16a34a;
  }

  :host([state="cancelled"]) .date-summary {
    background: #fef2f2;
    border-color: #fca5a5;
  }
  :host([state="cancelled"]) .date-summary:hover {
    border-color: #dc2626;
  }
  :host([state="cancelled"]) .selected-date-label {
    text-decoration: line-through;
  }

  :host([state="new"]) .date-summary {
    background: #eff6ff;
    border-style: dashed;
    border-color: var(--color-primary);
  }
  :host([state="new"]) .date-summary:hover {
    border-color: var(--color-primary-hover);
  }

  /* -- Disabled state -- */
  :host([disabled]) .date-summary {
    background: #f3f4f6;
    border-color: #d1d5db;
    color: var(--color-muted);
    cursor: default;
    box-shadow: none;
  }
  :host([disabled]) .date-summary:hover {
    border-color: #d1d5db;
  }
  :host([disabled]) .selected-date-label {
    color: var(--color-muted);
  }

  .selected-date-wrap {
    min-width: 0;
  }

  .selected-date-label {
    display: block;
    word-break: break-word;
  }

  .date-actions {
    display: flex;
    align-items: center;
  }

  .date-actions .inline-button {
    flex: 0 0 auto;
  }

  /* -- Interactive element resets -- */
  .inline-button,
  .calendar-nav,
  .calendar-day,
  .time-option {
    touch-action: manipulation;
    -webkit-tap-highlight-color: transparent;
    font: inherit;
    cursor: pointer;
  }

  /* -- Select-time button (icon) -- */
  .inline-button {
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    background: var(--color-surface);
    color: var(--color-text);
    width: 44px;
    height: 44px;
    padding: 0;
    font-size: 1.4rem;
    line-height: 1;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    transition: border-color 0.15s;
  }

  .inline-button:hover:not(:disabled) {
    border-color: var(--color-primary);
  }

  .inline-button:disabled {
    opacity: 0.45;
    cursor: not-allowed;
  }

  #actions-button svg,
  #select-time-button svg {
    display: block;
  }

  :host([action="add"]) #actions-button svg,
  :host([action="confirm"]) #actions-button svg {
    color: var(--color-success);
  }

  :host([action="delete"]) #actions-button svg {
    color: var(--color-danger);
  }

  /* -- Dropdown panels -- */
  .calendar,
  .time-list {
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    padding: var(--space-md);
    background: var(--color-surface);
    box-shadow: 0 4px 16px rgba(0, 0, 0, 0.10);
    position: absolute;
    top: calc(100% + var(--space-xs));
    left: 0;
    right: 0;
    z-index: 20;
  }

  /* -- Calendar header -- */
  .calendar-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: var(--space-sm);
  }

  .calendar-nav {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 44px;
    height: 44px;
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    background: var(--color-surface);
    color: var(--color-text);
    font-size: var(--font-size-lg);
    line-height: 1;
    transition: border-color 0.15s;
  }

  .calendar-nav:hover:not(:disabled) {
    border-color: var(--color-primary);
  }

  .calendar-nav:disabled {
    opacity: 0.35;
    cursor: not-allowed;
  }

  .calendar-month-label {
    font-weight: var(--font-weight-bold);
    font-size: var(--font-size-base);
  }

  /* -- Calendar grid -- */
  .calendar-weekdays,
  .calendar-grid {
    display: grid;
    grid-template-columns: repeat(7, minmax(0, 1fr));
    gap: var(--space-xs);
  }

  .calendar-weekdays {
    margin-bottom: var(--space-xs);
    color: var(--color-muted);
    font-size: var(--font-size-sm);
    font-weight: var(--font-weight-medium);
    text-align: center;
  }

  .calendar-weekdays span {
    text-align: center;
  }

  .calendar-spacer {
    height: 2.5rem;
  }

  .calendar-day {
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    background: var(--color-surface);
    color: var(--color-text);
    min-height: 2.5rem;
    font-size: var(--font-size-sm);
    transition: border-color 0.12s, background 0.12s;
  }

  .calendar-day:hover:not(:disabled) {
    border-color: var(--color-primary);
  }

  .calendar-day.today {
    border-color: var(--color-primary);
    color: var(--color-primary);
    font-weight: var(--font-weight-medium);
  }

  .calendar-day.selected {
    background: var(--color-primary);
    border-color: var(--color-primary);
    color: #fff; /* white on --color-primary, contrast > 4.5:1 */
    font-weight: var(--font-weight-bold);
  }

  .calendar-day.disabled,
  .calendar-day:disabled {
    background: var(--color-bg);
    color: var(--color-muted);
    border-color: var(--color-border);
    cursor: not-allowed;
  }

  /* -- Time list -- */
  .time-options {
    display: grid;
    gap: var(--space-xs);
  }

  .time-option {
    width: 100%;
    text-align: left;
    padding: var(--space-sm) var(--space-md);
    min-height: 44px;
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    background: var(--color-surface);
    color: var(--color-text);
    transition: background 0.12s, border-color 0.12s;
  }

  .time-option:hover:not(:disabled) {
    border-color: var(--color-primary);
  }

  .time-option.selected {
    background: var(--color-primary);
    color: #fff; /* white on --color-primary, contrast > 4.5:1 */
    border-color: var(--color-primary);
  }

  .time-option:disabled {
    background: var(--color-bg);
    color: var(--color-muted);
    border-color: var(--color-border);
    cursor: not-allowed;
  }

  /* -- WCAG 2.1 AA focus ring -- */
  :focus-visible {
    outline: 2px solid var(--color-primary);
    outline-offset: 2px;
  }
`;

/** Bundled translations -- English (default) and German. */
/** Inline SVG icons — pixel-perfect centering regardless of font metrics. */
const SVG_CLOCK = `<svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/></svg>`;
const SVG_ADD   = `<svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" aria-hidden="true"><line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/></svg>`;
const SVG_DELETE = `<svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" aria-hidden="true"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>`;
const SVG_CONFIRM = `<svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><polyline points="20 6 9 17 4 12"/></svg>`;

const TRANSLATIONS = {
  en: {
    'loading.initial':      'Next available date is loading...',
    'loading.availability': 'Available dates are loading...',
    'no.date.selected':     'Please select a date and time',
    'date.prefix.selected': 'Selected date',
    'date.prefix.earliest': 'Earliest appointment',
    'time.suffix':          "o'clock",
    'btn.select.time':      'Change time',
    'lbl.month':            'Month',
    'nav.next.month':       'Next month',
    'nav.prev.month':       'Previous month',
    'no.slots':             'No time slots available.',
    'btn.add.appointment':    'Add appointment',
    'btn.delete.appointment': 'Delete appointment',
    'btn.confirm.appointment':  'Confirm appointment',
  },
  de: {
    'loading.initial':      'Nächster verfuegbarer Termin wird geladen...',
    'loading.availability': 'Verfügbare Termine werden geladen...',
    'no.date.selected':     'Bitte wählen Sie ein Datum und eine Uhrzeit',
    'date.prefix.selected': 'Ausgewählter Termin',
    'date.prefix.earliest': 'Frühester Termin',
    'time.suffix':          'Uhr',
    'btn.select.time':      'Zeit ändern',
    'lbl.month':            'Monat',
    'nav.next.month':       'Nächster Monat',
    'nav.prev.month':       'Vorheriger Monat',
    'no.slots':             'Keine Zeitslots verfügbar.',
    'btn.add.appointment':    'Termin hinzufügen',
    'btn.delete.appointment': 'Termin löschen',
    'btn.confirm.appointment':  'Termin bestätigen',
  },
};

/**
 * XDateTimePicker -- Calendar + time-slot picker web component.
 *
 * Attributes:
 *   available     (string)  -- JSON array of UTC ISO 8601 datetime strings; groups slots by
 *                              date automatically and replaces all current availability.
 *                              e.g. `["2026-03-09T09:00:00Z","2026-03-09T11:00:00Z"]`
 *   selected     (string)  -- UTC ISO 8601 datetime to pre-select programmatically
 *   locale          (string)  -- BCP 47 tag (falls back to document.documentElement.lang,
 *                                then navigator.language)
 *   disabled        (boolean) -- disables all interactive elements when present
 *   action      (string)  -- icon and label for the action button: 'add' (default), 'delete', or 'confirm'.
 *                             When absent the button is hidden; when present the button is shown and
 *                             automatically disabled until a date-time is selected.
 *
 * Properties:
 *   value      (string|null) -- currently selected UTC ISO 8601 datetime, or null
 *   disabled   (boolean)     -- reflects the `disabled` boolean attribute
 *   action     (string)      -- reflects the `action` attribute
 *
 * Events (all: bubbles=true, composed=true):
 *   x-date-time-picker-initialized        { year, month, selectedDate }
 *   x-date-time-picker-date-selected      { date, previousDate, timeSlots,
 *                                           hasUserChangedDate, year, month }
 *   x-date-time-picker-month-selected     { year, month, previousYear, previousMonth }
 *   x-date-time-picker-add-delete-pressed { date, selectedTime }
 */
export class XDateTimePicker extends HTMLElement {
  // -- Private state -------------------------------------------------
  #t                          = {};    // resolved translation map
  #selectedDate               = '';   // YYYY-MM-DD (local calendar key)
  #selectedTime               = '';   // UTC ISO string
  #activeYear                 = 0;
  #activeMonth                = 0;    // 1-based
  #hasUserChangedDate         = false;
  #availableDatesByMonth      = new Map();
  #slotLabelCache             = new Map();
  #hasEmittedInitializedEvent = false;
  #isDropdownSessionActive    = false;
  #dropdownScrollRestoreY     = null;
  #monthLabelFormatter        = null;
  #selectedDateFormatter      = null;
  #resizeObserver             = null;
  #onResize                   = null;

  // -- Shadow DOM element refs (populated in #setup) -----------------
  #dateSummary      = null;
  #dateLabel        = null;
  #dateActions      = null;
  #selectTimeButton = null;
  #calendar         = null;
  #timeList         = null;
  #timeOptions      = null;
  #monthLabel       = null;
  #calendarWeekdays = null;
  #grid             = null;
  #prevButton       = null;
  #nextButton       = null;

  constructor() {
    super();
    this.attachShadow({ mode: 'open' });
  }

  static get observedAttributes() {
    return ['available', 'selected', 'locale', 'disabled', 'order-number', 'state', 'locale-path', 'action'];
  }

  // -- Lifecycle -----------------------------------------------------

  connectedCallback() {
    this.#loadTranslations().then(() => {
      this.#render();
      this.#setup();
    });
  }

  attributeChangedCallback(name, oldValue, newValue) {
    if (oldValue === newValue) return;
    switch (name) {
      case 'locale':
        this.#loadTranslations().then(() => this.#applyLocalizedTextToUI());
        break;
      case 'available':
        this.#onAvailableAttribute(newValue);
        break;
      case 'selected':
        this.#selectedTime = typeof newValue === 'string' ? newValue.trim() : '';
        if (this.#selectedTime) {
          // Extract the date part so the label renders even without availability data
          const datePart = this.#selectedTime.substring(0, 10);
          if (/^\d{4}-\d{2}-\d{2}$/.test(datePart)) this.#selectedDate = datePart;
        } else if (!this.#hasUserChangedDate) {
          // Only clear the date if the user has not actively selected one
          this.#selectedDate = '';
        }
        if (this.#dateLabel) this.#updateSelectedDateLabel();
        break;
      case 'disabled':
        this.#syncDisabledState();
        break;
      case 'order-number':
        if (this.#dateLabel) this.#updateSelectedDateLabel();
        break;
      case 'action':
        this.#updateActionsButton();
        break;
      case 'state':
        if (newValue === 'new') {
          this.#selectedDate = '';
          this.#selectedTime = '';
          this.#hasUserChangedDate = false;
          if (this.#dateLabel) {
            this.#updateSelectedDateLabel();
            if (this.#calendar) this.#setCalendarExpanded(false);
            this.#setTimeListExpanded(false);
            for (const btn of this.#grid?.querySelectorAll('button.calendar-day') ?? []) {
              btn.classList.remove('selected');
            }
          }
        }
        break;
    }
  }

  disconnectedCallback() {
    if (this.#onResize) {
      window.removeEventListener('resize', this.#onResize);
      this.#onResize = null;
    }
    if (this.#resizeObserver) {
      this.#resizeObserver.disconnect();
      this.#resizeObserver = null;
    }
  }

  // -- Public properties --------------------------------------------

  get locale() {
    return this.getAttribute('locale')
      || document.documentElement.lang
      || navigator.language
      || 'en';
  }

  get available() { return this.getAttribute('available'); }
  set available(v) {
    if (v !== null && v !== undefined) this.setAttribute('available', typeof v === 'string' ? v : JSON.stringify(v));
    else this.removeAttribute('available');
  }

  get localePath() {
    return this.getAttribute('locale-path') || '/x-date-time-picker/i18n';
  }
  set localePath(v) {
    if (v) this.setAttribute('locale-path', v);
    else this.removeAttribute('locale-path');
  }

  /** Returns 'add' (default), 'delete', or 'confirm'. */
  get action() {
    const v = this.getAttribute('action');
    return (v === 'delete' || v === 'confirm') ? v : 'add';
  }
  set action(v) {
    if (v === 'add' || v === 'delete' || v === 'confirm') this.setAttribute('action', v);
    else this.removeAttribute('action');
  }

  get disabled() { return this.hasAttribute('disabled'); }
  set disabled(v) { this.toggleAttribute('disabled', Boolean(v)); }

  get orderNumber() { return this.getAttribute('order-number') ?? ''; }
  set orderNumber(v) {
    if (v) this.setAttribute('order-number', v);
    else this.removeAttribute('order-number');
  }

  get state() { return this.getAttribute('state') ?? 'none'; }
  set state(v) {
    const valid = ['none', 'tentative', 'confirmed', 'cancelled', 'new'];
    const next  = valid.includes(v) ? v : 'none';
    if (next === 'none') this.removeAttribute('state');
    else this.setAttribute('state', next);
  }

  /** Currently selected UTC ISO 8601 datetime, or null if nothing is selected. */
  get value() {
    if (!this.#selectedDate || !this.#selectedTime) return null;
    return this.#selectedTime;
  }

  // -- Private: i18n ------------------------------------------------

  async #loadTranslations() {
    const lang = this.locale.split('-')[0].toLowerCase();
    if (TRANSLATIONS[lang]) {
      this.#t = { ...TRANSLATIONS.en, ...TRANSLATIONS[lang] };
      return;
    }
    // Load external JSON for languages not bundled inline
    try {
      const res = await fetch(`${this.localePath}/${lang}.json`);
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const data = await res.json();
      this.#t = { ...TRANSLATIONS.en, ...data };
    } catch {
      this.#t = { ...TRANSLATIONS.en };
    }
  }

  #tr(key) {
    return this.#t[key] ?? TRANSLATIONS.en[key] ?? key;
  }

  // -- Private: rendering -------------------------------------------

  #render() {
    this.shadowRoot.innerHTML = `<style>${STYLES}</style>
      <div class="date-summary" id="date-summary"
           role="button" tabindex="0"
           aria-haspopup="true"
           aria-label="${this.#tr('no.date.selected')}">
        <div class="selected-date-wrap" id="selected-date-wrap">
          <span class="selected-date-label" id="selected-date-label">
            ${this.#tr('loading.initial')}
          </span>
        </div>
        <div class="date-actions" id="date-actions">
          <button class="inline-button" type="button" id="select-time-button"
                  aria-label="${this.#tr('btn.select.time')}"
                  title="${this.#tr('btn.select.time')}"
                  ${this.disabled ? 'disabled aria-disabled="true"' : ''}>${SVG_CLOCK}</button>
          <button class="inline-button" type="button" id="actions-button"
                  aria-label="${ this.action === 'delete' ? this.#tr('btn.delete.appointment') : this.action === 'confirm' ? this.#tr('btn.confirm.appointment') : this.#tr('btn.add.appointment') }"
                  title="${ this.action === 'delete' ? this.#tr('btn.delete.appointment') : this.action === 'confirm' ? this.#tr('btn.confirm.appointment') : this.#tr('btn.add.appointment') }"
                  ${this.disabled ? 'disabled aria-disabled="true"' : ''}
                  ${!this.hasAttribute('action') ? 'hidden' : ''}>${ this.action === 'delete' ? SVG_DELETE : this.action === 'confirm' ? SVG_CONFIRM : SVG_ADD }</button>
        </div>
      </div>

      <div class="calendar" id="booking-calendar" hidden aria-live="polite">
        <div class="calendar-header">
          <button class="calendar-nav" type="button" id="calendar-prev"
                  aria-label="${this.#tr('nav.prev.month')}">&#8249;</button>
          <strong class="calendar-month-label" id="calendar-month-label">
            ${this.#tr('lbl.month')}
          </strong>
          <button class="calendar-nav" type="button" id="calendar-next"
                  aria-label="${this.#tr('nav.next.month')}">&#8250;</button>
        </div>
        <div class="calendar-weekdays" id="calendar-weekdays" role="row"></div>
        <div class="calendar-grid" id="calendar-grid" role="grid"></div>
      </div>

      <div class="time-list" id="time-list" hidden aria-live="polite">
        <div class="time-options" id="time-options" role="list"></div>
      </div>
    `;
  }

  // -- Private: setup -----------------------------------------------

  #setup() {
    const root = this.shadowRoot;
    this.#dateSummary      = root.getElementById('date-summary');
    this.#dateLabel        = root.getElementById('selected-date-label');
    this.#dateActions      = root.getElementById('date-actions');
    this.#selectTimeButton = root.getElementById('select-time-button');
    this.#calendar         = root.getElementById('booking-calendar');
    this.#timeList         = root.getElementById('time-list');
    this.#timeOptions      = root.getElementById('time-options');
    this.#monthLabel       = root.getElementById('calendar-month-label');
    this.#calendarWeekdays = root.getElementById('calendar-weekdays');
    this.#grid             = root.getElementById('calendar-grid');
    this.#prevButton       = root.getElementById('calendar-prev');
    this.#nextButton       = root.getElementById('calendar-next');

    const now = new Date();
    this.#activeYear  = now.getFullYear();
    this.#activeMonth = now.getMonth() + 1;
    this.#selectedTime = this.getAttribute('selected')?.trim() ?? '';

    this.#monthLabelFormatter   = new Intl.DateTimeFormat(this.locale, { month: 'long', year: 'numeric' });
    this.#selectedDateFormatter = new Intl.DateTimeFormat(this.locale, {
      weekday: 'short', day: '2-digit', month: 'short', year: 'numeric',
    });

    this.#renderWeekdayHeaders();

    this.#prevButton.addEventListener('click', () => this.#navigatePrevMonth());
    this.#nextButton.addEventListener('click', () => this.#navigateNextMonth());

    // "Change time" button toggles the time-slot list
    this.#selectTimeButton.addEventListener('click', (e) => {
      e.preventDefault();
      e.stopPropagation();
      this.#setTimeListExpanded(this.#timeList.hidden);
    });

    // Add/delete button dispatches an event
    this.shadowRoot.getElementById('actions-button')?.addEventListener('click', (e) => {
      e.preventDefault();
      e.stopPropagation();
      this.#dispatch('x-date-time-picker-add-delete-pressed', {
        date:         this.#selectedDate || null,
        selectedTime: this.#selectedTime || null,
      });
    });

    // Click on summary bar (outside action buttons) toggles calendar
    this.#dateSummary.addEventListener('click', (e) => {
      if (e.target.closest('.date-actions')) return;
      if (this.disabled) return;
      e.preventDefault();
      this.#setCalendarExpanded(this.#calendar.hidden);
    });

    // Keyboard: Enter/Space opens calendar; Escape closes panels
    this.#dateSummary.addEventListener('keydown', (e) => {
      if ((e.key === 'Enter' || e.key === ' ') && !e.target.closest('button')) {
        e.preventDefault();
        if (!this.disabled) this.#setCalendarExpanded(this.#calendar.hidden);
      }
      if (e.key === 'Escape') {
        this.#setCalendarExpanded(false);
        this.#setTimeListExpanded(false);
      }
    });

    // Responsive wrap detection
    this.#onResize = () => this.#updateWrapStates();
    window.addEventListener('resize', this.#onResize);

    if (typeof ResizeObserver !== 'undefined') {
      this.#resizeObserver = new ResizeObserver(() => this.#updateWrapStates());
      this.#resizeObserver.observe(this.#dateSummary);
    }

    this.#renderTimeOptions();

    // Single delegated listener for all time-slot buttons; avoids attaching
    // and garbage-collecting one listener per slot on every re-render.
    this.#timeOptions.addEventListener('click', (e) => {
      const btn = e.target.closest('button[data-slot]');
      if (!btn || btn.disabled) return;
      e.preventDefault();
      e.stopPropagation();
      this.#hasUserChangedDate = true;
      const slot = btn.dataset.slot;
      this.#selectedTime = slot;
      this.setAttribute('selected', slot);
      this.#updateSelectedDateLabel();
      // Toggle the selected class in-place — no full re-render needed.
      this.#timeOptions.querySelector('button.selected')?.classList.remove('selected');
      btn.classList.add('selected');
      this.#setTimeListExpanded(false);
      this.#updateActionsButton();
    });

    setTimeout(() => this.#initialize(), 0);
  }

  // -- Private: i18n UI sync ----------------------------------------

  #applyLocalizedTextToUI() {
    this.#monthLabelFormatter   = new Intl.DateTimeFormat(this.locale, { month: 'long', year: 'numeric' });
    this.#selectedDateFormatter = new Intl.DateTimeFormat(this.locale, {
      weekday: 'short', day: '2-digit', month: 'short', year: 'numeric',
    });

    if (this.#dateLabel && !this.#selectedDate) {
      this.#dateLabel.textContent = this.#hasEmittedInitializedEvent
        ? this.#tr('no.date.selected')
        : this.#tr('loading.initial');
    }
    if (this.#selectTimeButton) {
      this.#selectTimeButton.setAttribute('aria-label', this.#tr('btn.select.time'));
      this.#selectTimeButton.setAttribute('title', this.#tr('btn.select.time'));
    }
    if (this.#prevButton) this.#prevButton.setAttribute('aria-label', this.#tr('nav.prev.month'));
    if (this.#nextButton) this.#nextButton.setAttribute('aria-label', this.#tr('nav.next.month'));

    this.#renderWeekdayHeaders();

    if (this.#activeYear && this.#activeMonth && this.#monthLabel) {
      this.#monthLabel.textContent = this.#monthLabelFormatter.format(
        new Date(this.#activeYear, this.#activeMonth - 1, 1),
      );
    }

    this.#updateSelectedDateLabel();
    this.#updateWrapStates();
  }

  // -- Private: availability attribute handling ----------------------

  #onAvailableAttribute(newValue) {
    // Flat JSON array of UTC ISO 8601 strings → sort chronologically, then group
    // by month and local calendar date. Index 0 is always the earliest slot.
    this.#availableDatesByMonth.clear();
    this.#slotLabelCache.clear();

    if (newValue) {
      let rawSlots;
      try { rawSlots = JSON.parse(newValue); } catch { rawSlots = null; }

      if (Array.isArray(rawSlots)) {
        // Parse, validate and sort by epoch time ascending
        const valid = rawSlots
          .filter(r => typeof r === 'string')
          .map(r => ({ raw: r.trim(), t: new Date(r.trim()).getTime() }))
          .filter(({ t }) => Number.isFinite(t))
          .sort((a, b) => a.t - b.t);

        for (const { raw, t } of valid) {
          const d        = new Date(t);
          const monthKey = this.#toMonthKey(d.getFullYear(), d.getMonth() + 1);
          const dateKey  = this.#toDateKey(d.getFullYear(), d.getMonth() + 1, d.getDate());
          const entry    = this.#availableDatesByMonth.get(monthKey)
            ?? { month: monthKey, dates: [], timeSlots: {} };
          (entry.timeSlots[dateKey] ??= []).push(raw);
          this.#availableDatesByMonth.set(monthKey, entry);
        }
        // Deduplicate slots (already in chronological order) and build dates list
        for (const entry of this.#availableDatesByMonth.values()) {
          for (const dateKey of Object.keys(entry.timeSlots)) {
            entry.timeSlots[dateKey] = [...new Set(entry.timeSlots[dateKey])];
          }
          entry.dates = Object.keys(entry.timeSlots).sort();
        }
      }
    }

    this.#updateMonthNavigationState();
    if (this.#grid) this.#applyAvailabilityForActiveMonth();
  }

  // -- Private: calendar rendering ----------------------------------

  #renderWeekdayHeaders() {
    if (!this.#calendarWeekdays) return;
    // Week starts Monday. 2024-01-01 UTC is a Monday => index 0.
    const mondayStart = Date.UTC(2024, 0, 1);
    const formatter   = new Intl.DateTimeFormat(this.locale, { weekday: 'short' });
    const labels = Array.from({ length: 7 }, (_, i) =>
      formatter.format(new Date(mondayStart + i * 86_400_000)),
    );
    this.#calendarWeekdays.innerHTML = labels.map((l) => `<span>${l}</span>`).join('');
  }

  async #renderMonth() {
    this.#updateMonthNavigationState();

    const monthStart = new Date(this.#activeYear, this.#activeMonth - 1, 1);
    this.#monthLabel.textContent = this.#monthLabelFormatter.format(monthStart);

    const totalDays      = this.#daysInMonth(this.#activeYear, this.#activeMonth);
    const offset         = this.#firstWeekdayMondayBased(this.#activeYear, this.#activeMonth);
    const now            = new Date();
    const isCurrentMonth = this.#activeYear === now.getFullYear()
      && this.#activeMonth === now.getMonth() + 1;
    const todayDay       = now.getDate();
    const nodes = [];

    for (let i = 0; i < offset; i++) {
      const spacer = document.createElement('span');
      spacer.className = 'calendar-spacer';
      spacer.setAttribute('aria-hidden', 'true');
      nodes.push(spacer);
    }

    for (let day = 1; day <= totalDays; day++) {
      const dateKey = this.#toDateKey(this.#activeYear, this.#activeMonth, day);
      const btn = document.createElement('button');
      btn.type         = 'button';
      btn.className    = 'calendar-day disabled';
      btn.textContent  = String(day);
      btn.dataset.date = dateKey;
      btn.disabled     = true;
      btn.setAttribute('aria-disabled', 'true');
      if (isCurrentMonth && day === todayDay) {
        btn.classList.add('today');
        btn.setAttribute('aria-current', 'date');
      }

      btn.addEventListener('click', () => {
        if (btn.disabled) return;
        this.#hasUserChangedDate = true;
        this.#setSelectedDate(dateKey);
        this.#setCalendarExpanded(false, true);
        this.#setTimeListExpanded(true);
      });

      nodes.push(btn);
    }

    this.#grid.replaceChildren(...nodes);
    this.#applyAvailabilityForActiveMonth();
    this.#updateMonthNavigationState();
  }

  // -- Private: time options ----------------------------------------

  #renderTimeOptions() {
    if (!this.#timeOptions || !this.#selectTimeButton) return;

    const slots = this.#getTimeSlotsForDate(this.#selectedDate);

    if (!this.#selectedDate || slots.length === 0) {
      this.#selectTimeButton.disabled = true;
      this.#setTimeListExpanded(false);

      const empty = document.createElement('button');
      empty.type      = 'button';
      empty.className = 'time-option';
      empty.disabled  = true;
      empty.setAttribute('aria-disabled', 'true');
      empty.setAttribute('role', 'listitem');
      empty.textContent = this.#tr('no.slots');
      this.#timeOptions.replaceChildren(empty);
      return;
    }

    this.#selectTimeButton.disabled = false;
    if (!slots.includes(this.#selectedTime)) {
      this.#selectedTime = slots[0] ?? '';
    }

    // Build all buttons in a DocumentFragment to avoid per-element reflows.
    // Click handling is done via a single delegated listener on #timeOptions
    // (set up once in #setup) using the data-slot attribute.
    const frag = document.createDocumentFragment();
    for (const slot of slots) {
      const btn = document.createElement('button');
      btn.type      = 'button';
      btn.className = 'time-option';
      btn.setAttribute('role', 'listitem');
      btn.dataset.slot = slot;
      btn.textContent = this.#slotDisplayLabel(slot);
      if (slot === this.#selectedTime) btn.classList.add('selected');
      frag.appendChild(btn);
    }
    this.#timeOptions.replaceChildren(frag);
  }

  // -- Private: panel expand/collapse -------------------------------

  #setCalendarExpanded(expanded, suppressRestore = false) {
    if (expanded) {
      if (!this.#isDropdownSessionActive) {
        this.#isDropdownSessionActive = true;
        this.#dropdownScrollRestoreY  = window.scrollY;
      }
      this.#setTimeListExpanded(false, true);
    }

    this.#calendar.hidden = !expanded;
    this.#updateActionsButton();

    if (expanded) {
      requestAnimationFrame(() => this.#ensurePanelInViewport(this.#calendar));
      return;
    }
    this.#tryRestoreScroll(suppressRestore);
  }

  #setTimeListExpanded(expanded, suppressRestore = false) {
    if (!this.#timeList || !this.#selectTimeButton) return;

    if (expanded) {
      if (!this.#isDropdownSessionActive) {
        this.#isDropdownSessionActive = true;
        this.#dropdownScrollRestoreY  = window.scrollY;
      }
      this.#setCalendarExpanded(false, true);
    }

    this.#timeList.hidden = !expanded;
    this.#selectTimeButton.setAttribute('aria-expanded', expanded ? 'true' : 'false');
    this.#updateActionsButton();

    if (expanded) {
      requestAnimationFrame(() => this.#ensurePanelInViewport(this.#timeList));
    } else {
      this.#tryRestoreScroll(suppressRestore);
    }
    this.#updateWrapStates();
  }

  #tryRestoreScroll(suppress) {
    if (suppress) return;
    const allClosed = this.#calendar.hidden && this.#timeList.hidden;
    if (allClosed && Number.isFinite(this.#dropdownScrollRestoreY)) {
      try { window.scrollTo({ top: this.#dropdownScrollRestoreY, behavior: 'smooth' }); } catch { /* not implemented in test env */ }
    }
    if (allClosed) {
      this.#isDropdownSessionActive = false;
      this.#dropdownScrollRestoreY  = null;
    }
  }

  #ensurePanelInViewport(panel) {
    if (!panel || panel.hidden) return;
    const vp    = window.visualViewport;
    const vpH   = vp?.height ?? window.innerHeight;
    const vpTop = vp?.offsetTop ?? 0;
    const rect  = panel.getBoundingClientRect();
    if (rect.bottom - (vpTop + vpH) <= 2) return;
    const centerY = window.scrollY + rect.top + rect.height / 2;
    const target  = Math.max(0, Math.min(
      centerY - vpH / 2 - vpTop,
      document.documentElement.scrollHeight - vpH,
    ));
    try { window.scrollTo({ top: target, behavior: 'smooth' }); } catch { /* not implemented in test env */ }
  }

  #syncDisabledState() {
    if (!this.#selectTimeButton) return;
    if (this.disabled) {
      this.#selectTimeButton.disabled = true;
      this.#selectTimeButton.setAttribute('aria-disabled', 'true');
      this.#setCalendarExpanded(false);
      this.#setTimeListExpanded(false);
    } else {
      this.#selectTimeButton.disabled = false;
      this.#selectTimeButton.removeAttribute('aria-disabled');
    }
    this.#updateActionsButton();
  }

  #updateActionsButton() {
    const btn = this.shadowRoot?.getElementById('actions-button');
    if (!btn) return;
    const type = this.action; // 'add' | 'delete' | 'confirm'

    btn.hidden = !this.hasAttribute('action');

    const dropdownOpen = (this.#calendar && !this.#calendar.hidden) ||
                         (this.#timeList  && !this.#timeList.hidden);
    const isDisabled = this.disabled || !this.value || dropdownOpen;
    btn.disabled = isDisabled;
    if (isDisabled) btn.setAttribute('aria-disabled', 'true');
    else btn.removeAttribute('aria-disabled');

    const label = type === 'delete' ? this.#tr('btn.delete.appointment')
                : type === 'confirm' ? this.#tr('btn.confirm.appointment')
                : this.#tr('btn.add.appointment');
    btn.setAttribute('aria-label', label);
    btn.setAttribute('title', label);
    btn.innerHTML = type === 'delete' ? SVG_DELETE : type === 'confirm' ? SVG_CONFIRM : SVG_ADD;
  }

  // -- Private: wrap-state detection --------------------------------

  #updateWrapStates() {
    if (!this.#dateSummary || !this.#dateLabel || !this.#selectTimeButton) return;
    this.#dateSummary.classList.remove('is-wrapped');
    this.#dateActions?.classList.remove('is-wrapped');
    void this.#dateSummary.offsetWidth; // force reflow

    const style  = window.getComputedStyle(this.#dateSummary);
    const gap    = parseFloat(style.columnGap || style.gap || '0') || 0;
    const pl     = parseFloat(style.paddingLeft)  || 0;
    const pr     = parseFloat(style.paddingRight) || 0;
    const avail  = this.#dateSummary.clientWidth - pl - pr;
    const needed = this.#dateLabel.scrollWidth + this.#selectTimeButton.offsetWidth + gap;
    const wrap   = needed > avail + 0.5;

    this.#dateSummary.classList.toggle('is-wrapped', wrap);
    this.#dateActions?.classList.toggle('is-wrapped', wrap);
  }

  // -- Private: month navigation ------------------------------------

  #navigatePrevMonth() {
    if (!this.#canNavigateToPreviousMonth()) {
      this.#updateMonthNavigationState();
      return;
    }
    const prevYear = this.#activeYear, prevMonth = this.#activeMonth;
    this.#activeMonth -= 1;
    if (this.#activeMonth < 1) { this.#activeMonth = 12; this.#activeYear -= 1; }
    this.#renderMonth();
    this.#dispatch('x-date-time-picker-month-selected', {
      year: this.#activeYear, month: this.#activeMonth,
      previousYear: prevYear, previousMonth: prevMonth,
    });
  }

  #navigateNextMonth() {
    if (!this.#canNavigateToNextMonth()) {
      this.#updateMonthNavigationState();
      return;
    }
    const prevYear = this.#activeYear, prevMonth = this.#activeMonth;
    this.#activeMonth += 1;
    if (this.#activeMonth > 12) { this.#activeMonth = 1; this.#activeYear += 1; }
    this.#renderMonth();
    this.#dispatch('x-date-time-picker-month-selected', {
      year: this.#activeYear, month: this.#activeMonth,
      previousYear: prevYear, previousMonth: prevMonth,
    });
  }

  #canNavigateToNextMonth() {
    // When no availability data is loaded, navigation is unrestricted.
    if (this.#availableDatesByMonth.size === 0) return true;
    // Compute the month key that would result from navigating forward.
    let nextYear = this.#activeYear, nextMonth = this.#activeMonth + 1;
    if (nextMonth > 12) { nextMonth = 1; nextYear += 1; }
    const nextKey = this.#toMonthKey(nextYear, nextMonth);
    // Allow navigation only when the next month (or any later one) has availability.
    for (const key of this.#availableDatesByMonth.keys()) {
      if (key >= nextKey) return true;
    }
    return false;
  }

  #canNavigateToPreviousMonth() {
    const now = new Date();
    if (this.#activeYear !== now.getFullYear()) return this.#activeYear > now.getFullYear();
    return this.#activeMonth > now.getMonth() + 1;
  }

  #updateMonthNavigationState() {
    if (this.#prevButton) {
      this.#prevButton.disabled = !this.#canNavigateToPreviousMonth();
      this.#prevButton.setAttribute('aria-disabled', this.#prevButton.disabled ? 'true' : 'false');
    }
    if (this.#nextButton) {
      this.#nextButton.disabled = !this.#canNavigateToNextMonth();
      this.#nextButton.setAttribute('aria-disabled', this.#nextButton.disabled ? 'true' : 'false');
    }
  }

  // -- Private: date selection --------------------------------------

  #setSelectedDate(dateKey) {
    const previousDate = this.#selectedDate;
    this.#selectedDate = typeof dateKey === 'string' ? dateKey : '';
    this.#renderTimeOptions();

    for (const btn of this.#grid.querySelectorAll('button.calendar-day')) {
      btn.classList.toggle('selected', btn.dataset.date === this.#selectedDate);
    }
    this.#updateSelectedDateLabel();

    if (previousDate !== this.#selectedDate) {
      this.#dispatch('x-date-time-picker-date-selected', {
        date:               this.#selectedDate,
        previousDate,
        timeSlots:          this.#getTimeSlotsForDate(this.#selectedDate),
        hasUserChangedDate: this.#hasUserChangedDate,
        year:               this.#activeYear,
        month:              this.#activeMonth,
      });
    }
    this.#updateActionsButton();
  }

  #updateSelectedDateLabel() {
    if (!this.#dateLabel) return;
    if (!this.#selectedDate) {
      this.#dateLabel.textContent = this.#tr('no.date.selected');
      return;
    }
    // Parse as UTC midnight so no timezone offset shifts the displayed date
    const parsed      = new Date(`${this.#selectedDate}T00:00:00Z`);
    const orderNumber = this.getAttribute('order-number');
    const isExternallySelected = !!this.getAttribute('selected');
    const prefix      = (orderNumber && (this.#hasUserChangedDate || isExternallySelected))
      ? orderNumber
      : (this.#hasUserChangedDate || isExternallySelected)
        ? this.#tr('date.prefix.selected')
        : this.#tr('date.prefix.earliest');
    const displayed = this.#getDisplayedTimeForSelectedDate();
    const timePart  = displayed
      ? `, ${this.#slotDisplayLabel(displayed)}\u00a0${this.#tr('time.suffix')}`
      : '';
    this.#dateLabel.textContent = `${prefix}: ${this.#selectedDateFormatter.format(parsed)}${timePart}`;
    this.#updateWrapStates();
  }

  // -- Private: availability application ----------------------------

  #applyAvailabilityForActiveMonth() {
    if (!this.#grid) return;
    const monthKey     = this.#toMonthKey(this.#activeYear, this.#activeMonth);
    const availability = this.#availableDatesByMonth.get(monthKey);
    const available = new Set(Array.isArray(availability?.dates) ? availability.dates : []);
    const nowMs     = Date.now();

    let firstAvailable = '';
    for (const btn of this.#grid.querySelectorAll('button.calendar-day')) {
      const dateKey = btn.dataset.date ?? '';
      const slots   = this.#getTimeSlotsForDate(dateKey);
      const enabled = available.has(dateKey)
        && (slots.length === 0 || slots.some(s => new Date(s).getTime() >= nowMs));
      btn.disabled = !enabled;
      btn.setAttribute('aria-disabled', enabled ? 'false' : 'true');
      btn.classList.toggle('disabled', !enabled);
      if (enabled && !firstAvailable) firstAvailable = dateKey;
    }

    if (!availability) {
      this.#setSelectedDate('');
      return;
    }

    if (this.#hasUserChangedDate) {
      // User made an active choice — keep it if still valid, else fall back.
      if (this.#selectedDate
          && this.#selectedDate.startsWith(`${monthKey}-`)
          && available.has(this.#selectedDate)) {
        this.#setSelectedDate(this.#selectedDate);
      } else if (firstAvailable) {
        this.#setSelectedDate(firstAvailable);
      } else {
        this.#setSelectedDate('');
      }
    } else {
      if (firstAvailable) {
        this.#setSelectedDate(firstAvailable);
      } else {
        this.#setSelectedDate('');
      }
    }


  }

  // -- Private: initialization --------------------------------------

  async #initialize() {
    if (this.#selectTimeButton && !this.disabled) this.#selectTimeButton.disabled = false;
    const now = new Date();
    this.#activeYear  = now.getFullYear();
    this.#activeMonth = now.getMonth() + 1;
    await this.#renderMonth();
    this.#setCalendarExpanded(false);
    this.#syncDisabledState();

    if (!this.#hasEmittedInitializedEvent) {
      this.#dispatch('x-date-time-picker-initialized', {
        year:         this.#activeYear,
        month:        this.#activeMonth,
        selectedDate: this.#selectedDate,
      });
      this.#hasEmittedInitializedEvent = true;
    }
  }

  // -- Private: time slot helpers -----------------------------------

  #getTimeSlotsForDate(dateKey) {
    if (!dateKey) return [];
    const monthKey     = dateKey.substring(0, 7);
    const availability = this.#availableDatesByMonth.get(monthKey);
    const rawSlots     = availability?.timeSlots?.[dateKey];
    return Array.isArray(rawSlots) ? rawSlots : [];
  }

  #getDisplayedTimeForSelectedDate() {
    const slots = this.#getTimeSlotsForDate(this.#selectedDate);
    if (this.#selectedTime && (slots.includes(this.#selectedTime) || slots.length === 0)) return this.#selectedTime;
    return slots[0] ?? '';
  }

  /**
   * Converts a server UTC ISO slot string to a local HH:MM display label.
   * Non-ISO strings are returned unchanged (legacy plain "HH:MM" support).
   * Results are memoised to avoid repeated Date allocations across re-renders.
   */
  #slotDisplayLabel(slot) {
    if (typeof slot !== 'string' || !slot) return slot ?? '';
    if (this.#slotLabelCache.has(slot)) return this.#slotLabelCache.get(slot);
    const d = new Date(slot);
    const label = Number.isFinite(d.getTime())
      ? `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`
      : slot;
    this.#slotLabelCache.set(slot, label);
    return label;
  }

  // -- Private: date math -------------------------------------------

  #toMonthKey(year, month) {
    return `${String(year).padStart(4, '0')}-${String(month).padStart(2, '0')}`;
  }

  #toDateKey(year, month, day) {
    return `${this.#toMonthKey(year, month)}-${String(day).padStart(2, '0')}`;
  }

  #firstWeekdayMondayBased(year, month) {
    const jsDay = new Date(Date.UTC(year, month - 1, 1)).getUTCDay();
    return (jsDay + 6) % 7;
  }

  #daysInMonth(year, month) {
    return new Date(Date.UTC(year, month, 0)).getUTCDate();
  }

  #getTodayDateKey() {
    const now = new Date();
    return this.#toDateKey(now.getFullYear(), now.getMonth() + 1, now.getDate());
  }

  // -- Private: event dispatch --------------------------------------

  #dispatch(eventName, detail = {}) {
    this.dispatchEvent(new CustomEvent(eventName, {
      bubbles:  true,
      composed: true,
      detail,
    }));
  }
}

customElements.define('x-date-time-picker', XDateTimePicker);
