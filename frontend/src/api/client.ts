const API_BASE = 'http://localhost:8080/api';

export const apiClient = {
  async start(payload: any) {
    const resp = await fetch(`${API_BASE}/start`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    });
    if (!resp.ok) throw new Error(await resp.text());
    return resp.json();
  },

  async stop() {
    const resp = await fetch(`${API_BASE}/stop`, { method: 'POST' });
    return resp.json();
  },

  async getStatus() {
    const resp = await fetch(`${API_BASE}/status`);
    return resp.json();
  },
};