const X_SERVICE_PICKER_STYLES = `
.x-service-picker {
	font-family: Inter, "Segoe UI", system-ui, -apple-system, sans-serif;
	color: #1f2937;
	display: grid;
	gap: 0.35rem;
	width: 100%;
	box-sizing: border-box;
}

.service-summary,
.service-list,
.service-details {
	border: 1px solid #d1d5db;
	border-radius: 10px;
	width: 100%;
	box-sizing: border-box;
}

.service-summary {
	display: flex;
	align-items: center;
	justify-content: space-between;
	gap: 0.75rem;
	background: #ffffff;
	padding: 0.7rem 0.85rem;
}

.service-summary,
.service-button,
.service-option {
	touch-action: manipulation;
	-webkit-tap-highlight-color: transparent;
}

.selected-service-wrap {
	flex: 0 0 auto;
	width: fit-content;
	max-width: 100%;
	min-width: 0;
	text-align: left;
}

.selected-service-label {
	display: block;
	white-space: nowrap;
	font-weight: 600;
}

.service-actions {
	display: inline-flex;
	gap: 0.5rem;
	flex-wrap: wrap;
	flex: 0 0 auto;
	justify-content: flex-end;
	margin-left: auto;
	max-width: 100%;
}

.service-actions .service-button {
	flex: 0 0 auto;
}

.service-actions.is-wrapped {
	display: flex;
	width: 100%;
	margin-left: 0;
	justify-content: stretch;
}

.service-actions.is-wrapped .service-button {
	flex: 1 1 100%;
	width: 100%;
}

.service-summary.is-wrapped {
	flex-wrap: wrap;
	justify-content: center;
}

.service-summary.is-wrapped .selected-service-wrap {
	flex: 1 1 100%;
	width: 100%;
	text-align: center;
}

.service-summary.is-wrapped .service-actions {
	width: 100%;
	margin-left: 0;
	justify-content: center;
}

.service-button,
.service-option {
	border: 1px solid #d1d5db;
	border-radius: 8px;
	background: #ffffff;
	color: #1f2937;
	font: inherit;
	cursor: pointer;
}

.service-button {
	padding: 0.4rem 0.7rem;
	font-weight: 600;
}

.service-list,
.service-details {
	padding: 0.75rem;
	background: #fbfdff;
}

.service-options {
	display: grid;
	gap: 0.45rem;
}

.service-option {
	width: 100%;
	text-align: left;
	padding: 0.6rem 0.7rem;
	transition: background-color 120ms ease, border-color 120ms ease, box-shadow 120ms ease;
}

.service-option.selected {
	background: #0f62fe;
	color: #ffffff;
	border-color: #0f62fe;
}

.service-details strong {
	display: block;
	margin-bottom: 0.35rem;
}

.service-details p {
	margin: 0.35rem 0;
}
`;

const X_SERVICE_PICKER_TRANSLATIONS = {
	en: {
		btnActionExpand: "What is it",
		btnActionCollapse: "Close",
		metaDurationLabel: "Duration",
		metaDurationUnit: "min",
		metaPriceLabel: "Price",
	},
	de: {
		btnActionExpand: "Was ist das",
		btnActionCollapse: "Schließen",
		metaDurationLabel: "Dauer",
		metaDurationUnit: "Min",
		metaPriceLabel: "Preis",
	},
};

export class xServicePicker extends HTMLElement {
	constructor() {
		super();
		this.dataset.initialized = "false";
		this.translationKeys = {
			btnActionExpand: "btnActionExpand",
			btnActionCollapse: "btnActionCollapse",
			metaDurationLabel: "metaDurationLabel",
			metaDurationUnit: "metaDurationUnit",
			metaPriceLabel: "metaPriceLabel",
		};
		this.localizedTexts = {
			...X_SERVICE_PICKER_TRANSLATIONS.en,
		};
	}

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

	get btnActionExpandText() {
		return this.localizedTexts[this.translationKeys.btnActionExpand] || this.translationKeys.btnActionExpand;
	}

	get btnActionCollapseText() {
		return this.localizedTexts[this.translationKeys.btnActionCollapse] || this.translationKeys.btnActionCollapse;
	}

	static get observedAttributes() {
		return ["data", "locale"];
	}

	attributeChangedCallback(name) {
		if (this.dataset.initialized !== "true" || !this.serviceDetailsButton) {
			return;
		}

		if (name === "data") {
			this.applyServicesFromAttribute();
			return;
		}

		if (name === "locale") {
			this.applyTranslations();
			return;
		}

		const expanded = this.serviceDetailsButton.getAttribute("aria-expanded") === "true";
		this.serviceDetailsButton.textContent = expanded ? this.btnActionCollapseText : this.btnActionExpandText;
		this.updateWrapStates();
	}

	disconnectedCallback() {
		if (this.serviceActionsResizeObserver) {
			this.serviceActionsResizeObserver.disconnect();
			this.serviceActionsResizeObserver = null;
		}

		if (this.boundUpdateServiceActionsWrapState) {
			window.removeEventListener("resize", this.boundUpdateServiceActionsWrapState);
		}
	}

	connectedCallback() {
		if (this.dataset.initialized === "true") {
			return;
		}

		this.dataset.initialized = "true";
		this.refreshLocalizedTexts();
		this.render();
		this.setup();
		this.applyLocalizedTextToUI();
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
		return this.localizedTexts[translationKey] || X_SERVICE_PICKER_TRANSLATIONS.en[translationKey] || translationKey;
	}

	applyLocalizedTextToUI() {
		if (this.serviceDetailsButton) {
			const expanded = this.serviceDetailsButton.getAttribute("aria-expanded") === "true";
			this.serviceDetailsButton.textContent = expanded ? this.btnActionCollapseText : this.btnActionExpandText;
		}

		if (this.selectedServiceName && this.serviceDetailsPanel && !this.serviceDetailsPanel.hidden) {
			const selectedService = this.servicesByName.get(this.selectedServiceName);
			if (selectedService) {
				this.applyServiceDetails(selectedService);
			}
		}
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
			const payload = X_SERVICE_PICKER_TRANSLATIONS[candidate];
			if (payload && typeof payload === "object") {
				return this.normalizeI18nPayload(payload);
			}
		}

		return this.normalizeI18nPayload(X_SERVICE_PICKER_TRANSLATIONS.en);
	}

	refreshLocalizedTexts() {
		const localeTexts = this.resolveLocaleTranslations(this.locale);
		this.localizedTexts = {
			...X_SERVICE_PICKER_TRANSLATIONS.en,
			...localeTexts,
		};

		this.servicePriceFormatter = new Intl.NumberFormat(this.locale || "en", { style: "currency", currency: "EUR" });
	}

	applyTranslations() {
		this.refreshLocalizedTexts();
		this.applyLocalizedTextToUI();
	}

	applyServiceDetails(service) {
		this.serviceDetailsSummary.textContent = service.summary;
		this.serviceDetailsDescription.textContent = service.details;
		this.serviceDetailsMeta.textContent = `${this.t("metaDurationLabel")}: ${service.durationMinutes} ${this.t("metaDurationUnit")} · ${this.t("metaPriceLabel")}: ${this.servicePriceFormatter.format(service.priceEUR)}`;
	}

	render() {
		this.innerHTML = `
			<style>${X_SERVICE_PICKER_STYLES}</style>
			<div class="x-service-picker">
				<div class="service-summary">
					<div class="selected-service-wrap">
						<span class="selected-service-label"></span>
					</div>
					<div class="service-actions">
						<button class="service-button service-details-button" type="button" aria-expanded="false">${this.btnActionExpandText}</button>
					</div>
				</div>

				<div class="service-list" hidden>
					<div class="service-options"></div>
				</div>

				<div class="service-details" hidden>
					<strong class="service-details-summary"></strong>
					<p class="service-details-description"></p>
					<p class="service-details-meta"></p>
				</div>
			</div>
		`;
	}

	setup() {
		this.selectedServiceLabel = this.querySelector(".selected-service-label");
		this.selectedServiceWrap = this.querySelector(".selected-service-wrap");
		this.serviceSummary = this.querySelector(".service-summary");
		this.serviceActions = this.querySelector(".service-actions");
		this.serviceDetailsButton = this.querySelector(".service-details-button");
		this.serviceActionButtons = [...this.querySelectorAll(".service-actions .service-button")];
		this.serviceList = this.querySelector(".service-list");
		this.serviceOptions = this.querySelector(".service-options");
		this.serviceDetailsPanel = this.querySelector(".service-details");
		this.serviceDetailsSummary = this.querySelector(".service-details-summary");
		this.serviceDetailsDescription = this.querySelector(".service-details-description");
		this.serviceDetailsMeta = this.querySelector(".service-details-meta");

		this.servicesByName = new Map();
		this.selectedServiceName = "";

		this.addEventListener("click", (event) => {
			event.stopPropagation();
		});

		const onSummaryAreaClick = (event) => {
			if (event.currentTarget !== this.serviceSummary) {
				event.stopPropagation();
			}

			const clickedInsideActions =
				event.target && typeof event.target.closest === "function"
					? !!event.target.closest(".service-actions")
					: false;

			if (clickedInsideActions) {
				return;
			}

			event.preventDefault();
			event.stopPropagation();

			if (!this.serviceDetailsPanel.hidden) {
				this.showSelectedServiceDetails(false);
			}

			this.setServiceListExpanded(this.serviceList.hidden);
		};

		this.selectedServiceWrap.addEventListener("click", onSummaryAreaClick);
		this.selectedServiceLabel.addEventListener("click", onSummaryAreaClick);
		this.serviceSummary.addEventListener("click", onSummaryAreaClick);

		this.serviceDetailsButton.addEventListener("click", (event) => {
			event.preventDefault();
			event.stopPropagation();
			this.showSelectedServiceDetails(this.serviceDetailsPanel.hidden);
		});

		this.serviceDetailsPanel.addEventListener("click", (event) => {
			event.preventDefault();
			event.stopPropagation();

			if (!this.serviceDetailsPanel.hidden) {
				this.showSelectedServiceDetails(false);
			}
		});

		this.boundUpdateServiceActionsWrapState = () => this.updateWrapStates();
		window.addEventListener("resize", this.boundUpdateServiceActionsWrapState);

		if (typeof ResizeObserver !== "undefined") {
			this.serviceActionsResizeObserver = new ResizeObserver(() => {
				this.updateWrapStates();
			});
			this.serviceActionsResizeObserver.observe(this.serviceActions);
			this.serviceActionsResizeObserver.observe(this.serviceSummary);
			this.serviceActionsResizeObserver.observe(this.selectedServiceWrap);
		}

		requestAnimationFrame(() => this.updateWrapStates());

		this.applyServicesFromAttribute();
	}

	updateServiceActionsWrapState() {
		if (!this.serviceActions || this.serviceActionButtons.length < 2) {
			return;
		}

		this.serviceActions.classList.remove("is-wrapped");
		void this.serviceActions.offsetWidth;

		const firstTop = this.serviceActionButtons[0].offsetTop;
		const wrapped = this.serviceActionButtons.some((button) => button.offsetTop !== firstTop);
		this.serviceActions.classList.toggle("is-wrapped", wrapped);
	}

	updateServiceSummaryWrapState() {
		if (!this.serviceSummary || !this.selectedServiceWrap || !this.serviceActions) {
			return;
		}

		this.serviceSummary.classList.remove("is-wrapped");
		this.serviceActions.classList.remove("is-wrapped");
		void this.serviceSummary.offsetWidth;

		const summaryStyle = window.getComputedStyle(this.serviceSummary);
		const gap = Number.parseFloat(summaryStyle.columnGap || summaryStyle.gap || "0") || 0;
		const paddingLeft = Number.parseFloat(summaryStyle.paddingLeft || "0") || 0;
		const paddingRight = Number.parseFloat(summaryStyle.paddingRight || "0") || 0;
		const availableWidth = this.serviceSummary.clientWidth - paddingLeft - paddingRight;

		const labelRequiredWidth = this.selectedServiceLabel ? this.selectedServiceLabel.scrollWidth : this.selectedServiceWrap.scrollWidth;
		const actionsRequiredWidth = this.serviceActionButtons.reduce((total, button, index) => {
			const buttonWidth = button.offsetWidth;
			const gapWidth = index > 0 ? gap : 0;
			return total + buttonWidth + gapWidth;
		}, 0);

		const requiredWidth = labelRequiredWidth + actionsRequiredWidth + gap;

		const wrapped = requiredWidth > availableWidth + 0.5;
		this.serviceSummary.classList.toggle("is-wrapped", wrapped);
	}

	updateWrapStates() {
		this.updateServiceSummaryWrapState();
		this.updateServiceActionsWrapState();
	}

	applyServicesFromAttribute() {
		const servicesJson = this.getAttribute("data");
		if (!servicesJson || !servicesJson.trim()) {
			return false;
		}

		try {
			const parsedServices = JSON.parse(servicesJson);
			this.setServices(parsedServices);
			return true;
		} catch (error) {
			this.setLoadingFailed();
			return false;
		}
	}

	setServiceListExpanded(expanded) {
		if (expanded) {
			this.showSelectedServiceDetails(false);
		}

		this.serviceList.hidden = !expanded;
		this.updateWrapStates();
	}

	setSelectedService(serviceName) {
		const previousServiceName = this.selectedServiceName || "";
		const previousUid = this.getAttribute("uid") || "";
		const selectedService = serviceName ? this.servicesByName.get(serviceName) : null;
		const selectedUid = selectedService?.uid == null ? "" : String(selectedService.uid);

		this.selectedServiceName = serviceName;
		if (serviceName) {
			this.selectedServiceLabel.textContent = serviceName;
		} else {
			this.selectedServiceLabel.textContent = "";
		}

		if (selectedUid) {
			this.setAttribute("uid", selectedUid);
		} else {
			this.removeAttribute("uid");
		}

		[...this.serviceOptions.querySelectorAll("button.service-option")].forEach((button) => {
			button.classList.toggle("selected", button.dataset.value === serviceName);
		});

		this.dispatchEvent(
			new CustomEvent("service-change", {
				detail: { serviceName, uid: selectedUid || null },
				bubbles: true,
			})
		);

		const hasSelectionChanged = previousServiceName !== serviceName || previousUid !== selectedUid;
		if (hasSelectionChanged) {
			this.dispatchEvent(
				new CustomEvent("changed", {
					detail: { serviceName, uid: selectedUid || null },
					bubbles: true,
				})
			);
		}

		this.updateWrapStates();
	}

	showSelectedServiceDetails(expanded) {
		if (!expanded) {
			this.serviceDetailsPanel.hidden = true;
			this.serviceDetailsButton.textContent = this.btnActionExpandText;
			this.serviceDetailsButton.setAttribute("aria-expanded", "false");
			return;
		}

		if (!this.selectedServiceName) {
			this.showSelectedServiceDetails(false);
			return;
		}

		const service = this.servicesByName.get(this.selectedServiceName);
		if (!service) {
			this.showSelectedServiceDetails(false);
			return;
		}

		this.serviceList.hidden = true;

		this.applyServiceDetails(service);
		this.serviceDetailsPanel.hidden = false;
		this.serviceDetailsButton.textContent = this.btnActionCollapseText;
		this.serviceDetailsButton.setAttribute("aria-expanded", "true");
		this.updateWrapStates();
	}

	setNoServices() {
		this.setSelectedService("");
		this.serviceDetailsButton.disabled = true;
	}

	setLoadingFailed() {
		this.setSelectedService("");
		this.serviceDetailsButton.disabled = true;
	}

	setServices(services) {
		if (!Array.isArray(services) || services.length === 0) {
			this.setNoServices();
			return;
		}

		this.servicesByName = new Map(services.map((service) => [service.name, service]));
		this.serviceOptions.innerHTML = "";
		this.serviceDetailsButton.disabled = false;

		for (const service of services) {
			const optionButton = document.createElement("button");
			optionButton.type = "button";
			optionButton.className = "service-option";
			optionButton.dataset.value = service.name;
			optionButton.textContent = service.name;
			optionButton.addEventListener("click", () => {
				this.setSelectedService(service.name);
				this.setServiceListExpanded(false);
				this.showSelectedServiceDetails(false);
			});
			this.serviceOptions.appendChild(optionButton);
		}

		this.setSelectedService(services[0].name);
	}
}

if (!customElements.get("x-service-picker")) {
	customElements.define("x-service-picker", xServicePicker);
}
