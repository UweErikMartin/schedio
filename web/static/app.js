document.getElementById('ping')?.addEventListener('click', async () => {
  const out = document.getElementById('result');
  if (!out) return;

  out.textContent = 'loading...';

  try {
    const response = await fetch('/healthz');
    const body = await response.text();
    out.textContent = `${response.status} ${body}`;
  } catch (error) {
    out.textContent = `error: ${error}`;
  }
});
