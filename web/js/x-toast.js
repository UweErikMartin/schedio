const X_TOAST_STYLES = `
.x-toast {
	font-family: Inter, "Segoe UI", system-ui, -apple-system, sans-serif;
	position: fixed;
	top: 1rem;
	right: 1rem;
	z-index: 9999;
	display: flex;
	align-items: flex-start;
	gap: 0.75rem;
	min-width: 260px;
	max-width: 420px;
	background: #ffffff;
	border: 1px solid #d1d5db;
	border-radius: 10px;
	box-shadow: 0 4px 16px rgba(0, 0, 0, 0.12);
	padding: 0.85rem 1rem;
	box-sizing: border-box;
	opacity: 1;
	transform: translateY(0);
	transition: opacity 200ms ease, transform 200ms ease;
}

.x-toast.is-hidden {
	display: none;
}

.x-toast.is-entering {
	opacity: 0;
	transform: translateY(-0.5rem);
}

.x-toast.is-leaving {
	opacity: 0;
	transform: translateY(-0.5rem);
}

.toast-accent {
	flex: 0 0 4px;
	align-self: stretch;
	border-radius: 4px;
	min-height: 1rem;
	background: #0f62fe;
}

.x-toast[data-variant="success"] .toast-accent {
	background: #16a34a;
}

.x-toast[data-variant="error"] .toast-accent {
	background: #dc2626;
}

.x-toast[data-variant="info"] .toast-accent {
	background: #0f62fe;
}

.toast-body {
	flex: 1 1 auto;
	min-width: 0;
	color: #1f2937;
	font-size: 0.9375rem;
	line-height: 1.45;
	word-break: break-word;
	padding-top: 0.05rem;
}

.toast-close {
	flex: 0 0 auto;
	border: none;
	background: transparent;
	color: #6b7280;
	font-size: 1.1rem;
	line-height: 1;
	cursor: pointer;
	padding: 0.1rem 0.3rem;
	border-radius: 4px;
	font: inherit;
	touch-action: manipulation;
	-webkit-tap-highlight-color: transparent;
	margin: -0.1rem -0.15rem 0 0;
}

.toast-close:hover {
	color: #1f2937;
	background: #f3f4f6;
}

.toast-close:focus-visible {
	outline: 2px solid #0f62fe;
	outline-offset: 2px;
}

@media (max-width: 480px) {
	.x-toast {
		left: 1rem;
		right: 1rem;
		max-width: none;
		min-width: 0;
		top: 0.75rem;
	}
}
`;

const X_TOAST_TRANSLATIONS = {
	en: {
		closeBtnLabel: "Close",
	},
	de: {
		closeBtnLabel: "Schließen",
	},
};

export class xToast extends HTMLElement {
	constructor() {
		super();
		this.dataset.initialized = "false";
		this._message = "";
		this._dismissTimer = null;
		this.translationKeys = {
			closeBtnLabel: "closeBtnLabel",
		};
		this.localizedTexts = { ...X_TOAST_TRANSLATIONS.en };
	}

	// ─── Locale helpers (shared pattern) ────────────────────────────────────────

	get locale() {
		const explicitLocale = this.getAttribute("locale");
		if (explicitLocale && explicitLocale.trim()) {
			return explicitLocale.trim().toLowerCase();
		}

		const docLocale = document?.documentElement?.lang;
		if (docLocale && docLocale.trim()) {
			return docLocale.trim().toLowerCase();
		}

		const browserLocale = navigator.language;
		if (browserLocale && browserLocale.trim()) {
			return browserLocale.trim().toLowerCase();
		}

		return "en";
	}

	normalizeI18nPayload(payload) {
		if (!payload || typeof payload !== "object") {
			return {};
		}

		const normalized = {};
		for (const translationKey of Object.values(this.translationKeys)) {
			const value = payload[translationKey];
			if (typeof value === "string" && value.trim()) {
				normalized[translationKey] = value.trim();
			}
		}

		return normalized;
	}

	t(translationKeyName) {
		const translationKey = this.translationKeys[translationKeyName] || translationKeyName;
		return this.localizedTexts[translationKey] || X_TOAST_TRANSLATIONS.en[translationKey] || translationKey;
	}

	getLocaleCandidates(localeValue) {
		const normalized = String(localeValue || "").trim().toLowerCase();
		const candidates = [];

		if (normalized) {
			candidates.push(normalized);
			const base = normalized.split("-")[0];
			if (base && base !== normalized) {
				candidates.push(base);
			}
		}

		if (!candidates.includes("en")) {
			candidates.push("en");
		}

		return candidates;
	}

	resolveLocaleTranslations(localeValue) {
		for (const candidate of this.getLocaleCandidates(localeValue)) {
			const payload = X_TOAST_TRANSLATIONS[candidate];
			if (payload && typeof payload === "object") {
				return this.normalizeI18nPayload(payload);
			}
		}

		return this.normalizeI18nPayload(X_TOAST_TRANSLATIONS.en);
	}

	refreshLocalizedTexts() {
		const localeTexts = this.resolveLocaleTranslations(this.locale);
		this.localizedTexts = {
			...X_TOAST_TRANSLATIONS.en,
			...localeTexts,
		};
	}

	applyLocalizedTextToUI() {
		if (this.toastClose) {
			this.toastClose.setAttribute("aria-label", this.t("closeBtnLabel"));
		}
	}

	// ─── Observed attributes ─────────────────────────────────────────────────────

	static get observedAttributes() {
		return ["variant", "duration", "locale"];
	}

	attributeChangedCallback(name) {
		if (this.dataset.initialized !== "true") {
			return;
		}

		if (name === "locale") {
			this.refreshLocalizedTexts();
			this.applyLocalizedTextToUI();
			return;
		}

		if (name === "variant" && this.toastEl) {
			this.toastEl.dataset.variant = this.resolvedVariant;
		}

		// "duration" changes take effect the next time a timer is started; no
		// immediate action required.
	}

	// ─── Reflected getters ───────────────────────────────────────────────────────

	get resolvedVariant() {
		const val = this.getAttribute("variant");
		return val === "success" || val === "error" || val === "info" ? val : "info";
	}

	get resolvedDuration() {
		const val = Number(this.getAttribute("duration"));
		return Number.isFinite(val) && val > 0 ? val : 4000;
	}

	// ─── message property ────────────────────────────────────────────────────────

	get message() {
		return this._message;
	}

	set message(value) {
		this._message = String(value ?? "");
		if (this.dataset.initialized === "true") {
			this.applyMessage();
		}
	}

	// ─── Lifecycle ───────────────────────────────────────────────────────────────

	connectedCallback() {
		if (this.dataset.initialized === "true") {
			return;
		}

		this.dataset.initialized = "true";
		this.refreshLocalizedTexts();
		this.render();
		this.setup();

		// Apply message that may have been set before connection.
		if (this._message) {
			this.applyMessage();
		}
	}

	disconnectedCallback() {
		this.clearTimer();
	}

	// ─── Rendering ───────────────────────────────────────────────────────────────

	render() {
		this.innerHTML = `
			<style>${X_TOAST_STYLES}</style>
			<div class="x-toast is-hidden" role="alert" aria-live="assertive" aria-atomic="true" data-variant="${this.resolvedVariant}">
				<span class="toast-accent" aria-hidden="true"></span>
				<span class="toast-body"></span>
				<button type="button" class="toast-close" aria-label="${this.t("closeBtnLabel")}">&#x2715;</button>
			</div>
		`;
	}

	setup() {
		this.toastEl = this.querySelector(".x-toast");
		this.toastBody = this.querySelector(".toast-body");
		this.toastClose = this.querySelector(".toast-close");

		this.toastClose.addEventListener("click", () => {
			this.hide(true);
		});
	}

	// ─── Timer ───────────────────────────────────────────────────────────────────

	clearTimer() {
		if (this._dismissTimer !== null) {
			clearTimeout(this._dismissTimer);
			this._dismissTimer = null;
		}
	}

	startTimer() {
		this.clearTimer();
		this._dismissTimer = setTimeout(() => {
			this.hide(true);
		}, this.resolvedDuration);
	}

	// ─── State transitions ───────────────────────────────────────────────────────

	applyMessage() {
		if (!this._message) {
			// Spec: setting "" hides immediately without dispatching toast-dismissed.
			this.clearTimer();
			this.hideImmediate(false);
			return;
		}

		this.toastEl.dataset.variant = this.resolvedVariant;
		this.toastBody.textContent = this._message;

		if (this.toastEl.classList.contains("is-hidden") || this.toastEl.classList.contains("is-leaving")) {
			// Not currently visible — run the show animation.
			this.show();
		} else {
			// Already visible — spec says just reset the dismiss timer.
			this.startTimer();
		}
	}

	show() {
		this.toastEl.classList.remove("is-leaving");
		this.toastEl.classList.add("is-entering");
		this.toastEl.classList.remove("is-hidden");

		// Force a reflow so the entering state is painted before the transition.
		void this.toastEl.offsetWidth;

		this.toastEl.classList.remove("is-entering");
		this.startTimer();
	}

	hide(shouldDispatch) {
		this.clearTimer();

		// Guard: if already hidden, nothing to do.
		if (this.toastEl.classList.contains("is-hidden")) {
			return;
		}

		this.toastEl.classList.add("is-leaving");

		const onTransitionEnd = () => {
			this.toastEl.removeEventListener("transitionend", onTransitionEnd);
			clearTimeout(fallback);
			this.hideImmediate(shouldDispatch);
		};

		// Safety fallback in case the transitionend never fires (e.g. reduced motion,
		// transitions disabled in tests).
		const fallback = setTimeout(() => {
			this.toastEl.removeEventListener("transitionend", onTransitionEnd);
			this.hideImmediate(shouldDispatch);
		}, 250);

		this.toastEl.addEventListener("transitionend", onTransitionEnd);
	}

	hideImmediate(shouldDispatch) {
		this._message = "";
		this.toastEl.classList.remove("is-entering", "is-leaving");
		this.toastEl.classList.add("is-hidden");
		this.toastBody.textContent = "";

		if (shouldDispatch) {
			this.dispatchEvent(
				new CustomEvent("toast-dismissed", {
					detail: {},
					bubbles: true,
				})
			);
		}
	}
}

if (!customElements.get("x-toast")) {
	customElements.define("x-toast", xToast);
}
