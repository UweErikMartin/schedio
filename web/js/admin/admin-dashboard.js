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
    max-width: 900px;
  }

  .page-title {
    margin: 0 0 0.25rem;
    font-size: 1.5rem;
    font-weight: 700;
    color: #1f2937;
  }

  .date-label {
    margin-bottom: 1.75rem;
    font-size: 0.9375rem;
    color: #6b7280;
  }

  .panels {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(260px, 1fr));
    gap: 1.25rem;
  }

  .panel {
    background: #ffffff;
    border: 1px solid #d1d5db;
    border-radius: 12px;
    padding: 1.5rem;
  }
  .panel-title {
    margin: 0 0 0.5rem;
    font-size: 1rem;
    font-weight: 600;
    color: #374151;
  }
  .panel-body {
    color: #6b7280;
    font-size: 0.9rem;
    line-height: 1.5;
  }
`;

const TEMPLATE = document.createElement('template');
TEMPLATE.innerHTML = `
  <style>${STYLES}</style>
  <div class="page">
    <h1 class="page-title">Dashboard</h1>
    <p class="date-label" id="date-label"></p>
    <div class="panels">
      <div class="panel">
        <div class="panel-title">Heutige Termine</div>
        <div class="panel-body">Übersicht der heutigen Buchungen (folgt).</div>
      </div>
      <div class="panel">
        <div class="panel-title">Ausstehende Anfragen</div>
        <div class="panel-body">Offene Buchungsanfragen (folgt).</div>
      </div>
    </div>
  </div>
`;

/**
 * `<x-admin-dashboard>` — main dashboard panel.
 *
 * Shows the current date and placeholder panels for today's bookings and
 * pending requests. Detailed panels will be added in follow-up iterations.
 */
class XAdminDashboard extends HTMLElement {
  connectedCallback() {
    const root = this.attachShadow({ mode: 'open' });
    root.appendChild(TEMPLATE.content.cloneNode(true));

    const now = new Date();
    root.getElementById('date-label').textContent = now.toLocaleDateString('de-DE', {
      weekday: 'long',
      day:     'numeric',
      month:   'long',
      year:    'numeric',
    });
  }
}

customElements.define('x-admin-dashboard', XAdminDashboard);
