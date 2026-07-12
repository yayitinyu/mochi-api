// Theme helpers. The initial theme is applied by an inline script in
// index.html before first paint; these functions handle runtime toggling.

export function isDark(): boolean {
  return document.documentElement.classList.contains('dark');
}

export function toggleTheme(): boolean {
  const dark = !isDark();
  document.documentElement.classList.toggle('dark', dark);
  localStorage.setItem('mochi-theme', dark ? 'dark' : 'light');
  return dark;
}
