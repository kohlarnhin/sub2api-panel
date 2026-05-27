export function formatNumber(n: number): string {
  if (n === null || n === undefined || Number.isNaN(n)) return '-'
  return n.toLocaleString('en-US')
}

export function formatCompact(n: number): string {
  if (n === null || n === undefined || Number.isNaN(n)) return '-'
  if (Math.abs(n) >= 1_000_000_000) return (n / 1_000_000_000).toFixed(2) + 'B'
  if (Math.abs(n) >= 1_000_000) return (n / 1_000_000).toFixed(2) + 'M'
  if (Math.abs(n) >= 1_000) return (n / 1_000).toFixed(2) + 'K'
  return String(n)
}

export function formatCurrency(n: number, digits = 4): string {
  if (n === null || n === undefined || Number.isNaN(n)) return '-'
  return '$' + n.toFixed(digits)
}

export function formatCompactCurrency(n: number): string {
  if (n === null || n === undefined || Number.isNaN(n)) return '-'
  const abs = Math.abs(n)
  if (abs >= 1_000_000_000) return '$' + (n / 1_000_000_000).toFixed(2) + 'B'
  if (abs >= 1_000_000) return '$' + (n / 1_000_000).toFixed(2) + 'M'
  if (abs >= 1_000) return '$' + (n / 1_000).toFixed(2) + 'K'
  return '$' + n.toFixed(2)
}

export function formatDateShort(iso: string): string {
  // iso is YYYY-MM-DD
  const [, m, d] = iso.split('-')
  return `${m}/${d}`
}

export function formatTime(iso: string): string {
  if (!iso) return ''
  try {
    return new Date(iso).toLocaleTimeString('zh-CN', { hour12: false })
  } catch {
    return iso
  }
}
