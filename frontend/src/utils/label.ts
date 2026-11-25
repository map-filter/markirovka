import type { Code, LabelFields, Organization, Product } from '@/api'
import { toGtin14 } from './gtin'

export interface LabelSample {
  name: string
  size: string
  composition: string
  gtin: string
  manufacturer: string
  serial: string
  codeHr: string
  codeRaw: string | null
}

export const defaultFields: LabelFields = {
  datamatrix: true,
  gtin: true,
  name: true,
  size: true,
  composition: true,
  manufacturer: true,
  serial: false,
}

const SERIAL_CHARS = 'ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz0123456789'
const B64 = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/'

function pick(chars: string, n: number, seed: number): string {
  let out = ''
  let x = seed || 1
  for (let i = 0; i < n; i++) {
    x = (x * 1103515245 + 12345) & 0x7fffffff
    out += chars[x % chars.length]
  }
  return out
}

export function manufacturerLine(org: Organization | null): string {
  if (!org) return ''
  return [org.name, org.address].filter(Boolean).join(', ')
}

export function buildSample(product: Product | null, code: Code | null, org: Organization | null): LabelSample {
  const gtin14 = toGtin14(code?.gtin || product?.gtin || '04870712001017')
  const seed = Number(gtin14.slice(-6)) || 7
  const serial = code?.serial || pick(SERIAL_CHARS, 13, seed)
  const codeHr = code?.code_hr || `(01)${gtin14}(21)${serial}(91)EE10(92)${pick(B64, 44, seed + 17)}`
  return {
    name: product?.name || code?.product.name || 'Футболка женская хлопковая',
    size: product?.size || '44',
    composition: product?.composition || '100% хлопок',
    gtin: gtin14,
    manufacturer: manufacturerLine(org) || 'ТОО «Производитель», Казахстан',
    serial,
    codeHr,
    codeRaw: code?.code || null,
  }
}
