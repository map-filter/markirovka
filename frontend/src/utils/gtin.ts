export function gtinCheckDigit(body: string): number {
  let sum = 0
  for (let i = 0; i < body.length; i++) {
    const digit = Number(body[body.length - 1 - i])
    sum += i % 2 === 0 ? digit * 3 : digit
  }
  return (10 - (sum % 10)) % 10
}

export function validateGtin(value: string): string | null {
  const v = value.trim()
  if (!v) return 'Укажите GTIN'
  if (!/^\d+$/.test(v)) return 'GTIN состоит только из цифр'
  if (v.length !== 13 && v.length !== 14) return 'GTIN должен содержать 13 или 14 цифр'
  const expected = gtinCheckDigit(v.slice(0, -1))
  if (Number(v[v.length - 1]) !== expected) return `Неверная контрольная цифра: должна быть ${expected}`
  return null
}

export function toGtin14(value: string): string {
  const v = value.trim()
  return v.length >= 14 ? v.slice(0, 14) : v.padStart(14, '0')
}
