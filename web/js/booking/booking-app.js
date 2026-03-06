// Import existing components so they self-register before we use them.
import '../x-service-picker.js';
import '../x-date-time-picker.js';
import '../x-toast.js';
import '../manage/booking-manager.js';

const BOOKING_APP_STYLES = `
  :host {
    display: block;
    font-family: Inter, "Segoe UI", system-ui, -apple-system, sans-serif;
    font-size: 1rem;
    line-height: 1.5;
    color: #1f2937;
    --bg: #f4f7fb;
    --surface: #ffffff;
    --text: #1f2937;
    --muted: #4b5563;
    --primary: #0f62fe;
    --primary-hover: #0b4fd1;
    --border: #d1d5db;
  }

  *, *::before, *::after {
    box-sizing: border-box;
  }

  .container {
    width: 100%;
    padding: 0 0.75rem;
    margin: 0 auto;
  }

  .content {
    padding: 1.5rem 0;
  }

  .card {
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 12px;
    padding: 1.25rem;
    min-width: 0;
  }

  .form-grid {
    display: grid;
    gap: 1rem;
  }

  .field-group {
    display: grid;
    gap: 0.35rem;
    min-width: 0;
  }

  .field-group-label {
    font-weight: 600;
    font-size: 1rem;
  }

  .field-group-hint {
    font-weight: 400;
    font-size: 0.9rem;
    color: var(--muted);
    margin: 0;
  }

  label {
    display: grid;
    gap: 0.25rem;
    font-weight: 500;
    font-size: 0.95rem;
  }

  input[type="text"],
  input[type="email"],
  input[type="tel"] {
    width: 100%;
    border: 1px solid var(--border);
    border-radius: 6px;
    padding: 0.5rem 0.6rem;
    font: inherit;
    font-size: 0.95rem;
    color: var(--text);
    background: var(--surface);
  }

  input:focus {
    outline: 2px solid var(--primary);
    outline-offset: 2px;
    border-color: transparent;
  }

  .button {
    display: block;
    width: 100%;
    border: none;
    background: var(--primary);
    color: #fff;
    padding: 0.7rem 1rem;
    border-radius: 8px;
    font: inherit;
    font-weight: 600;
    cursor: pointer;
    text-align: center;
  }

  .button:hover:not(:disabled) {
    background: var(--primary-hover);
  }

  .button:disabled {
    opacity: 0.45;
    cursor: not-allowed;
  }

  .button-secondary {
    display: inline;
    background: none;
    border: none;
    color: var(--primary);
    font: inherit;
    font-size: 0.9rem;
    cursor: pointer;
    padding: 0;
    text-decoration: underline;
  }

  .back-row {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    font-size: 0.9rem;
    color: var(--muted);
  }

  .error-banner {
    background: #fef2f2;
    border: 1px solid #fca5a5;
    border-radius: 8px;
    padding: 0.75rem;
    color: #991b1b;
    font-size: 0.9rem;
  }

  .success-icon {
    font-size: 2.5rem;
    text-align: center;
  }

  .success-title {
    font-size: 1.2rem;
    font-weight: 700;
    text-align: center;
    color: var(--text);
  }

  .success-body {
    font-size: 0.95rem;
    color: var(--muted);
    text-align: center;
  }

  .summary-box {
    background: #f0f7ff;
    border: 1px solid #bfdbfe;
    border-radius: 8px;
    padding: 0.75rem 1rem;
    font-size: 0.875rem;
    color: #1e40af;
  }

  [hidden] { display: none !important; }
`;

/**
 * BookingApp is the top-level Custom Element for the customer booking SPA.
 *
 * First milestone: one-page booking flow.
 * - View "selection": service-picker + date-time-picker â†’ "Weiter" button
 * - View "contact":  contact form (name / email / phone) â†’ "Jetzt buchen"
 * - View "success":  confirmation message after a successful API submission
 *
 * API sequence on submit:
 *   1. POST /api/v1/sessions               â†’ sessionId
 *   2. POST /api/v1/sessions/{id}/bookings â†’ bookingId
 *   3. POST /api/v1/sessions/{id}/submit   â†’ confirmation + triggers emails
 */
export class BookingApp extends HTMLElement {
	/** @type {'selection'|'contact'|'success'} */
	#view = 'selection';

	/** @type {string} */
	#serviceId = '';

	/** @type {string} */
	#serviceName = '';

	/** @type {string} */
	#selectedDate = '';

	/** @type {string} */
	#selectedTime = '';

	/** @type {number} */
	#activeYear = 0;

	/** @type {number} */
	#activeMonth = 0;

	constructor() {
		super();
		this.attachShadow({ mode: 'open' });
	}

	connectedCallback() {
		// Management-link mode: ?id=<bookingID>&token=<signedToken>
		const params = new URLSearchParams(window.location.search);
		const bookingId = params.get('id');
		const token     = params.get('token');
		if (bookingId && token) {
			this.#renderManagementMode(bookingId, token);
			return;
		}

		this.#render();
		this.#wirePickers();
		this.#wireContactForm();
		this.#initialize().catch((err) => {
			console.error('[x-booking-app] initialization failed', err);
		});
	}

	#renderManagementMode(bookingId, token) {
		const root = this.shadowRoot;
		root.innerHTML = '';
		const manager = document.createElement('x-booking-manager');
		manager.setAttribute('booking-id', bookingId);
		manager.setAttribute('token', token);
		root.appendChild(manager);
	}

	// -------------------------------------------------------------------------
	// Rendering
	// -------------------------------------------------------------------------

	#render() {
		this.shadowRoot.innerHTML = `
			<style>${BOOKING_APP_STYLES}</style>
			<main class="container content">
				<section class="card">

					<!-- ── View: selection ── -->
					<div id="view-selection" class="form-grid">

						<div class="field-group" id="service-group">
								<span class="field-group-label">Behandlung wählen</span>
								<p class="field-group-hint">
									Wähle die Behandlung, die du buchen möchtest, und ruf Informationen
								zu den Behandlungen ab, um die richtige Wahl zu treffen.
							</p>
							<x-service-picker id="service-picker"></x-service-picker>
						</div>

						<div class="field-group" id="datetime-group">
								<span class="field-group-label">Termin wählen</span>
								<p class="field-group-hint">
									Wähle einen passenden Termin und eine passende Uhrzeit aus.
							</p>
							<x-date-time-picker
								id="date-time-picker"
								data-availability-endpoint="api/v1/availability">
							</x-date-time-picker>
						</div>

						<div id="selection-error" class="error-banner" hidden></div>

						<button class="button" id="next-btn" type="button" disabled>
							Weiter
						</button>

					</div>

					<!-- ── View: contact ── -->
					<div id="view-contact" class="form-grid" hidden>

						<div class="back-row">
							<button class="button-secondary" id="back-btn" type="button">← Zurück</button>
						</div>

						<div class="field-group">
							<span class="field-group-label">Kontaktdaten eingeben</span>
							<p class="field-group-hint">
								Bitte gib deine Kontaktdaten an, damit wir deine Buchungsanfrage
								bearbeiten können.
							</p>
						</div>

						<div id="appointment-summary" class="summary-box"></div>

						<label>
							Vorname *
							<input type="text" id="first-name" autocomplete="given-name" required />
						</label>

						<label>
							Nachname *
							<input type="text" id="last-name" autocomplete="family-name" required />
						</label>

						<label>
							E-Mail-Adresse *
							<input type="email" id="email" autocomplete="email" required />
						</label>

						<label>
							Telefonnummer
							<input type="tel" id="phone" autocomplete="tel" />
						</label>

						<div id="contact-error" class="error-banner" hidden></div>

						<button class="button" id="submit-btn" type="button">
							Jetzt buchen
						</button>

					</div>

					<!-- ── View: success ── -->
					<div id="view-success" class="form-grid" hidden>
						<div class="success-icon">✅</div>
						<p class="success-title">Buchungsanfrage eingegangen!</p>
						<p class="success-body" id="success-msg"></p>
					</div>

				</section>
			</main>
			<x-toast id="email-toast" variant="info" duration="60000"></x-toast>
		`;
	}

	// -------------------------------------------------------------------------
	// View switching
	// -------------------------------------------------------------------------

	#showView(name) {
		this.#view = name;
		const sel = this.shadowRoot.getElementById('view-selection');
		const con = this.shadowRoot.getElementById('view-contact');
		const suc = this.shadowRoot.getElementById('view-success');
		sel.hidden = name !== 'selection';
		con.hidden = name !== 'contact';
		suc.hidden = name !== 'success';
	}

	// -------------------------------------------------------------------------
	// Event wiring – selection view
	// -------------------------------------------------------------------------

	#wirePickers() {
		const sp = this.shadowRoot.getElementById('service-picker');
		const dp = this.shadowRoot.getElementById('date-time-picker');
		const nextBtn = this.shadowRoot.getElementById('next-btn');

		// ─ Service picker ─
		sp.addEventListener('service-change', async (e) => {
			this.#serviceId   = e.detail?.uid         || '';
			this.#serviceName = e.detail?.serviceName || '';
			if (this.#activeYear && this.#activeMonth) {
				await this.#fetchAndApplyAvailability(this.#activeYear, this.#activeMonth);
			}
			this.#syncNextState();
		});

		// ─ Date/time picker ─
		dp.addEventListener('x-date-time-picker-initialized', async (e) => {
			const year = Number(e.detail?.year);
			const month = Number(e.detail?.month);
			if (year && month) {
				this.#activeYear = year;
				this.#activeMonth = month;
				await this.#fetchAndApplyAvailability(year, month);
			}
		});

		dp.addEventListener('x-date-time-picker-month-selected', async (e) => {
			const year = Number(e.detail?.year);
			const month = Number(e.detail?.month);
			if (year && month) {
				this.#activeYear = year;
				this.#activeMonth = month;
				await this.#fetchAndApplyAvailability(year, month);
			}
		});

		dp.addEventListener('x-date-time-picker-date-selected', (e) => {
			this.#selectedDate = e.detail?.date || '';
			const slots = Array.isArray(e.detail?.timeSlots) ? e.detail.timeSlots : [];
			if (slots.length > 0) {
				const current = (dp.getAttribute('selected-time') || '').trim();
				const time = (current && slots.includes(current)) ? current : slots[0];
				dp.setAttribute('selected-time-source', 'auto');
				dp.setAttribute('selected-time', time);
				this.#selectedTime = time;
			} else {
				this.#selectedTime = '';
			}
			this.#syncNextState();
		});

		const timeObserver = new MutationObserver(() => {
			this.#selectedTime = (dp.getAttribute('selected-time') || '').trim();
			this.#syncNextState();
		});
		timeObserver.observe(dp, { attributes: true, attributeFilter: ['selected-time'] });

		// ─ "Weiter" button → show contact view ─
		nextBtn.addEventListener('click', () => {
			this.#clearError('selection');
			this.#updateAppointmentSummary();
			this.#showView('contact');
		});
	}

	// -------------------------------------------------------------------------
	// Event wiring – contact view
	// -------------------------------------------------------------------------

	#wireContactForm() {
		const backBtn  = this.shadowRoot.getElementById('back-btn');
		const submitBtn = this.shadowRoot.getElementById('submit-btn');

		backBtn.addEventListener('click', () => {
			this.#clearError('contact');
			this.#showView('selection');
		});

		submitBtn.addEventListener('click', async () => {
			await this.#handleSubmit();
		});

		// Allow Enter key in form fields to trigger submit.
		['first-name', 'last-name', 'email', 'phone'].forEach((id) => {
			this.shadowRoot.getElementById(id).addEventListener('keydown', async (e) => {
				if (e.key === 'Enter') await this.#handleSubmit();
			});
		});
	}

	// -------------------------------------------------------------------------
	// Submit flow (API calls)
	// -------------------------------------------------------------------------

	async #handleSubmit() {
		const firstName = this.shadowRoot.getElementById('first-name').value.trim();
		const lastName  = this.shadowRoot.getElementById('last-name').value.trim();
		const emailVal  = this.shadowRoot.getElementById('email').value.trim();
		const phone     = this.shadowRoot.getElementById('phone').value.trim();

		if (!firstName || !lastName || !emailVal) {
			this.#showError('contact', 'Bitte fülle alle Pflichtfelder aus (Vorname, Nachname, E-Mail).');
			return;
		}

		const submitBtn = this.shadowRoot.getElementById('submit-btn');
		submitBtn.disabled = true;
		submitBtn.textContent = 'Bitte warten…';
		this.#clearError('contact');

		// Show a persistent info toast while the API calls + synchronous
		// e-mail dispatch are in flight on the server.
		const toast = this.shadowRoot.getElementById('email-toast');
		toast.setAttribute('variant', 'info');
		toast.setAttribute('duration', '60000');
		toast.message = 'Anfrage wird übermittelt …';

		try {
			// 1. Create session
			const sessionRes = await this.#apiFetch('api/v1/sessions', 'POST', {
				service_id: this.#serviceId,
			});
			const sessionId = sessionRes.id;

			// 2. Add the booking line
			const startISO = this.#buildStartISO();
			await this.#apiFetch(`api/v1/sessions/${sessionId}/bookings`, 'POST', {
				start: startISO,
			});

			// 3. Submit with contact details. The server sends the confirmation
			//    e-mail synchronously and reports the result in the response body.
			const submitResult = await this.#apiFetch(`api/v1/sessions/${sessionId}/submit`, 'POST', {
				first_name: firstName,
				last_name:  lastName,
				email:      emailVal,
				phone:      phone,
				timezone:   Intl.DateTimeFormat().resolvedOptions().timeZone,
			});

			// Show success view — the booking is confirmed regardless of e-mail status.
			const dateLabel = this.#formatDateLabel();
			this.shadowRoot.getElementById('success-msg').textContent =
				`Deine Anfrage für ${dateLabel} wurde erfolgreich übermittelt.`;
			this.#showView('success');

			// Update the toast with the server-reported e-mail delivery result.
			if (submitResult?.email_sent) {
				toast.setAttribute('variant', 'success');
				toast.setAttribute('duration', '5000');
				toast.message = `Bestätigungs-E-Mail wurde an ${emailVal} gesendet.`;
			} else if (submitResult?.email_error) {
				toast.setAttribute('variant', 'error');
				toast.setAttribute('duration', '10000');
				toast.message = submitResult.email_error;
			} else {
				// No e-mail configured on the server — hide the in-flight toast quietly.
				toast.message = '';
			}

		} catch (err) {
			console.error('[x-booking-app] submit failed', err);
			const msg = err.message || 'Ein unerwarteter Fehler ist aufgetreten. Bitte versuche es erneut.';

			// Update the toast to reflect the failure.
			toast.setAttribute('variant', 'error');
			toast.setAttribute('duration', '8000');
			toast.message = `Übermittlung fehlgeschlagen: ${msg}`;

			this.#showError('contact', msg);
			submitBtn.disabled = false;
			submitBtn.textContent = 'Jetzt buchen';
		}
	}

	// -------------------------------------------------------------------------
	// Helpers
	// -------------------------------------------------------------------------

	/**
	 * Returns the start datetime for the API call.
	 * selectedTime is now the original UTC RFC-3339 string from the server,
	 * so it can be submitted directly without any local-time reconstruction.
	 * @returns {string} RFC 3339 UTC start datetime, e.g. "2026-03-02T08:00:00Z"
	 */
	#buildStartISO() {
		return this.#selectedTime;
	}

	/** @returns {string} Human-readable appointment label */
	#formatDateLabel() {
		if (!this.#selectedTime) return 'den gewählten Termin';
		try {
			// selectedTime is the UTC RFC-3339 string from the server; new Date() parses it
			// correctly and toLocaleString() renders in the browser's local timezone.
			const d = new Date(this.#selectedTime);
			const locale = navigator.languages?.[0] ?? navigator.language ?? 'de-DE';
			const timeZone = Intl.DateTimeFormat().resolvedOptions().timeZone;
			return d.toLocaleString(locale, {
				weekday: 'long',
				day: '2-digit',
				month: 'long',
				year: 'numeric',
				hour: '2-digit',
				minute: '2-digit',
				timeZone,
			});
		} catch {
			return this.#selectedDate || 'den gewählten Termin';
		}
	}

	/** Populates the appointment summary box in the contact view. */
	#updateAppointmentSummary() {
		const box = this.shadowRoot.getElementById('appointment-summary');
		const serviceName = this.#serviceName || 'Gewählte Behandlung';
		box.textContent = `${serviceName} · ${this.#formatDateLabel()}`;
	}

	#syncNextState() {
		const btn = this.shadowRoot.getElementById('next-btn');
		if (!btn) return;
		btn.disabled = !(this.#serviceId && this.#selectedDate && this.#selectedTime);
	}

	#showError(view, message) {
		const id = view === 'selection' ? 'selection-error' : 'contact-error';
		const el = this.shadowRoot.getElementById(id);
		if (!el) return;
		el.textContent = message;
		el.hidden = false;
	}

	#clearError(view) {
		const id = view === 'selection' ? 'selection-error' : 'contact-error';
		const el = this.shadowRoot.getElementById(id);
		if (!el) return;
		el.textContent = '';
		el.hidden = true;
	}

	/**
	 * Thin fetch wrapper that parses JSON and throws on non-2xx status.
	 * @param {string} url
	 * @param {string} method
	 * @param {object} body
	 * @returns {Promise<any>}
	 */
	async #apiFetch(url, method, body) {
		const res = await fetch(url, {
			method,
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify(body),
		});
		let payload;
		try {
			payload = await res.json();
		} catch {
			payload = null;
		}
		if (!res.ok) {
			throw new Error(
				(typeof payload === 'string' ? payload : null)
				|| `Serverfehler (${res.status}). Bitte versuche es erneut.`,
			);
		}
		return payload;
	}

	// -------------------------------------------------------------------------
	// Async initialization
	// -------------------------------------------------------------------------

	async #initialize() {
		await customElements.whenDefined('x-service-picker');
		await customElements.whenDefined('x-date-time-picker');
		await customElements.whenDefined('x-toast');

		const sp = this.shadowRoot.getElementById('service-picker');

		try {
			const services = await this.#loadServices();
			sp.setAttribute('data', JSON.stringify(services));
			if (typeof sp.setServices === 'function') {
				sp.setServices(services);
			}
			this.#serviceId = services[0]?.uid || '';
			this.#serviceName = services[0]?.name || '';
			// The date-time picker may have already fired its initialized event
			// before this async load finished, so trigger availability now.
			if (this.#serviceId && this.#activeYear && this.#activeMonth) {
				await this.#fetchAndApplyAvailability(this.#activeYear, this.#activeMonth);
			}
		} catch (err) {
			console.error('[x-booking-app] service load failed', err);
			sp.setAttribute('data', '[]');
			if (typeof sp.setServices === 'function') {
				sp.setServices([]);
			}
		}
	}

	// -------------------------------------------------------------------------
	// API helpers
	// -------------------------------------------------------------------------

	/**
	 * Fetches services from the API and normalises them to the shape expected
	 * by `<x-service-picker>`.
	 * @returns {Promise<Array<{uid:string,name:string,summary:string,details:string,priceEUR:number,durationMinutes:number}>>}
	 */
	async #loadServices() {
		const res = await fetch('api/v1/services');
		if (!res.ok) {
			throw new Error(`service fetch failed: ${res.status}`);
		}
		const raw = await res.json();
		return (Array.isArray(raw) ? raw : []).map((s) => ({
			uid: s.id,
			name: s.name,
			summary: s.summary,
			details: s.description,
			priceEUR: s.price,
			durationMinutes: s.duration_minutes,
		}));
	}

	/**
	 * Fetches availability for a given month + service and applies the result
	 * to `<x-date-time-picker>` via its `available-dates` attribute.
	 * @param {number} year
	 * @param {number} month  (1-based)
	 */
	async #fetchAndApplyAvailability(year, month) {
		const dp = this.shadowRoot.getElementById('date-time-picker');
		if (!dp) return;
		// Don't fetch until a service is selected – the server requires service_id.
		if (!this.#serviceId) return;
		try {
			const availability = await this.#loadAvailability(year, month);
			dp.setAttribute('available-dates', JSON.stringify(availability));
		} catch (err) {
			console.error('[x-booking-app] availability fetch failed', err);
		}
	}

	/**
	 * @param {number} year
	 * @param {number} month  (1-based)
	 * @returns {Promise<{month:string, dates:string[], timeSlots:Record<string,string[]>}>}
	 */
	async #loadAvailability(year, month) {
		const dp = this.shadowRoot.getElementById('date-time-picker');
		const endpoint = (dp?.getAttribute('data-availability-endpoint') || 'api/v1/availability').trim();
		const monthKey = `${String(year).padStart(4, '0')}-${String(month).padStart(2, '0')}`;
		const sep = endpoint.includes('?') ? '&' : '?';
		const svcParam = this.#serviceId
			? `&service_id=${encodeURIComponent(this.#serviceId)}`
			: '';

		const res = await fetch(`${endpoint}${sep}period=${encodeURIComponent(monthKey)}${svcParam}`);
		if (!res.ok) {
			throw new Error(`availability fetch failed: ${res.status}`);
		}
		const payload = await res.json();
		const monthDates =
			payload?.months?.[monthKey] && typeof payload.months[monthKey] === 'object'
				? payload.months[monthKey]
				: {};

		return {
			month: monthKey,
			dates: Object.keys(monthDates).sort(),
			timeSlots: monthDates,
		};
	}
}

if (!customElements.get('x-booking-app')) {
	customElements.define('x-booking-app', BookingApp);
}
