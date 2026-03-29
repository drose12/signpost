// web/src/theme.ts

export function getTheme(): 'light' | 'dark' {
  return (localStorage.getItem('signpost-theme') as 'light' | 'dark') || 'light';
}

export function setTheme(theme: 'light' | 'dark') {
  localStorage.setItem('signpost-theme', theme);
  if (theme === 'dark') {
    document.documentElement.classList.add('dark');
  } else {
    document.documentElement.classList.remove('dark');
  }
}

export function initTheme() {
  setTheme(getTheme());
}
