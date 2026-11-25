import { toSVG } from 'bwip-js/browser'
import type { RenderOptions } from 'bwip-js/browser'

const GS = '\u001d'

type Options = RenderOptions & { dontlint?: boolean }

function hasNestedParens(hr: string): boolean {
  const re = /\((\d{2,4})\)([^]*?)(?=\(\d{2,4}\)|$)/g
  let m: RegExpExecArray | null
  let total = 0
  while ((m = re.exec(hr))) {
    if (/[()]/.test(m[2])) return true
    total += m[0].length
    if (m[0].length === 0) break
  }
  return total !== hr.length
}

export function hrToRaw(hr: string): string {
  const parts: string[] = []
  const re = /\((\d{2,4})\)([^(]*)/g
  let m: RegExpExecArray | null
  while ((m = re.exec(hr))) parts.push(`${m[1]}${m[2]}`)
  return parts.join(GS)
}

function render(opts: Options): string {
  return toSVG({ scale: 1, paddingwidth: 0, paddingheight: 0, ...opts })
}

export function renderDataMatrix(hr: string, raw?: string | null): string {
  const errors: unknown[] = []
  const cleanRaw = raw && raw.length ? raw.replace(/^\u00e8|^\]d2/, '') : ''
  const consistent = !cleanRaw || hrToRaw(hr) === cleanRaw
  if (hr && consistent && !hasNestedParens(hr)) {
    try {
      return render({ bcid: 'gs1datamatrix', text: hr })
    } catch (e) {
      errors.push(e)
    }
  }
  const source = cleanRaw || hrToRaw(hr)
  if (source) {
    try {
      return render({ bcid: 'datamatrix', text: `^FNC1${source.split(GS).join('^FNC1')}`, parsefnc: true })
    } catch (e) {
      errors.push(e)
    }
  }
  throw errors[0] ?? new Error('empty')
}
