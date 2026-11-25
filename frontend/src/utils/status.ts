import type {
  CodeStatus,
  DocumentStatus,
  NktStatus,
  OperationLevel,
  OrderStatus,
  PrintJobStatus,
  Severity,
} from '@/api'

export type Tone = 'neutral' | 'info' | 'success' | 'warning' | 'danger' | 'accent' | 'muted'

export interface StatusInfo {
  label: string
  tone: Tone
}

export const codeStatuses: Record<CodeStatus, StatusInfo> = {
  received: { label: 'Получен', tone: 'info' },
  printed: { label: 'Напечатан', tone: 'accent' },
  applied: { label: 'Нанесён', tone: 'warning' },
  introduced: { label: 'В обороте', tone: 'success' },
  annulled: { label: 'Аннулирован', tone: 'muted' },
}

export const orderStatuses: Record<OrderStatus, StatusInfo> = {
  new: { label: 'Новый', tone: 'neutral' },
  sent: { label: 'Отправлен', tone: 'info' },
  processing: { label: 'В обработке', tone: 'warning' },
  ready: { label: 'Готов', tone: 'success' },
  failed: { label: 'Ошибка', tone: 'danger' },
}

export const documentStatuses: Record<DocumentStatus, StatusInfo> = {
  draft: { label: 'Черновик', tone: 'neutral' },
  sent: { label: 'Отправлен', tone: 'info' },
  processing: { label: 'Обрабатывается', tone: 'warning' },
  accepted: { label: 'Принят', tone: 'success' },
  rejected: { label: 'Отклонён', tone: 'danger' },
  failed: { label: 'Ошибка', tone: 'danger' },
}

export const printJobStatuses: Record<PrintJobStatus, StatusInfo> = {
  ready: { label: 'Готово к скачиванию', tone: 'success' },
  queued: { label: 'В очереди агента', tone: 'warning' },
  sent: { label: 'Передано агенту', tone: 'info' },
  printed: { label: 'Напечатано', tone: 'success' },
  failed: { label: 'Ошибка', tone: 'danger' },
}

export const nktStatuses: Record<NktStatus, StatusInfo> = {
  unchecked: { label: 'Не проверен', tone: 'neutral' },
  found: { label: 'Есть в НКТ', tone: 'success' },
  not_found: { label: 'Нет в НКТ', tone: 'danger' },
}

export const levelStatuses: Record<OperationLevel, StatusInfo> = {
  info: { label: 'Инфо', tone: 'info' },
  warning: { label: 'Внимание', tone: 'warning' },
  error: { label: 'Ошибка', tone: 'danger' },
}

export const severityTone: Record<Severity, Tone> = {
  info: 'info',
  warning: 'warning',
  danger: 'danger',
}

export const documentTypes: Record<string, string> = {
  application: 'Отчёт о нанесении',
  introduction: 'Ввод в оборот',
}

export function statusOf<K extends string>(map: Record<K, StatusInfo>, key: string | null | undefined): StatusInfo {
  if (key && key in map) return map[key as K]
  return { label: key ?? '-', tone: 'neutral' }
}

export const codeStatusOrder: CodeStatus[] = ['received', 'printed', 'applied', 'introduced', 'annulled']
