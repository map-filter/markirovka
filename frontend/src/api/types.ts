export interface PageMeta {
  current_page: number
  last_page: number
  per_page: number
  total: number
}

export interface Paginated<T> {
  data: T[]
  meta: PageMeta
}

export interface DataWrap<T> {
  data: T
}

export type UserRole = 'admin' | 'operator'

export interface User {
  id: number
  name: string
  email: string
  role: UserRole
}

export interface Organization {
  id: number
  name: string
  bin: string
  address: string
  phone: string | null
}

export interface LoginResponse {
  token: string
  user: User
}

export interface MeResponse {
  user: User
  organization: Organization
}

export type SignMode = 'ncalayer' | 'emulate'
export type IsmptMode = 'emulator' | 'production'

export interface AppConfig {
  sign_mode: SignMode
  ismpt_mode: IsmptMode
  ismpt_base_url: string
  emulator_console_url: string | null
  annul_days: number
}

export interface IsmptSession {
  connected: boolean
  expires_at: string | null
  mode: IsmptMode
}

export interface IsmptAuthKey {
  uuid: string
  data: string
}

export type CodeStatus = 'received' | 'printed' | 'applied' | 'introduced' | 'annulled'
export type OrderStatus = 'new' | 'sent' | 'processing' | 'ready' | 'failed'
export type DocumentStatus = 'draft' | 'sent' | 'processing' | 'accepted' | 'rejected' | 'failed'
export type DocumentType = 'application' | 'introduction'
export type PrintJobStatus = 'ready' | 'queued' | 'sent' | 'printed' | 'failed'
export type PrintFormat = 'pdf' | 'zpl'
export type PrintTarget = 'download' | 'agent'
export type NktStatus = 'unchecked' | 'found' | 'not_found'
export type OperationLevel = 'info' | 'warning' | 'error'
export type Severity = 'info' | 'warning' | 'danger'
export type LabelSize = '58x40' | '43x25'

export interface CodeCounts {
  received: number
  printed: number
  applied: number
  introduced: number
  annulled: number
}

export type TodoKind =
  | 'connect_ismpt'
  | 'nkt_missing'
  | 'order_ready'
  | 'print_codes'
  | 'apply_codes'
  | 'introduce_codes'
  | 'expiring_codes'
  | 'order_failed'
  | 'document_rejected'

export interface TodoItem {
  kind: TodoKind
  severity: Severity
  title: string
  description: string
  count: number | null
  action?: {
    label: string
    route: string
    query?: Record<string, unknown>
  } | null
}

export interface ProductShort {
  id: number
  gtin: string
  name: string
}

export interface ProductInput {
  gtin: string
  name: string
  model?: string
  size?: string
  color?: string
  composition?: string
  tnved?: string
  category?: string
  brand?: string
  country?: string
}

export interface Product extends ProductInput {
  id: number
  nkt_status: NktStatus
  nkt_checked_at: string | null
  codes: CodeCounts
  created_at: string
}

export interface ImportResult {
  created: number
  updated: number
  errors: { row: number; message: string }[]
}

export interface CodeOrder {
  id: number
  number: string
  product: ProductShort
  quantity: number
  status: OrderStatus
  ismpt_order_id: string | null
  error: string | null
  codes: CodeCounts
  created_at: string
  ready_at: string | null
  expires_at: string | null
}

export interface Code {
  id: number
  code: string
  code_hr: string
  gtin: string
  serial: string
  status: CodeStatus
  product: ProductShort
  order_id: number
  order_number: string
  emitted_at: string
  expires_at: string
  printed_at: string | null
  applied_at: string | null
  introduced_at: string | null
}

export interface CodeSummaryRow {
  product: ProductShort
  counts: CodeCounts
}

export interface LabelFields {
  datamatrix: boolean
  gtin: boolean
  name: boolean
  size: boolean
  composition: boolean
  manufacturer: boolean
  serial: boolean
}

export interface LabelTemplateInput {
  name: string
  size: LabelSize
  fields: LabelFields
  font_scale: number
  is_default: boolean
}

export interface LabelTemplate extends LabelTemplateInput {
  id: number
  width_mm: number
  height_mm: number
}

export interface PrintAgent {
  id: number
  name: string
  hostname: string | null
  printer: string | null
  version: string | null
  last_seen_at: string | null
  online: boolean
}

export interface PrintJob {
  id: number
  template: LabelTemplate | null
  format: PrintFormat
  target: PrintTarget
  status: PrintJobStatus
  codes_count: number
  agent: PrintAgent | null
  error: string | null
  download_url: string
  created_at: string
}

export interface PrintJobInput {
  template_id: number
  format: PrintFormat
  target: PrintTarget
  agent_id?: number | null
  code_ids?: number[]
  product_id?: number
  order_id?: number
  quantity?: number
  reprint?: boolean
}

export interface DocumentError {
  code: string
  message: string
}

export interface IsmptDocument {
  id: number
  type: DocumentType
  type_label: string
  status: DocumentStatus
  codes_count: number
  body_base64: string
  ismpt_document_id: string | null
  errors: DocumentError[] | null
  created_at: string
  processed_at: string | null
}

export interface DocumentSelection {
  code_ids?: number[]
  product_id?: number
  order_id?: number
}

export interface Operation {
  id: number
  created_at: string
  user: string | null
  type: string
  level: OperationLevel
  message: string
}

export interface ExpiringItem {
  product: ProductShort
  order_id: number
  order_number: string
  count: number
  expires_at: string
  days_left: number
}

export interface Dashboard {
  ismpt: IsmptSession
  stats: {
    products: number
    orders_in_progress: number
    codes: CodeCounts
  }
  todo: TodoItem[]
  expiring: ExpiringItem[]
  recent_operations: Operation[]
}

export interface Settings {
  organization: Organization
  integration_token_set: boolean
  agents: PrintAgent[]
}

export interface OrganizationInput {
  name: string
  address: string
  phone: string
}
