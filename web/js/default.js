(function () {
const dateInput = document.getElementById("booking-date");
const timeInput = document.getElementById("booking-time");
const serviceInput = document.getElementById("service-value");
const servicePicker = document.getElementById("service-picker");
const datePicker = document.getElementById("date-time-picker") || document.querySelector("x-date-time-picker");

if (!datePicker) {
    return;
}

const hasDateSelectionControls = !!dateInput && !!timeInput && !!datePicker;

let activeYear = 0;
let activeMonth = 0;
let preloadedAvailability = null;
let preloadedAvailabilityResult = null;

const buildMonthKey = (year, month) => {
    return `${String(year).padStart(4, "0")}-${String(month).padStart(2, "0")}`;
};

const loadServices = async () => {
    const response = await fetch("services");
    if (!response.ok) {
        throw new Error("Behandlungen konnten nicht geladen werden");
    }
    const payload = await response.json();
    return Array.isArray(payload?.services) ? payload.services : [];
};

const getAvailabilityEndpoint = () => {
    const configuredEndpoint = (datePicker.getAttribute("data-availability-endpoint") || "").trim();
    return configuredEndpoint || "availability";
};

const loadAvailability = async (year, month) => {
    const monthKey = buildMonthKey(year, month);
    const endpoint = getAvailabilityEndpoint();
    const separator = endpoint.includes("?") ? "&" : "?";
    const response = await fetch(`${endpoint}${separator}month=${encodeURIComponent(monthKey)}`);
    if (!response.ok) {
        throw new Error("Verfügbarkeit konnte nicht geladen werden");
    }
    const payload = await response.json();
    const monthDates = payload?.months?.[monthKey] && typeof payload.months[monthKey] === "object"
        ? payload.months[monthKey]
        : {};

    const dates = Object.keys(monthDates).sort();
    return {
        month: monthKey,
        dates,
        timeSlots: monthDates
    };
};

const preloadAvailabilityForCurrentMonth = () => {
    if (!hasDateSelectionControls) {
    return null;
    }

    const now = new Date();
    const year = now.getFullYear();
    const month = now.getMonth() + 1;
    const monthKey = buildMonthKey(year, month);

    if (preloadedAvailability?.monthKey === monthKey) {
    return preloadedAvailability.promise;
    }

    preloadedAvailabilityResult = null;
    preloadedAvailability = {
    monthKey,
    year,
    month,
    promise: loadAvailability(year, month)
        .then((availability) => {
        preloadedAvailabilityResult = availability;
        if ((activeYear === 0 && activeMonth === 0) || (activeYear === year && activeMonth === month)) {
            datePicker.setAttribute("available-dates", JSON.stringify(availability));
        }
        return availability;
        })
        .catch((error) => {
        console.error("failed to preload available dates from backend", error);
        return null;
        })
    };

    return preloadedAvailability.promise;
};

const getSelectedTimeFromPicker = () => {
    const selectedTimeAttr = (datePicker.getAttribute("selected-time") || "").trim();
    if (selectedTimeAttr) {
    return selectedTimeAttr;
    }

    const selectedTimeProperty = typeof datePicker.selectedTime === "string" ? datePicker.selectedTime.trim() : "";
    return selectedTimeProperty;
};

const syncSelectedTime = (timeValue) => {
    if (!timeInput) {
    return;
    }
    const normalizedTime = (timeValue || "").trim();
    timeInput.value = normalizedTime;
    timeInput.setCustomValidity(normalizedTime ? "" : "Bitte wähle eine verfügbare Uhrzeit aus.");
};

const initializeServices = async () => {
    if (!servicePicker) {
        return;
    }

    await customElements.whenDefined("x-service-picker");

    const syncSelectedService = (serviceName) => {
        if (!serviceInput) {
            return;
        }
        serviceInput.value = serviceName || "";
    };

    const applyServicesToPicker = (services) => {
        const normalizedServices = Array.isArray(services) ? services : [];
        servicePicker.setAttribute("data", JSON.stringify(normalizedServices));
        if (typeof servicePicker.setServices === "function") {
            servicePicker.setServices(normalizedServices);
        }
        syncSelectedService(servicePicker.selectedServiceName || "");
    };

    servicePicker.addEventListener("service-change", (event) => {
    syncSelectedService(event.detail?.serviceName || "");

    if (hasDateSelectionControls && activeYear > 0 && activeMonth > 0) {
        fetchAndApplyAvailability({ year: activeYear, month: activeMonth });
    }
    });

    try {
        const services = await loadServices();
        applyServicesToPicker(services);
    } catch (error) {
        applyServicesToPicker([]);
    }
};

const applyTimeSlots = (slots = []) => {
    if (!hasDateSelectionControls) {
        return;
    }
    if (slots.length === 0) {
        datePicker.setAttribute("selected-time-source", "auto");
        datePicker.setAttribute("selected-time", "");
        syncSelectedTime("");
        timeInput.setCustomValidity("Für dieses Datum sind keine freien Uhrzeiten verfügbar.");
        return;
    }

    const pickerTime = getSelectedTimeFromPicker();
    const selectedTime = pickerTime && slots.includes(pickerTime) ? pickerTime : slots[0];
    datePicker.setAttribute("selected-time-source", "auto");
    datePicker.setAttribute("selected-time", selectedTime);
    syncSelectedTime(selectedTime);
};

const initialize = async () => {
    preloadAvailabilityForCurrentMonth();
    await initializeServices();
    if (hasDateSelectionControls) {
        applyTimeSlots([]);

        const now = new Date();
        await fetchAndApplyAvailability({
            year: now.getFullYear(),
            month: now.getMonth() + 1
        });
    }
};

if (hasDateSelectionControls) {
    datePicker.addEventListener("x-date-time-picker-date-selected", (event) => {
        const selectedDate = event.detail?.date || "";
        const slots = Array.isArray(event.detail?.timeSlots) ? event.detail.timeSlots : [];
        dateInput.value = selectedDate;
        dateInput.setCustomValidity(selectedDate ? "" : "Bitte wähle ein verfügbares Datum aus.");
        applyTimeSlots(slots);
    });

    const selectedTimeObserver = new MutationObserver(() => {
        syncSelectedTime(getSelectedTimeFromPicker());
    });

    selectedTimeObserver.observe(datePicker, {
        attributes: true,
        attributeFilter: ["selected-time", "selectedtime", "selectedTime"]
    });
}

const fetchAndApplyAvailability = async (eventDetail) => {
    if (!hasDateSelectionControls) {
        return;
    }
    const year = Number(eventDetail?.year);
    const month = Number(eventDetail?.month);
    if (!year || !month) {
        return;
    }

    const monthKey = buildMonthKey(year, month);

    activeYear = year;
    activeMonth = month;

    try {
        let availability = null;
        if (preloadedAvailability?.monthKey === monthKey) {
            availability = preloadedAvailabilityResult || await preloadedAvailability.promise;
        }
        if (!availability) {
            availability = await loadAvailability(year, month);
        }
        datePicker.setAttribute("available-dates", JSON.stringify(availability));
    } catch (error) {
        console.error("failed to fetch available dates from backend", error);
    }
};

if (hasDateSelectionControls) {
datePicker.addEventListener("x-date-time-picker-initialized", async (event) => {
    await fetchAndApplyAvailability(event.detail || {});
});

datePicker.addEventListener("x-date-time-picker-month-selected", async (event) => {
    await fetchAndApplyAvailability(event.detail || {});
});
}

initialize();
})();
