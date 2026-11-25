const TZ = 'Asia/Almaty'

const dateTime = new Intl.DateTimeFormat('ru-RU', {
  timeZone: TZ,
  day: '2-digit',
  month: '2-digit',
  year: 'numeric',
  hour: '2-digit',
  minute: '2-digit',
})
const dateOnly = new Intl.DateTimeFormat('ru-RU', { timeZone: TZ, day: '2-digit', month: '2-digit', year: 'numeric' })
const timeOnly = new Intl.DateTimeFormat('ru-RU', { timeZone: TZ, hour: '2-digit', minute: '2-digit' })
const numberFmt = new Intl.NumberFormat('ru-RU')

function parse(value: string | null | undefined): Date | null {
  if (!value) return null
  const d = new Date(value)
  return Number.isNaN(d.getTime()) ? null : d
}

export function formatDateTime(value: string | null | undefined): string {
  const d = parse(value)
  return d ? dateTime.format(d) : '-'
}

export function formatDate(value: string | null | undefined): string {
  const d = parse(value)
  return d ? dateOnly.format(d) : '-'
}

export function formatTime(value: string | null | undefined): string {
  const d = parse(value)
  return d ? timeOnly.format(d) : '-'
}

export function isSameDayAlmaty(value: string | null | undefined): boolean {
  const d = parse(value)
  return !!d && dateOnly.format(d) === dateOnly.format(new Date())
}

export function formatUntil(value: string | null | undefined): string {
  const d = parse(value)
  if (!d) return '-'
  return isSameDayAlmaty(value) ? timeOnly.format(d) : dateTime.format(d)
}

export function formatNumber(value: number | null | undefined): string {
  return numberFmt.format(value ?? 0)
}

export function plural(n: number, one: string, few: string, many: string): string {
  const abs = Math.abs(n) % 100
  const last = abs % 10
  if (abs > 10 && abs < 20) return many
  if (last > 1 && last < 5) return few
  if (last === 1) return one
  return many
}

export function codesWord(n: number): string {
  return plural(n, 'код', 'кода', 'кодов')
}

export function daysWord(n: number): string {
  return plural(n, 'день', 'дня', 'дней')
}

export function daysUntil(value: string | null | undefined): number | null {
  const d = parse(value)
  if (!d) return null
  return Math.ceil((d.getTime() - Date.now()) / 86400000)
}
