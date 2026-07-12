// Formatting helpers shared across pages.

// Cost is stored as micro-dollars (1e6 = $1).
export function formatCost(micros: number): string {
  const dollars = micros / 1_000_000;
  if (dollars === 0) return '$0';
  if (dollars < 0.01) return '$' + dollars.toFixed(6).replace(/0+$/, '');
  return '$' + dollars.toFixed(4).replace(/0+$/, '').replace(/\.$/, '');
}

export function formatNumber(n: number): string {
  return n.toLocaleString('en-US');
}

export function formatTime(unix: number): string {
  const d = new Date(unix * 1000);
  const pad = (x: number) => String(x).padStart(2, '0');
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
}
