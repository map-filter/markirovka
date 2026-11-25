import { request, requestBlob } from './http'
import type {
  AppConfig,
  Code,
  CodeOrder,
  CodeStatus,
  CodeSummaryRow,
  Dashboard,
  DataWrap,
  DocumentSelection,
  DocumentType,
  ImportResult,
  IsmptAuthKey,
  IsmptDocument,
  IsmptSession,
  LabelTemplate,
  LabelTemplateInput,
  LoginResponse,
  MeResponse,
  Operation,
  OperationLevel,
  OrderStatus,
  Organization,
  OrganizationInput,
  Paginated,
  PrintAgent,
  PrintJob,
  PrintJobInput,
  Product,
  ProductInput,
  Settings,
} from './types'

export * from './types'
export { ApiError, getToken, setToken, saveBlob, openBlob } from './http'

export const api = {
  login: (email: string, password: string) =>
    request<LoginResponse>('/auth/login', { method: 'POST', body: { email, password }, auth: false }),
  logout: () => request<void>('/auth/logout', { method: 'POST' }),
  me: () => request<MeResponse>('/me'),
  config: () => request<AppConfig>('/config', { auth: false }),
  dashboard: () => request<Dashboard>('/dashboard'),

  ismptSession: () => request<IsmptSession>('/ismpt/session'),
  ismptAuthKey: () => request<IsmptAuthKey>('/ismpt/auth/key', { method: 'POST' }),
  ismptSignIn: (uuid: string, signature: string) =>
    request<IsmptSession>('/ismpt/auth/sign-in', { method: 'POST', body: { uuid, signature } }),
  signEmulate: (dataBase64: string) =>
    request<{ signature: string }>('/sign/emulate', { method: 'POST', body: { data_base64: dataBase64 } }),

  products: (query: { search?: string; category?: string; page?: number; per_page?: number }) =>
    request<Paginated<Product>>('/products', { query }),
  productCategories: () => request<{ data: string[] }>('/products/categories'),
  product: (id: number) => request<DataWrap<Product>>(`/products/${id}`),
  createProduct: (input: ProductInput) => request<DataWrap<Product>>('/products', { method: 'POST', body: input }),
  updateProduct: (id: number, input: ProductInput) =>
    request<DataWrap<Product>>(`/products/${id}`, { method: 'PUT', body: input }),
  deleteProduct: (id: number) => request<void>(`/products/${id}`, { method: 'DELETE' }),
  nktCheck: (id: number) =>
    request<{ data: Product; nkt: Record<string, unknown> | null }>(`/products/${id}/nkt-check`, { method: 'POST' }),
  nktRegister: (id: number) => request<DataWrap<Product>>(`/products/${id}/nkt-register`, { method: 'POST' }),
  importProducts: (file: File) => {
    const form = new FormData()
    form.append('file', file)
    return request<ImportResult>('/products/import', { method: 'POST', body: form })
  },
  importTemplate: () => requestBlob('/products/import/template'),

  orders: (query: { status?: OrderStatus; page?: number }) => request<Paginated<CodeOrder>>('/code-orders', { query }),
  order: (id: number) => request<DataWrap<CodeOrder>>(`/code-orders/${id}`),
  createOrder: (productId: number, quantity: number) =>
    request<DataWrap<CodeOrder>>('/code-orders', { method: 'POST', body: { product_id: productId, quantity } }),
  refreshOrder: (id: number) => request<DataWrap<CodeOrder>>(`/code-orders/${id}/refresh`, { method: 'POST' }),

  codes: (query: {
    status?: CodeStatus
    product_id?: number
    order_id?: number
    search?: string
    page?: number
    per_page?: number
  }) => request<Paginated<Code>>('/codes', { query }),
  codesSummary: () => request<{ data: CodeSummaryRow[] }>('/codes/summary'),

  labelTemplates: () => request<{ data: LabelTemplate[] }>('/label-templates'),
  createLabelTemplate: (input: LabelTemplateInput) =>
    request<DataWrap<LabelTemplate>>('/label-templates', { method: 'POST', body: input }),
  updateLabelTemplate: (id: number, input: LabelTemplateInput) =>
    request<DataWrap<LabelTemplate>>(`/label-templates/${id}`, { method: 'PUT', body: input }),
  deleteLabelTemplate: (id: number) => request<void>(`/label-templates/${id}`, { method: 'DELETE' }),
  labelTemplatePreview: (id: number, productId?: number) =>
    requestBlob(`/label-templates/${id}/preview`, { query: { product_id: productId } }),

  printJobs: (page = 1) => request<Paginated<PrintJob>>('/print-jobs', { query: { page } }),
  printJob: (id: number) => request<DataWrap<PrintJob>>(`/print-jobs/${id}`),
  createPrintJob: (input: PrintJobInput) => request<DataWrap<PrintJob>>('/print-jobs', { method: 'POST', body: input }),
  downloadPrintJob: (id: number) => requestBlob(`/print-jobs/${id}/download`),

  documents: (page = 1) => request<Paginated<IsmptDocument>>('/documents', { query: { page } }),
  document: (id: number) => request<DataWrap<IsmptDocument>>(`/documents/${id}`),
  prepareDocument: (type: DocumentType, selection: DocumentSelection) =>
    request<DataWrap<IsmptDocument>>('/documents/prepare', { method: 'POST', body: { type, ...selection } }),
  submitDocument: (id: number, signature: string) =>
    request<DataWrap<IsmptDocument>>(`/documents/${id}/submit`, { method: 'POST', body: { signature } }),

  operations: (query: { type?: string; level?: OperationLevel; page?: number }) =>
    request<Paginated<Operation>>('/operations', { query }),

  settings: () => request<Settings>('/settings'),
  updateSettings: (input: OrganizationInput) =>
    request<{ organization: Organization }>('/settings', { method: 'PUT', body: input }),
  issueIntegrationToken: () => request<{ token: string }>('/settings/integration-token', { method: 'POST' }),
  createAgent: (name: string) =>
    request<{ data: PrintAgent; token: string }>('/agents', { method: 'POST', body: { name } }),
  deleteAgent: (id: number) => request<void>(`/agents/${id}`, { method: 'DELETE' }),

  integrationCodes: (
    token: string,
    query: { format: 'json' | 'csv'; order?: string; gtin?: string; status?: string; updated_since?: string },
  ) => requestBlob('/integration/v1/codes', { query, token }),
}
