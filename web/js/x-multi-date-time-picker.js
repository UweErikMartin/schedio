// ─── x-multi-date-time-picker ────────────────────────────────────────────────
// A container that manages a dynamic list of x-date-time-picker instances,
// allowing users to select multiple date-and-time appointments from a shared
// availability pool.
// -----------------------------------------------------------------------------

import './x-date-time-picker.js';

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
  }

  *,
  *::before,
  *::after {
    box-sizing: border-box;
  }

  [hidden] { display: none !important; }

  /* -- Picker list -- */
  .picker-list {
    display: flex;
    flex-direction: column;
    gap: var(--space-md);
  }

  /* -- WCAG 2.1 AA focus ring -- */
  :focus-visible {
    outline: 2px solid var(--color-primary);
    outline-offset: 2px;
  }


`;

/** Bundled translations — English (default) and German. */
const TRANSLATIONS = {
  en: {
    'list.label': 'Appointment list',
  },
  de: {
    'list.label': 'Terminliste',
  },
};

/**
 * XMultiDateTimePicker — manages a dynamic list of x-date-time-picker instances.
 *
 * Attributes:
 *   available         (string)  — flat JSON array of UTC ISO 8601 datetime strings; propagated to all child pickers.
 *   locale            (string)  — BCP 47 tag; propagated to all child pickers.
 *   disabled          (boolean) — disables all child pickers and the add button.
 *   max-items         (number)  — maximum number of pickers allowed (0 = unlimited).
 *   initial-bookings  (string)  — JSON { bookings: Booking[] } that pre-populates the list.
 *                                 Each Booking: { id, start_at, end_at, state, cancel_reason,
 *                                                can_cancel, can_reschedule }
 *
 * Properties:
 *   values          (string|null)[] — array of selected ISO datetimes from all child pickers.
 *   selectedSlots   string[]        — sorted flat array of non-null selected ISO strings.
 *   initialBookings object|null     — reflects the initial-bookings attribute as a parsed object.
 *   disabled        (boolean)       — reflects the `disabled` boolean attribute.
 *   locale          (string)        — reflects the `locale` attribute with fallback chain.
 *   maxItems        (number)        — reflects the `max-items` attribute (0 = unlimited).
 *
 * Events (all: bubbles=true, composed=true):
 *   x-multi-date-time-picker-change        { values }                             — on every selection change
 *   x-multi-date-time-picker-slots-changed { slots: string[] }                    — when sorted flat list changes
 *   x-multi-date-time-picker-item-added    { index, count }
 *   x-multi-date-time-picker-item-removed  { index, count, removedValue, bookingId }
 */
export class XMultiDateTimePicker extends HTMLElement {
  // -- Private state -------------------------------------------------
  #t          = {};
  #pickerList = null;

  /** flat array of all available UTC ISO 8601 datetime strings */
  #masterSlots = [];

  /** picker element → currently selected ISO string | null */
  #pickerSelections   = new Map();

  /** picker element → MutationObserver watching selected attribute */
  #slotObservers      = new Map();

  /** Last emitted sorted flat list of selected ISO strings (used to detect real changes). */
  #lastEmittedSlots   = [];

  /** picker element → booking object (or null for manually-added pickers) */
  #pickerBookings     = new WeakMap();

  constructor() {
    super();
    this.attachShadow({ mode: 'open' });
  }

  static get observedAttributes() {
    return ['available', 'locale', 'disabled', 'max-items', 'initial-bookings'];
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
        this.#loadTranslations().then(() => this.#applyLocaleToUI());
        break;
      case 'available':
        if (newValue === null) {
          this.#masterSlots = [];
          this.#propagateAttributeToChildren('available', null);
        } else {
          try {
            const parsed = JSON.parse(newValue);
            if (Array.isArray(parsed)) {
              this.#masterSlots = parsed;
              this.#redistributeAvailability();
            }
          } catch { /* ignore invalid JSON */ }
        }
        break;
      case 'disabled':
        this.#syncDisabledState();
        break;
      case 'max-items':
        this.#updatePickerActions();
        break;
      case 'initial-bookings':
        if (this.#pickerList) this.#applyBookings(newValue);
        break;
    }
  }

  // -- Public properties --------------------------------------------

  get locale() {
    return this.getAttribute('locale')
      || document.documentElement.lang
      || navigator.language
      || 'en';
  }

  get available() { return this.getAttribute('available') ?? ''; }
  set available(v) {
    if (v) this.setAttribute('available', v);
    else this.removeAttribute('available');
  }

  get initialBookings() {
    try { return JSON.parse(this.getAttribute('initial-bookings') ?? 'null'); } catch { return null; }
  }
  set initialBookings(v) {
    if (v !== null && v !== undefined) this.setAttribute('initial-bookings', JSON.stringify(v));
    else this.removeAttribute('initial-bookings');
  }

  get disabled() { return this.hasAttribute('disabled'); }
  set disabled(v) { this.toggleAttribute('disabled', Boolean(v)); }

  /**
   * Maximum number of date-time pickers.
   * 0 means unlimited (attribute absent or non-positive value).
   */
  get maxItems() {
    const v = parseInt(this.getAttribute('max-items'), 10);
    return Number.isFinite(v) && v > 0 ? v : 0;
  }
  set maxItems(v) {
    const n = parseInt(v, 10);
    if (Number.isFinite(n) && n > 0) this.setAttribute('max-items', String(n));
    else this.removeAttribute('max-items');
  }

  /**
   * Array of currently selected ISO 8601 datetime strings,
   * one entry per child picker (null when a picker has no selection).
   */
  get values() {
    return this.#getPickers().map((p) => p.value);
  }

  /**
   * Sorted flat array of all non-null ISO 8601 datetime strings currently selected
   * across all child pickers.
   */
  get selectedSlots() {
    return this.#getPickers()
      .map(p => p.value)
      .filter(v => v !== null)
      .sort((a, b) => new Date(a).getTime() - new Date(b).getTime());
  }

  // -- Private: i18n ------------------------------------------------

  async #loadTranslations() {
    const lang = this.locale.split('-')[0].toLowerCase();
    this.#t = TRANSLATIONS[lang]
      ? { ...TRANSLATIONS.en, ...TRANSLATIONS[lang] }
      : { ...TRANSLATIONS.en };
  }

  #tr(key) {
    return this.#t[key] ?? TRANSLATIONS.en[key] ?? key;
  }

  // -- Private: rendering -------------------------------------------

  #render() {
    this.shadowRoot.innerHTML = `<style>${STYLES}</style>
      <div class="picker-list" id="picker-list"
           role="list"
           aria-label="${this.#tr('list.label')}"></div>
    `;
  }

  // -- Private: setup -----------------------------------------------

  #setup() {
    this.#pickerList = this.shadowRoot.getElementById('picker-list');

    const initialBookingsAttr = this.getAttribute('initial-bookings');
    if (initialBookingsAttr) {
      this.#applyBookings(initialBookingsAttr);
    } else {
      // Start with one empty picker
      this.#addPicker();
    }

    this.#syncDisabledState();
  }

  // -- Private: picker management -----------------------------------

  /**
   * Clears all child pickers and their associated tracking state.
   * Does NOT dispatch any events.
   */
  #clearAllPickers() {
    for (const picker of this.#getPickers()) {
      const obs = this.#slotObservers.get(picker);
      if (obs) { obs.disconnect(); this.#slotObservers.delete(picker); }
      this.#pickerSelections.delete(picker);
    }
    if (this.#pickerList) this.#pickerList.innerHTML = '';
    this.#lastEmittedSlots = [];
  }

  /**
   * Parses a JSON string containing a bookings array and rebuilds the picker list
   * with one pre-populated picker per booking.  Falls back to a single empty picker
   * when the JSON is absent, invalid, or contains an empty array.
   */
  #applyBookings(jsonString) {
    let bookings = [];
    try {
      const parsed = JSON.parse(jsonString ?? 'null');
      bookings = Array.isArray(parsed?.bookings) ? parsed.bookings : [];
    } catch { /* ignore invalid JSON */ }

    this.#clearAllPickers();

    if (bookings.length === 0) {
      this.#addPicker();
      return;
    }

    for (const booking of bookings) {
      this.#addPicker({ booking });
    }
  }

  #addPicker({ booking = null } = {}) {
    if (!this.#pickerList) return;
    const maxItems = this.maxItems;
    const currentPickers = this.#getPickers();
    if (maxItems > 0 && currentPickers.length >= maxItems) return;

    const picker = document.createElement('x-date-time-picker');
    picker.setAttribute('action', 'delete');

    if (booking?.start_at) picker.setAttribute('selected', booking.start_at);
    if (booking?.state)    picker.setAttribute('state', booking.state);
    this.#pickerBookings.set(picker, booking);

    const locale = this.getAttribute('locale');
    if (locale) picker.setAttribute('locale', locale);

    if (this.disabled) picker.toggleAttribute('disabled', true);

    const item = document.createElement('div');
    item.className = 'picker-item';
    item.setAttribute('role', 'listitem');
    item.appendChild(picker);
    this.#pickerList.appendChild(item);

    // Track this picker's selection (starts empty)
    this.#pickerSelections.set(picker, null);

    // Observe selected attribute to catch explicit time-slot changes
    const obs = new MutationObserver(() => {
      const val = picker.value ?? null;
      if (val !== this.#pickerSelections.get(picker)) {
        this.#pickerSelections.set(picker, val);
        this.#redistributeAvailability();
        this.#dispatch('x-multi-date-time-picker-change', { values: this.values });
        this.#notifySlotsChanged();
      }
    });
    obs.observe(picker, { attributes: true, attributeFilter: ['selected'] });
    this.#slotObservers.set(picker, obs);

    // Action button pressed: 'add' action on last picker adds a new one; 'delete' removes it
    picker.addEventListener('x-date-time-picker-add-delete-pressed', () => {
      if (picker.action === 'add') {
        this.#addPicker();
      } else {
        const allPickers   = this.#getPickers();
        const pickerIndex  = allPickers.indexOf(picker);
        const removedValue = picker.value;
        const bookingId    = this.#pickerBookings.get(picker)?.id ?? null;
        // Clean up slot tracking before removal so freed slot is visible to siblings
        const observer = this.#slotObservers.get(picker);
        if (observer) { observer.disconnect(); this.#slotObservers.delete(picker); }
        this.#pickerSelections.delete(picker);
        item.remove();
        this.#updatePickerActions();
        this.#updateAddButtonVisibility();
        this.#redistributeAvailability();
        this.#dispatch('x-multi-date-time-picker-item-removed', {
          index:        pickerIndex,
          count:        this.#getPickers().length,
          removedValue,
          bookingId,
        });
        this.#dispatch('x-multi-date-time-picker-change', { values: this.values });
        this.#notifySlotsChanged();
      }
    });

    // Date selected: record selection, redistribute availability, bubble change event
    picker.addEventListener('x-date-time-picker-date-selected', () => {
      const val = picker.value ?? null;
      this.#pickerSelections.set(picker, val);
      this.#redistributeAvailability();
      this.#dispatch('x-multi-date-time-picker-change', { values: this.values });
      this.#notifySlotsChanged();
    });

    // Initialized: bubble initial change so consumers can read values
    picker.addEventListener('x-date-time-picker-initialized', () => {
      this.#dispatch('x-multi-date-time-picker-change', { values: this.values });
    });

    const newIndex = this.#getPickers().length - 1; // 0-based, picker is already appended
    this.#updatePickerActions();
    this.#updateAddButtonVisibility();
    // Apply current master availability to the newly added picker
    this.#redistributeAvailability();
    this.#dispatch('x-multi-date-time-picker-item-added', {
      index: newIndex,
      count: this.#getPickers().length,
    });
  }

  #getPickers() {
    return Array.from(this.#pickerList?.querySelectorAll('x-date-time-picker') ?? []);
  }

  /**
   * Ensures the last picker shows action="add" and all others show action="delete".
   * When max-items is reached the last picker also gets "delete".
   */
  #updatePickerActions() {
    const pickers  = this.#getPickers();
    const maxItems = this.maxItems;
    const atMax    = maxItems > 0 && pickers.length >= maxItems;
    pickers.forEach((p, i) => {
      const isLast = !atMax && i === pickers.length - 1;
      p.setAttribute('action', isLast ? 'add' : 'delete');
    });
  }

  /**
   * Recomputes each picker's available attribute by filtering the master slot list
   * to exclude slots already selected by sibling pickers.
   */
  #redistributeAvailability() {
    if (this.#masterSlots.length === 0) return;
    const pickers = this.#getPickers();
    for (const picker of pickers) {
      const otherSlots = new Set();
      for (const [p, slot] of this.#pickerSelections) {
        if (p !== picker && slot) otherSlots.add(slot);
      }
      const flatSlots = this.#masterSlots.filter(s => !otherSlots.has(s));
      picker.setAttribute('available', JSON.stringify(flatSlots));
    }
  }

  #propagateAttributeToChildren(name, value) {
    for (const picker of this.#getPickers()) {
      if (value === null) picker.removeAttribute(name);
      else picker.setAttribute(name, value);
    }
  }

  #applyLocaleToUI() {
    this.#propagateAttributeToChildren('locale', this.getAttribute('locale'));
  }

  #syncDisabledState() {
    for (const picker of this.#getPickers()) {
      picker.disabled = this.disabled;
    }
  }

  #updateAddButtonVisibility() {
    // No external add button; kept as a no-op so call sites compile cleanly.
  }

  // -- Private: slots-changed notification ------------------------

  #notifySlotsChanged() {
    const current = this.selectedSlots;
    const last    = this.#lastEmittedSlots;
    if (current.length === last.length && current.every((v, i) => v === last[i])) return;
    this.#lastEmittedSlots = current;
    this.#dispatch('x-multi-date-time-picker-slots-changed', { slots: current });
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

customElements.define('x-multi-date-time-picker', XMultiDateTimePicker);
