const STYLES = `
  :host {
    display: block;
    font-family: Inter, "Segoe UI", system-ui, -apple-system, sans-serif;
    font-size: 1rem;
    color: #1f2937;
  }

  *, *::before, *::after { box-sizing: border-box; }

  .page {
    padding: 2rem;
    max-width: 600px;
  }

  .page-title {
    margin: 0 0 1.75rem;
    font-size: 1.5rem;
    font-weight: 700;
    color: #1f2937;
  }

  .card {
    background: #ffffff;
    border: 1px solid #d1d5db;
    border-radius: 12px;
    padding: 1.75rem 2rem;
  }

  .form-grid {
    display: grid;
    gap: 1.1rem;
  }

  .field {
    display: grid;
    gap: 0.35rem;
  }

  label {
    font-size: 0.9rem;
    font-weight: 600;
    color: #374151;
  }
  label .hint {
    font-weight: 400;
    color: #6b7280;
    font-size: 0.8125rem;
  }

  input[type="text"],
  input[type="number"] {
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

  .form-actions {
    margin-top: 1.5rem;
    display: flex;
    align-items: center;
    gap: 1rem;
  }

  .btn-save {
    padding: 0.65rem 1.5rem;
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
    gap: 0.5rem;
    min-height: 2.5rem;
  }
  .btn-save:hover:not(:disabled) { background: #0b4fd1; }
  .btn-save:disabled { background: #93c0fd; cursor: default; }

  .spinner {
    width: 1rem;
    height: 1rem;
    border: 2px solid rgba(255,255,255,0.4);
    border-top-color: #fff;
    border-radius: 50%;
    animation: spin 0.7s linear infinite;
  }
  .spinner.dark {
    border-color: rgba(15,98,254,0.2);
    border-top-color: #0f62fe;
  }
  @keyframes spin { to { transform: rotate(360deg); } }

  .loading-box {
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 3rem;
  }
  .loading-box .spinner {
    width: 1.75rem;
    height: 1.75rem;
  }

  .error-banner {
    padding: 0.75rem 1rem;
    background: #fef2f2;
    border: 1px solid #fecaca;
    border-radius: 8px;
    color: #dc2626;
    font-size: 0.9rem;
    margin-bottom: 1rem;
    display: none;
  }
  .error-banner.visible { display: block; }

  .toast {
    position: fixed;
    bottom: 1.5rem;
    right: 1.5rem;
    background: #16a34a;
    color: #fff;
    padding: 0.75rem 1.25rem;
    border-radius: 8px;
    font-size: 0.9375rem;
    font-weight: 500;
    box-shadow: 0 4px 16px rgba(0,0,0,0.15);
    opacity: 0;
    transform: translateY(0.5rem);
    transition: opacity 220ms, transform 220ms;
    pointer-events: none;
    z-index: 9999;
  }
  .toast.show { opacity: 1; transform: translateY(0); }
`;

const FIELDS = [
  { key: 'calendar_url',          label: 'Kalender-URL',                    type: 'text',   hint: 'Hostname oder URL des CalDAV-Servers (z. B. caldav.example.com)' },
  { key: 'default_calendar_name', label: 'Kalenderbezeichnung',             type: 'text',   hint: 'Anzeigename des Standardkalenders im CalDAV-Client (leer = "Default Calendar")' },
  { key: 'sender_name',           label: 'Absendername',                    type: 'text',   hint: 'Name in ausgehenden E-Mails' },
  { key: 'appointment_location',  label: 'Terminsort',                      type: 'text',   hint: 'Adresse oder Videolink' },
  { key: 'currency',              label: 'Währung',                         type: 'text',   hint: 'z. B. EUR' },
  { key: 'no_show_deadline_hours',label: 'No-Show-Frist (Stunden)',         type: 'number', hint: 'Stunden nach Terminbeginn' },
  { key: 'retention_period_days', label: 'Datenspeicherung (Tage)',         type: 'number', hint: 'Aufbewahrungsdauer für Buchungsdaten' },
  { key: 'reminder_lead_time_days',label: 'Erinnerung im Voraus (Tage)',   type: 'number', hint: 'Tage vor dem Termin' },
];

const API_URL = '/admin/api/v1/settings';

/**
 * `<x-settings-form>` — global settings editor for the admin SPA.
 *
 * Fetches current settings on mount and persists changes via PUT
 * `/admin/api/v1/settings`.
 */
class XSettingsForm extends HTMLElement {
  #root;
  #settings = null;

  connectedCallback() {
    this.#root = this.attachShadow({ mode: 'open' });
    this.#root.innerHTML = `<style>${STYLES}</style><div class="page"><h1 class="page-title">Einstellungen</h1><div id="body"></div></div>`;
    this.#load();
  }

  async #load() {
    this.#bodyEl().innerHTML = `<div class="loading-box"><span class="spinner dark"></span></div>`;
    try {
      const res = await fetch(API_URL, { credentials: 'same-origin' });
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      this.#settings = await res.json();
      this.#renderForm();
    } catch (err) {
      this.#bodyEl().innerHTML = `
        <div class="error-banner visible">Fehler beim Laden der Einstellungen: ${err.message}</div>
      `;
    }
  }

  #bodyEl() { return this.#root.getElementById('body'); }

  #renderForm() {
    const fieldsHTML = FIELDS.map(f => {
      const val = this.#settings?.[f.key] ?? '';
      return `
        <div class="field">
          <label for="${f.key}">${f.label} <span class="hint">${f.hint}</span></label>
          <input
            id="${f.key}"
            name="${f.key}"
            type="${f.type}"
            value="${String(val).replaceAll('"', '&quot;')}"
            ${f.type === 'number' ? 'min="0"' : ''}
          />
        </div>
      `;
    }).join('');

    this.#bodyEl().innerHTML = `
      <div class="error-banner" id="error-banner"></div>
      <div class="card">
        <form id="settings-form" novalidate>
          <div class="form-grid">${fieldsHTML}</div>
          <div class="form-actions">
            <button type="submit" class="btn-save" id="save-btn">
              <span id="save-label">Speichern</span>
            </button>
          </div>
        </form>
      </div>
      <div class="toast" id="toast">Einstellungen gespeichert</div>
    `;

    this.#root.getElementById('settings-form').addEventListener('submit', e => {
      e.preventDefault();
      this.#save();
    });
  }

  async #save() {
    const form    = this.#root.getElementById('settings-form');
    const saveBtn = this.#root.getElementById('save-btn');
    const saveLabel = this.#root.getElementById('save-label');
    const errBanner = this.#root.getElementById('error-banner');

    if (!form.reportValidity()) return;

    // Build payload
    const payload = {};
    FIELDS.forEach(f => {
      const input = this.#root.getElementById(f.key);
      payload[f.key] = f.type === 'number' ? Number(input.value) : input.value;
    });

    // Set busy state
    saveBtn.disabled = true;
    saveLabel.textContent = '';
    let spinner = saveBtn.querySelector('.spinner');
    if (!spinner) {
      spinner = document.createElement('span');
      spinner.className = 'spinner';
      saveBtn.insertBefore(spinner, saveLabel);
    }
    errBanner.className = 'error-banner';

    try {
      const res = await fetch(API_URL, {
        method:      'PUT',
        credentials: 'same-origin',
        headers:     { 'Content-Type': 'application/json' },
        body:        JSON.stringify(payload),
      });

      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      this.#settings = payload;
      this.#showToast();
    } catch (err) {
      errBanner.textContent = `Fehler beim Speichern: ${err.message}`;
      errBanner.className = 'error-banner visible';
    } finally {
      saveBtn.disabled = false;
      saveLabel.textContent = 'Speichern';
      if (spinner) spinner.remove();
    }
  }

  #showToast() {
    const toast = this.#root.getElementById('toast');
    if (!toast) return;
    toast.classList.add('show');
    setTimeout(() => toast.classList.remove('show'), 3000);
  }
}

customElements.define('x-settings-form', XSettingsForm);
