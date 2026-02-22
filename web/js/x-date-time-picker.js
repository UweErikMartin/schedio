const X_DATE_TIME_PICKER_STYLES = `
.x-date-time-picker-root {
	--surface: #ffffff;
	--text: #1f2937;
	--muted: #4b5563;
	--primary: #0f62fe;
	--border: #d1d5db;
	--calendar-offset: 0.35rem;
	font-family: Inter, "Segoe UI", system-ui, -apple-system, sans-serif;
	font-size: 1rem;
	color: var(--text);
	display: grid;
	gap: 0.35rem;
	position: relative;
}

.x-date-time-picker-root,
.x-date-time-picker-root * {
	box-sizing: border-box;
}

.x-date-time-picker-root .date-summary {
	display: flex;
	flex-wrap: nowrap;
	align-items: center;
	justify-content: space-between;
	gap: 0.75rem;
	border: 1px solid var(--border);
	border-radius: 10px;
	background: var(--surface);
	padding: 0.7rem 0.85rem;
	width: 100%;
}

.x-date-time-picker-root .date-summary,
.x-date-time-picker-root .select-time-button,
.x-date-time-picker-root .calendar-nav,
.x-date-time-picker-root .calendar-day,
.x-date-time-picker-root .time-option {
	touch-action: manipulation;
	-webkit-tap-highlight-color: transparent;
}

.x-date-time-picker-root .selected-date-wrap {
	flex: 0 0 auto;
	width: fit-content;
	max-width: 100%;
	min-width: 0;
	text-align: left;
}

.x-date-time-picker-root .selected-date-label {
	display: block;
	white-space: nowrap;
}

.x-date-time-picker-root .date-summary.is-wrapped .selected-date-label {
	white-space: normal;
	overflow-wrap: break-word;
	word-break: normal;
}

.x-date-time-picker-root .date-actions {
	display: inline-flex;
	gap: 0.5rem;
	flex-wrap: wrap;
	flex: 0 0 auto;
	justify-content: flex-end;
	margin-left: auto;
	max-width: 100%;
}

.x-date-time-picker-root .date-actions.is-wrapped {
	display: flex;
	width: 100%;
	margin-left: 0;
}

	.x-date-time-picker-root .date-actions.is-wrapped .select-time-button {
	flex: 1 1 100%;
}

.x-date-time-picker-root .date-summary.is-wrapped {
	flex-wrap: wrap;
	justify-content: center;
}

.x-date-time-picker-root .date-summary.is-wrapped .selected-date-wrap {
	flex: 1 1 100%;
	width: 100%;
	text-align: center;
}

.x-date-time-picker-root .date-summary.is-wrapped .date-actions {
	width: 100%;
	margin-left: 0;
	justify-content: center;
}

.x-date-time-picker-root .calendar {
	border: 1px solid var(--border);
	border-radius: 10px;
	padding: 0.75rem;
	background: #fbfdff;
	position: absolute;
	top: calc(100% + var(--calendar-offset));
	left: 0;
	right: 0;
	z-index: 20;
}

.x-date-time-picker-root .time-list {
	border: 1px solid var(--border);
	border-radius: 10px;
	padding: 0.75rem;
	background: #fbfdff;
	position: absolute;
	top: calc(100% + var(--calendar-offset));
	left: 0;
	right: 0;
	z-index: 20;
}

.x-date-time-picker-root .time-options {
	display: grid;
	gap: 0.45rem;
}

.x-date-time-picker-root .time-option {
	width: 100%;
	text-align: left;
	padding: 0.6rem 0.7rem;
	border: 1px solid var(--border);
	border-radius: 8px;
	background: var(--surface);
	color: var(--text);
	font: inherit;
	cursor: pointer;
	transition: background-color 120ms ease, border-color 120ms ease, box-shadow 120ms ease;
}

.x-date-time-picker-root .time-option.selected {
	background: var(--primary);
	color: #ffffff;
	border-color: var(--primary);
}

.x-date-time-picker-root .time-option:disabled {
	background: #f3f4f6;
	color: #9ca3af;
	border-color: #e5e7eb;
	cursor: not-allowed;
}

.x-date-time-picker-root .select-time-button {
	border: 1px solid var(--border);
	border-radius: 8px;
	background: var(--surface);
	color: var(--text);
	padding: 0.4rem 0.7rem;
	cursor: pointer;
	font: inherit;
	font-weight: 600;
	min-width: 0;
	max-width: 100%;
	white-space: normal;
	overflow-wrap: anywhere;
}

.x-date-time-picker-root .select-time-button:hover {
	border-color: var(--primary);
}

.x-date-time-picker-root .calendar-header {
	display: flex;
	align-items: center;
	justify-content: space-between;
	margin-bottom: 0.6rem;
}

.x-date-time-picker-root .calendar-nav {
	border: 1px solid var(--border);
	border-radius: 8px;
	background: var(--surface);
	color: var(--text);
	padding: 0.35rem 0.6rem;
	cursor: pointer;
}

.x-date-time-picker-root .calendar-weekdays,
.x-date-time-picker-root .calendar-grid {
	display: grid;
	grid-template-columns: repeat(7, minmax(0, 1fr));
	gap: 0.35rem;
}

.x-date-time-picker-root .calendar-weekdays {
	margin-bottom: 0.4rem;
	color: var(--muted);
	font-size: 0.82rem;
}

.x-date-time-picker-root .calendar-weekdays span {
	text-align: center;
	font-weight: 600;
}

.x-date-time-picker-root .calendar-spacer {
	height: 2.1rem;
}

.x-date-time-picker-root .calendar-day {
	border: 1px solid var(--border);
	border-radius: 8px;
	background: var(--surface);
	color: var(--text);
	min-height: 2.1rem;
	cursor: pointer;
}

.x-date-time-picker-root .calendar-day:hover:not(:disabled) {
	border-color: var(--primary);
}

.x-date-time-picker-root .calendar-day.selected {
	border-color: var(--primary);
	font-weight: 700;
	box-shadow: inset 0 0 0 1px #fff;
}

.x-date-time-picker-root .calendar-day.disabled,
.x-date-time-picker-root .calendar-day:disabled {
	background: #f3f4f6;
	color: #9ca3af;
	border-color: #e5e7eb;
	cursor: not-allowed;
}

@media (max-width: 520px) {
	.x-date-time-picker-root .date-summary {
		flex-wrap: wrap;
		justify-content: center;
	}

	.x-date-time-picker-root .selected-date-wrap {
		flex: 1 1 100%;
		width: 100%;
		text-align: center;
	}

	.x-date-time-picker-root .date-actions {
		width: 100%;
		margin-left: 0;
		justify-content: center;
	}

	.x-date-time-picker-root .date-actions .select-time-button {
		flex: 1 1 100%;
	}
}
`;

const X_DATE_TIME_PICKER_TRANSLATIONS = {
	en: {
		lblLoadingInitial: "Next available date is loading...",
		lblLoadingAvailability: "Available dates are loading...",
		lblNoDateSelected: "No available date selected",
		lblSelectedDatePrefix: "Selected date",
		lblEarliestDatePrefix: "Earliest appointment",
		lblTimeSuffix: "o'clock",
		btnSelectTime: "change time",
		lblMonth: "Month",
		lblNextMonth: "Next month",
		lblPreviousMonth: "Previous month",
		helpLoadingAvailability: "Available dates are loading...",
		helpLoadingMonth: "Availability is loading...",
		helpChooseEnabledDate: "Choose one of the enabled dates. Disabled dates are unavailable.",
		helpNoDatesInMonth: "No dates are available for this month.",
		helpDateUpdated: "Date updated. Click 'search another date' to choose a different date."
	},
	de: {
		lblLoadingInitial: "Nächster verfügbarer Termin wird geladen...",
		lblLoadingAvailability: "Verfügbare Termine werden geladen...",
		lblNoDateSelected: "Kein verfügbarer Termin ausgewählt",
		lblSelectedDatePrefix: "Ausgewählter Termin",
		lblEarliestDatePrefix: "Frühester Termin",
		lblTimeSuffix: "Uhr",
		btnSelectTime: "Zeit ändern",
		lblMonth: "Monat",
		lblNextMonth: "Nächster Monat",
		lblPreviousMonth: "Vorheriger Monat",
		helpLoadingAvailability: "Verfügbare Termine werden geladen...",
		helpLoadingMonth: "Verfügbarkeit wird geladen...",
		helpChooseEnabledDate: "Wähle eines der aktivierten Termine. Deaktivierte Termine sind nicht verfügbar.",
		helpNoDatesInMonth: "Für diesen Monat sind keine Termine verfügbar.",
		helpDateUpdated: "Termin aktualisiert. Klicke auf 'anderen Termin suchen', um einen anderen Termin zu wählen."
	},
};

export class xDateTimePicker extends HTMLElement {
	constructor() {
		super();
		this.translationKeys = {
			lblLoadingInitial: "lblLoadingInitial",
			lblLoadingAvailability: "lblLoadingAvailability",
			lblNoDateSelected: "lblNoDateSelected",
			lblSelectedDatePrefix: "lblSelectedDatePrefix",
			lblEarliestDatePrefix: "lblEarliestDatePrefix",
			lblTimeSuffix: "lblTimeSuffix",
			btnSelectTime: "btnSelectTime",
			lblMonth: "lblMonth",
			lblNextMonth: "lblNextMonth",
			lblPreviousMonth: "lblPreviousMonth",
			helpLoadingAvailability: "helpLoadingAvailability",
			helpLoadingMonth: "helpLoadingMonth",
			helpChooseEnabledDate: "helpChooseEnabledDate",
			helpNoDatesInMonth: "helpNoDatesInMonth",
			helpDateUpdated: "helpDateUpdated",
		};
		this.localizedTexts = {
			...X_DATE_TIME_PICKER_TRANSLATIONS.en,
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

	getLocalizedText(labelName) {
		const normalizedLabel = String(labelName || "").trim();
		if (!normalizedLabel) {
			return "";
		}

		const localizedValue = this.localizedTexts?.[normalizedLabel];
		if (typeof localizedValue === "string" && localizedValue.trim()) {
			return localizedValue;
		}

		return normalizedLabel;
	}

	get weekdayLabels() {
		const formatter = new Intl.DateTimeFormat(this.locale, { weekday: "short" });
		const mondayStart = Date.UTC(2024, 0, 1);
		return Array.from({ length: 7 }, (_, index) => {
			const date = new Date(mondayStart + index * 24 * 60 * 60 * 1000);
			return formatter.format(date);
		});
	}

	static get observedAttributes() {
		return ["available-dates", "availabledates", "availableDates", "selected-time", "selectedtime", "selectedTime", "locale"];
	}

	connectedCallback() {
		if (this.dataset.initialized === "true") {
			return;
		}

		this.dataset.initialized = "true";
		this.render();
		this.setup();
		this.applyLocaleTranslations();
	}

	attributeChangedCallback(name, oldValue, newValue) {
		const normalizedName = String(name || "").toLowerCase();
		const isAvailabilityAttribute = normalizedName === "available-dates" || normalizedName === "availabledates";
		const isSelectedTimeAttribute = normalizedName === "selected-time" || normalizedName === "selectedtime";
		const isLocaleAttribute = normalizedName === "locale";
		if (oldValue === newValue) {
			return;
		}

		if (this.dataset.initialized !== "true") {
			return;
		}

		if (isLocaleAttribute) {
			this.applyLocaleTranslations();
			return;
		}

		if (isSelectedTimeAttribute) {
			this.selectedTime = typeof newValue === "string" ? newValue.trim() : "";
			const selectedTimeSource = (this.getAttribute("selected-time-source") || "").trim().toLowerCase();
			if (typeof oldValue === "string" && oldValue !== newValue && this.selectedTime && selectedTimeSource === "user") {
				this.hasUserChangedDate = true;
			}
			if (this.dateLabel) {
				this.updateSelectedDateLabel();
			}
			return;
		}

		if (!isAvailabilityAttribute) {
			return;
		}

		try {
			const payload = JSON.parse(newValue || "{}");
			const normalizedAvailability = this.normalizeAvailabilityPayload(payload);
			if (!normalizedAvailability) {
				return;
			}

			this.availableDatesByMonth.set(normalizedAvailability.month, normalizedAvailability);

			if (this.toMonthKey(this.activeYear || 0, this.activeMonth || 0) === normalizedAvailability.month) {
				this.applyAvailabilityForActiveMonth();
			}
		} catch (error) {
		}
	}

	normalizeAvailabilityPayload(payload) {
		if (!payload || typeof payload !== "object") {
			return null;
		}

		const month = typeof payload.month === "string" ? payload.month : "";
		if (!month) {
			return null;
		}

		const rawTimeSlots = payload.timeSlots && typeof payload.timeSlots === "object" ? payload.timeSlots : {};
		const normalizedTimeSlots = {};

		for (const [sourceDateKey, slotValues] of Object.entries(rawTimeSlots)) {
			if (!Array.isArray(slotValues)) {
				continue;
			}

			for (const rawSlot of slotValues) {
				if (typeof rawSlot !== "string") {
					continue;
				}

				const trimmedSlot = rawSlot.trim();
				if (!trimmedSlot) {
					continue;
				}

				const parsedDate = new Date(trimmedSlot);
				if (Number.isFinite(parsedDate.getTime())) {
					const localDateKey = this.toDateKey(
						parsedDate.getFullYear(),
						parsedDate.getMonth() + 1,
						parsedDate.getDate(),
					);
					const localTimeLabel = `${String(parsedDate.getHours()).padStart(2, "0")}:${String(parsedDate.getMinutes()).padStart(2, "0")}`;

					if (!normalizedTimeSlots[localDateKey]) {
						normalizedTimeSlots[localDateKey] = [];
					}
					normalizedTimeSlots[localDateKey].push(localTimeLabel);
					continue;
				}

				if (!normalizedTimeSlots[sourceDateKey]) {
					normalizedTimeSlots[sourceDateKey] = [];
				}
				normalizedTimeSlots[sourceDateKey].push(trimmedSlot);
			}
		}

		const normalizedDates = Object.keys(normalizedTimeSlots).sort();
		for (const dateKey of normalizedDates) {
			normalizedTimeSlots[dateKey] = Array.from(new Set(normalizedTimeSlots[dateKey])).sort();
		}

		return {
			month,
			dates: normalizedDates,
			timeSlots: normalizedTimeSlots
		};
	}

	render() {
		this.innerHTML = `
			<style>${X_DATE_TIME_PICKER_STYLES}</style>
			<div class="x-date-time-picker-root">
				<div class="date-summary" id="date-summary">
					<div class="selected-date-wrap" id="selected-date-wrap">
						<span class="selected-date-label" id="selected-date-label">${this.getLocalizedText(this.translationKeys.lblLoadingInitial)}</span>
					</div>
					<div class="date-actions" id="date-actions">
						<button class="select-time-button" type="button" id="select-time-button">${this.getLocalizedText(this.translationKeys.btnSelectTime)}</button>
					</div>
				</div>
				<div class="calendar" id="booking-calendar" hidden>
					<div class="calendar-header">
						<button class="calendar-nav" type="button" id="calendar-prev" aria-label="${this.getLocalizedText(this.translationKeys.lblPreviousMonth)}">◀</button>
						<strong id="calendar-month-label">${this.getLocalizedText(this.translationKeys.lblMonth)}</strong>
						<button class="calendar-nav" type="button" id="calendar-next" aria-label="${this.getLocalizedText(this.translationKeys.lblNextMonth)}">▶</button>
					</div>
					<div class="calendar-weekdays" id="calendar-weekdays"></div>
					<div class="calendar-grid" id="calendar-grid"></div>
				</div>
				<div class="time-list" id="time-list" hidden>
					<div class="time-options" id="time-options"></div>
				</div>
			</div>
		`;
	}

	normalizeI18nPayload(payload) {
		if (!payload || typeof payload !== "object") {
			return {};
		}

		const normalized = {};
		for (const key of Object.values(this.translationKeys)) {
			const kebabKey = key.replace(/[A-Z]/g, (letter) => `-${letter.toLowerCase()}`);
			const value = payload[key] ?? payload[kebabKey];
			if (typeof value === "string" && value.trim()) {
				normalized[key] = value.trim();
			}
		}

		return normalized;
	}

	applyLocalizedTextToUI() {
		if (this.dateLabel && !this.selectedDate) {
			this.dateLabel.textContent = this.hasEmittedInitializedEvent
				? this.getLocalizedText(this.translationKeys.lblNoDateSelected)
				: this.getLocalizedText(this.translationKeys.lblLoadingInitial);
		}

		if (this.selectTimeButton) {
			this.selectTimeButton.textContent = this.getLocalizedText(this.translationKeys.btnSelectTime);
		}

		if (this.prevButton) {
			this.prevButton.setAttribute("aria-label", this.getLocalizedText(this.translationKeys.lblPreviousMonth));
		}

		if (this.nextButton) {
			this.nextButton.setAttribute("aria-label", this.getLocalizedText(this.translationKeys.lblNextMonth));
		}

		if (this.calendarWeekdays) {
			this.calendarWeekdays.innerHTML = this.weekdayLabels.map((label) => `<span>${label}</span>`).join("");
		}

		this.monthLabelFormatter = new Intl.DateTimeFormat(this.locale, { month: "long", year: "numeric" });
		this.selectedDateFormatter = new Intl.DateTimeFormat(this.locale, { weekday: "short", day: "2-digit", month: "short", year: "numeric" });

		if (this.activeYear && this.activeMonth && this.monthLabel) {
			const monthStart = new Date(this.activeYear, this.activeMonth - 1, 1);
			this.monthLabel.textContent = this.monthLabelFormatter.format(monthStart);
		}

		this.updateSelectedDateLabel();
		this.updateWrapStates();
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
			const payload = X_DATE_TIME_PICKER_TRANSLATIONS[candidate];
			if (payload && typeof payload === "object") {
				return this.normalizeI18nPayload(payload);
			}
		}

		return this.normalizeI18nPayload(X_DATE_TIME_PICKER_TRANSLATIONS.en);
	}

	applyLocaleTranslations() {
		const localeTexts = this.resolveLocaleTranslations(this.locale);
		this.localizedTexts = {
			...X_DATE_TIME_PICKER_TRANSLATIONS.en,
			...localeTexts,
		};
		this.applyLocalizedTextToUI();
	}

	setup() {
		this.dateSummary = this.querySelector("#date-summary");
		this.selectedDateWrap = this.querySelector("#selected-date-wrap");
		this.dateActions = this.querySelector("#date-actions");
		this.dateLabel = this.querySelector("#selected-date-label");
		this.selectTimeButton = this.querySelector("#select-time-button");
		this.calendar = this.querySelector("#booking-calendar");
		this.timeList = this.querySelector("#time-list");
		this.timeOptions = this.querySelector("#time-options");
		this.monthLabel = this.querySelector("#calendar-month-label");
		this.calendarWeekdays = this.querySelector("#calendar-weekdays");
		this.grid = this.querySelector("#calendar-grid");
		this.prevButton = this.querySelector("#calendar-prev");
		this.nextButton = this.querySelector("#calendar-next");

		if (!this.dateSummary || !this.selectedDateWrap || !this.dateActions || !this.dateLabel || !this.selectTimeButton || !this.calendar || !this.timeList || !this.timeOptions || !this.monthLabel || !this.calendarWeekdays || !this.grid || !this.prevButton || !this.nextButton) {
			return;
		}

		this.addEventListener("click", (event) => {
			event.stopPropagation();
		});

		const now = new Date();
		this.activeYear = now.getFullYear();
		this.activeMonth = now.getMonth() + 1;
		this.hasUserChangedDate = false;
		this.selectedDate = "";

		this.monthLabelFormatter = new Intl.DateTimeFormat(this.locale, { month: "long", year: "numeric" });
		this.selectedDateFormatter = new Intl.DateTimeFormat(this.locale, { weekday: "short", day: "2-digit", month: "short", year: "numeric" });
		this.hasEmittedInitializedEvent = false;
		this.availableDatesByMonth = new Map();
		this.selectedTime = typeof this.getAttribute("selected-time") === "string" ? this.getAttribute("selected-time").trim() : "";
		this.isDropdownSessionActive = false;
		this.dropdownScrollRestoreY = null;

		this.prevButton.addEventListener("click", () => {
			if (!this.canNavigateToPreviousMonth()) {
				this.updateMonthNavigationState();
				return;
			}

			const previousYear = this.activeYear;
			const previousMonth = this.activeMonth;
			this.activeMonth -= 1;
			if (this.activeMonth < 1) {
				this.activeMonth = 12;
				this.activeYear -= 1;
			}
			this.renderMonth();
			this.emitMonthSelectedEvent(previousYear, previousMonth);
		});

		this.nextButton.addEventListener("click", () => {
			const previousYear = this.activeYear;
			const previousMonth = this.activeMonth;
			this.activeMonth += 1;
			if (this.activeMonth > 12) {
				this.activeMonth = 1;
				this.activeYear += 1;
			}
			this.renderMonth();
			this.emitMonthSelectedEvent(previousYear, previousMonth);
		});

		this.selectTimeButton.addEventListener("click", (event) => {
			event.preventDefault();
			event.stopPropagation();
			this.setTimeListExpanded(this.timeList.hidden);
		});

		this.dateSummary.addEventListener("click", (event) => {
			const clickedInsideActions =
				event.target && typeof event.target.closest === "function"
					? !!event.target.closest(".date-actions")
					: false;

			if (clickedInsideActions) {
				return;
			}

			event.preventDefault();
			event.stopPropagation();
			this.setCalendarExpanded(this.calendar.hidden);
		});

		this.boundUpdateWrapStates = () => this.updateWrapStates();
		window.addEventListener("resize", this.boundUpdateWrapStates);

		if (typeof ResizeObserver !== "undefined") {
			this.datePickerResizeObserver = new ResizeObserver(() => {
				this.updateWrapStates();
			});
			this.datePickerResizeObserver.observe(this.dateSummary);
			this.datePickerResizeObserver.observe(this.selectedDateWrap);
			this.datePickerResizeObserver.observe(this.dateActions);
		}

		requestAnimationFrame(() => this.updateWrapStates());
		this.updateMonthNavigationState();

		this.calendarWeekdays.innerHTML = this.weekdayLabels.map((label) => `<span>${label}</span>`).join("");
		this.setHelpText(this.getLocalizedText(this.translationKeys.helpLoadingAvailability));
		this.renderTimeOptions();
		setTimeout(() => {
			this.initialize();
		}, 0);
	}

	setHelpText(text) {
		const hostLabel = this.closest("label");
		const help = hostLabel ? hostLabel.querySelector("#date-help") : document.getElementById("date-help");
		if (help) {
			help.textContent = text;
		}
	}

	toMonthKey(year, month) {
		return `${String(year).padStart(4, "0")}-${String(month).padStart(2, "0")}`;
	}

	toDateKey(year, month, day) {
		return `${this.toMonthKey(year, month)}-${String(day).padStart(2, "0")}`;
	}

	firstWeekdayMondayBased(year, month) {
		const jsDay = new Date(Date.UTC(year, month - 1, 1)).getUTCDay();
		return (jsDay + 6) % 7;
	}

	daysInMonth(year, month) {
		return new Date(Date.UTC(year, month, 0)).getUTCDate();
	}

	addMonths(year, month, offset) {
		const d = new Date(Date.UTC(year, month - 1 + offset, 1));
		return { year: d.getUTCFullYear(), month: d.getUTCMonth() + 1 };
	}

	getTodayDateKey() {
		const now = new Date();
		return this.toDateKey(now.getFullYear(), now.getMonth() + 1, now.getDate());
	}

	canNavigateToPreviousMonth() {
		const now = new Date();
		const currentYear = now.getFullYear();
		const currentMonth = now.getMonth() + 1;

		if (this.activeYear > currentYear) {
			return true;
		}

		if (this.activeYear < currentYear) {
			return false;
		}

		return this.activeMonth > currentMonth;
	}

	updateMonthNavigationState() {
		if (!this.prevButton) {
			return;
		}

		this.prevButton.disabled = !this.canNavigateToPreviousMonth();
	}

	updateSelectedDateLabel() {
		if (!this.selectedDate) {
			this.dateLabel.textContent = this.getLocalizedText(this.translationKeys.lblNoDateSelected);
			return;
		}

		const parsed = new Date(`${this.selectedDate}T00:00:00Z`);
		const labelPrefix = this.hasUserChangedDate
			? this.getLocalizedText(this.translationKeys.lblSelectedDatePrefix)
			: this.getLocalizedText(this.translationKeys.lblEarliestDatePrefix);
		const displayedTime = this.getDisplayedTimeForSelectedDate();
		const timeLabel = displayedTime ? `, ${displayedTime}\u00A0${this.getLocalizedText(this.translationKeys.lblTimeSuffix)}` : "";
		this.dateLabel.textContent = `${labelPrefix}: ${this.selectedDateFormatter.format(parsed)}${timeLabel}`;
		this.updateWrapStates();
	}

	updateDateSummaryWrapState() {
		if (!this.dateSummary || !this.selectedDateWrap || !this.dateActions || !this.selectTimeButton || !this.dateLabel) {
			return;
		}

		this.dateSummary.classList.remove("is-wrapped");
		this.dateActions.classList.remove("is-wrapped");
		void this.dateSummary.offsetWidth;

		const summaryStyle = window.getComputedStyle(this.dateSummary);
		const gap = Number.parseFloat(summaryStyle.columnGap || summaryStyle.gap || "0") || 0;
		const paddingLeft = Number.parseFloat(summaryStyle.paddingLeft || "0") || 0;
		const paddingRight = Number.parseFloat(summaryStyle.paddingRight || "0") || 0;
		const availableWidth = this.dateSummary.clientWidth - paddingLeft - paddingRight;

		const labelRequiredWidth = this.dateLabel.scrollWidth;
		const actionsRequiredWidth = this.selectTimeButton.offsetWidth;
		const requiredWidth = labelRequiredWidth + actionsRequiredWidth + gap;

		const wrapped = requiredWidth > availableWidth + 0.5;
		this.dateSummary.classList.toggle("is-wrapped", wrapped);
		this.dateActions.classList.toggle("is-wrapped", wrapped);
	}

	updateWrapStates() {
		this.updateDateSummaryWrapState();
	}

	setCalendarExpanded(expanded, suppressRestore = false) {
		if (expanded) {
			if (!this.isDropdownSessionActive) {
				this.isDropdownSessionActive = true;
				this.dropdownScrollRestoreY = window.scrollY || window.pageYOffset || 0;
			}

			this.setTimeListExpanded(false, true);
		}

		this.calendar.hidden = !expanded;

		if (expanded) {
			requestAnimationFrame(() => {
				this.ensureCalendarCenteredInViewport();
			});
			return;
		}

		const allDropdownsClosed = this.calendar.hidden && this.timeList.hidden;
		if (!suppressRestore && allDropdownsClosed && Number.isFinite(this.dropdownScrollRestoreY)) {
			window.scrollTo({
				top: this.dropdownScrollRestoreY,
				behavior: "smooth"
			});
		}

		if (allDropdownsClosed && !suppressRestore) {
			this.isDropdownSessionActive = false;
			this.dropdownScrollRestoreY = null;
		}
	}

	setTimeListExpanded(expanded, suppressRestore = false) {
		if (!this.timeList || !this.selectTimeButton) {
			return;
		}

		if (expanded) {
			if (!this.isDropdownSessionActive) {
				this.isDropdownSessionActive = true;
				this.dropdownScrollRestoreY = window.scrollY || window.pageYOffset || 0;
			}

			this.setCalendarExpanded(false, true);
		}

		this.timeList.hidden = !expanded;
		this.selectTimeButton.setAttribute("aria-expanded", expanded ? "true" : "false");

		if (expanded) {
			requestAnimationFrame(() => {
				this.ensureTimeListCenteredInViewport();
			});
		} else {
			const allDropdownsClosed = this.calendar.hidden && this.timeList.hidden;
			if (!suppressRestore && allDropdownsClosed && Number.isFinite(this.dropdownScrollRestoreY)) {
				window.scrollTo({
					top: this.dropdownScrollRestoreY,
					behavior: "smooth"
				});
			}

			if (allDropdownsClosed && !suppressRestore) {
				this.isDropdownSessionActive = false;
				this.dropdownScrollRestoreY = null;
			}
		}

		this.updateWrapStates();
	}

	renderTimeOptions() {
		if (!this.timeOptions || !this.selectTimeButton) {
			return;
		}

		const slots = this.getTimeSlotsForDate(this.selectedDate);
		this.timeOptions.innerHTML = "";

		if (!this.selectedDate || slots.length === 0) {
			this.selectTimeButton.disabled = true;
			this.setTimeListExpanded(false);

			const emptyOption = document.createElement("button");
			emptyOption.type = "button";
			emptyOption.className = "time-option";
			emptyOption.disabled = true;
			emptyOption.textContent = "No time slots available";
			this.timeOptions.appendChild(emptyOption);
			return;
		}

		this.selectTimeButton.disabled = false;
		if (!slots.includes(this.selectedTime)) {
			this.selectedTime = slots[0] || "";
		}

		for (const slot of slots) {
			const optionButton = document.createElement("button");
			optionButton.type = "button";
			optionButton.className = "time-option";
			optionButton.textContent = slot;
			optionButton.classList.toggle("selected", slot === this.selectedTime);
			optionButton.addEventListener("click", (event) => {
				event.preventDefault();
				event.stopPropagation();
				this.selectedTime = slot;
				this.setAttribute("selected-time-source", "user");
				this.setAttribute("selected-time", slot);
				this.updateSelectedDateLabel();
				this.renderTimeOptions();
				this.setTimeListExpanded(false);
			});
			this.timeOptions.appendChild(optionButton);
		}
	}

	ensureCalendarCenteredInViewport() {
		if (!this.calendar || this.calendar.hidden) {
			return false;
		}

		const visualViewport = window.visualViewport;
		const viewportHeight = visualViewport?.height || window.innerHeight || document.documentElement.clientHeight;
		const viewportTop = visualViewport?.offsetTop || 0;
		const viewportBottom = viewportTop + viewportHeight;
		const calendarRect = this.calendar.getBoundingClientRect();
		const overflowTolerancePx = 2;
		if (calendarRect.bottom - viewportBottom <= overflowTolerancePx) {
			return false;
		}

		const currentScrollY = window.scrollY || window.pageYOffset || 0;
		const calendarCenterY = currentScrollY + calendarRect.top + (calendarRect.height / 2);
		const unclampedTargetY = Math.max(0, calendarCenterY - (viewportHeight / 2) - viewportTop);
		const maxScrollY = Math.max(0, document.documentElement.scrollHeight - viewportHeight);
		const targetScrollY = Math.min(maxScrollY, unclampedTargetY);

		window.scrollTo({
			top: targetScrollY,
			behavior: "smooth",
		});

		return true;
	}

	ensureTimeListCenteredInViewport() {
		if (!this.timeList || this.timeList.hidden) {
			return false;
		}

		const visualViewport = window.visualViewport;
		const viewportHeight = visualViewport?.height || window.innerHeight || document.documentElement.clientHeight;
		const viewportTop = visualViewport?.offsetTop || 0;
		const viewportBottom = viewportTop + viewportHeight;
		const timeListRect = this.timeList.getBoundingClientRect();
		const overflowTolerancePx = 2;
		if (timeListRect.bottom - viewportBottom <= overflowTolerancePx) {
			return false;
		}

		const currentScrollY = window.scrollY || window.pageYOffset || 0;
		const timeListCenterY = currentScrollY + timeListRect.top + (timeListRect.height / 2);
		const unclampedTargetY = Math.max(0, timeListCenterY - (viewportHeight / 2) - viewportTop);
		const maxScrollY = Math.max(0, document.documentElement.scrollHeight - viewportHeight);
		const targetScrollY = Math.min(maxScrollY, unclampedTargetY);

		window.scrollTo({
			top: targetScrollY,
			behavior: "smooth",
		});

		return true;
	}

	getAvailabilityForActiveMonth() {
		const monthKey = this.toMonthKey(this.activeYear, this.activeMonth);
		return this.availableDatesByMonth.get(monthKey);
	}

	getTimeSlotsForDate(dateKey) {
		if (!dateKey) {
			return [];
		}

		const availability = this.getAvailabilityForActiveMonth();
		const rawSlots = availability?.timeSlots?.[dateKey];
		return Array.isArray(rawSlots) ? rawSlots : [];
	}

	getFirstAvailableTimeForDate(dateKey) {
		const slots = this.getTimeSlotsForDate(dateKey);
		return slots.length > 0 ? slots[0] : "";
	}

	getDisplayedTimeForSelectedDate() {
		const slots = this.getTimeSlotsForDate(this.selectedDate);
		if (this.selectedTime && slots.includes(this.selectedTime)) {
			return this.selectedTime;
		}
		return slots.length > 0 ? slots[0] : "";
	}

	emitDateSelectedEvent(previousDate) {
		const timeSlots = this.getTimeSlotsForDate(this.selectedDate);
		this.dispatchEvent(new CustomEvent("x-date-time-picker-date-selected", {
			bubbles: true,
			detail: {
				date: this.selectedDate || "",
				previousDate: previousDate || "",
				timeSlots,
				hasUserChangedDate: this.hasUserChangedDate,
				year: this.activeYear,
				month: this.activeMonth
			}
		}));
	}

	emitMonthSelectedEvent(previousYear, previousMonth) {
		this.dispatchEvent(new CustomEvent("x-date-time-picker-month-selected", {
			bubbles: true,
			detail: {
				year: this.activeYear,
				month: this.activeMonth,
				previousYear,
				previousMonth
			}
		}));
	}

	setSelectedDate(value) {
		const nextDate = typeof value === "string" ? value : "";
		const previousDate = this.selectedDate || "";
		this.selectedDate = nextDate;
		this.renderTimeOptions();
		[...this.grid.querySelectorAll("button.calendar-day")].forEach((button) => {
			button.classList.toggle("selected", button.dataset.date === nextDate);
		});
		this.updateSelectedDateLabel();

		if (previousDate !== nextDate) {
			this.emitDateSelectedEvent(previousDate);
		}
	}

	applyAvailabilityForActiveMonth() {
		if (!this.grid) {
			return;
		}

		const monthKey = this.toMonthKey(this.activeYear, this.activeMonth);
		const availability = this.availableDatesByMonth.get(monthKey);
		const availableDates = Array.isArray(availability?.dates) ? availability.dates : [];
		const availableSet = new Set(availableDates);
		const todayDateKey = this.getTodayDateKey();

		let firstAvailable = "";
		[...this.grid.querySelectorAll("button.calendar-day")].forEach((button) => {
			const dateKey = button.dataset.date || "";
			const enabled = availableSet.has(dateKey) && dateKey >= todayDateKey;
			button.disabled = !enabled;
			button.classList.toggle("disabled", !enabled);
			if (enabled && !firstAvailable) {
				firstAvailable = dateKey;
			}
		});

		if (!availability) {
			this.setHelpText(this.getLocalizedText(this.translationKeys.helpLoadingMonth));
			this.setSelectedDate("");
			return;
		}

		if (this.selectedDate && this.selectedDate.startsWith(`${monthKey}-`) && availableSet.has(this.selectedDate)) {
			this.setSelectedDate(this.selectedDate);
		} else if (firstAvailable) {
			this.setSelectedDate(firstAvailable);
		} else {
			this.setSelectedDate("");
		}

		this.setHelpText(firstAvailable
			? this.getLocalizedText(this.translationKeys.helpChooseEnabledDate)
			: this.getLocalizedText(this.translationKeys.helpNoDatesInMonth));
	}

	async renderMonth() {
		this.setHelpText(this.getLocalizedText(this.translationKeys.helpLoadingMonth));
		this.updateMonthNavigationState();

		const monthStart = new Date(this.activeYear, this.activeMonth - 1, 1);
		this.monthLabel.textContent = this.monthLabelFormatter.format(monthStart);
		const totalDays = this.daysInMonth(this.activeYear, this.activeMonth);
		const offset = this.firstWeekdayMondayBased(this.activeYear, this.activeMonth);
		const now = new Date();
		const isCurrentMonth = this.activeYear === now.getFullYear() && this.activeMonth === now.getMonth() + 1;
		const todayDay = now.getDate();
		const monthNodes = [];

		for (let i = 0; i < offset; i++) {
			const spacer = document.createElement("span");
			spacer.className = "calendar-spacer";
			monthNodes.push(spacer);
		}

		for (let day = 1; day <= totalDays; day++) {
			const dateKey = this.toDateKey(this.activeYear, this.activeMonth, day);
			const button = document.createElement("button");
			button.type = "button";
			button.className = "calendar-day disabled";
			button.textContent = String(day);
			button.dataset.date = dateKey;
			button.disabled = true;
			if (isCurrentMonth && day === todayDay) {
				button.setAttribute("aria-current", "date");
			}

			button.addEventListener("click", () => {
				if (button.disabled) {
					return;
				}
				this.hasUserChangedDate = true;
				this.setSelectedDate(dateKey);
				this.setCalendarExpanded(false, true);
				this.setTimeListExpanded(true);
				this.setHelpText(this.getLocalizedText(this.translationKeys.helpDateUpdated));
			});

			monthNodes.push(button);
		}

		this.grid.replaceChildren(...monthNodes);
		this.applyAvailabilityForActiveMonth();
		this.updateMonthNavigationState();
	}

	async initialize() {
		this.selectTimeButton.disabled = false;
		const now = new Date();
		this.activeYear = now.getFullYear();
		this.activeMonth = now.getMonth() + 1;
		await this.renderMonth();
		this.setCalendarExpanded(false);

		if (!this.hasEmittedInitializedEvent) {
			this.dispatchEvent(new CustomEvent("x-date-time-picker-initialized", {
				bubbles: true,
				detail: {
					year: this.activeYear,
					month: this.activeMonth,
					selectedDate: this.selectedDate || ""
				}
			}));
			this.hasEmittedInitializedEvent = true;
		}
	}

	disconnectedCallback() {
		if (this.boundUpdateWrapStates) {
			window.removeEventListener("resize", this.boundUpdateWrapStates);
			this.boundUpdateWrapStates = null;
		}

		if (this.datePickerResizeObserver) {
			this.datePickerResizeObserver.disconnect();
			this.datePickerResizeObserver = null;
		}
	}
}

if (!customElements.get("x-date-time-picker")) {
	customElements.define("x-date-time-picker", xDateTimePicker);
}