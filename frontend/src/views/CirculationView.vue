<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api } from '@/api'
import type { CodeOrder, CodeStatus, DocumentSelection, DocumentType, IsmptDocument, PageMeta, Product, ProductShort } from '@/api'
import { isIsmptConnected, session } from '@/stores/session'
import { notify } from '@/stores/toast'
import { runDocument, type DocumentStep } from '@/services/documents'
import { documentStatuses, documentTypes } from '@/utils/status'
import { codesWord, formatDateTime, formatNumber } from '@/utils/format'
import { errorMessage } from '@/utils/errors'
import { usePoller } from '@/utils/poll'
import Icon from '@/components/Icon.vue'
import StatusBadge from '@/components/StatusBadge.vue'
import Pagination from '@/components/Pagination.vue'
import ProductPicker from '@/components/ProductPicker.vue'
import IsmptConnect from '@/components/IsmptConnect.vue'

type Source = 'order' | 'product' | 'codes'

interface RunEntry {
  key: number
  type: DocumentType
  step: DocumentStep | 'error'
  document: IsmptDocument | null
  error: string
}

const route = useRoute()
const router = useRouter()

function parseIds(v: unknown): number[] {
  if (typeof v !== 'string') return []
  return v
    .split(',')
    .map((s) => Number(s.trim()))
    .filter((n) => Number.isInteger(n) && n > 0)
}

const codeIds = ref<number[]>(parseIds(route.query.code_ids))
const source = ref<Source>(codeIds.value.length ? 'codes' : route.query.product_id ? 'product' : 'order')
const orderId = ref<number | null>(Number(route.query.order_id) || null)
const productId = ref<number | null>(Number(route.query.product_id) || null)
const selectedProduct = ref<ProductShort | null>(null)
const orders = ref<CodeOrder[]>([])
const includeReceived = ref(false)
const counts = reactive({ received: 0, printed: 0, applied: 0, loading: false })

const running = ref(false)
const runs = ref<RunEntry[]>([])
let runSeq = 0
let unmounted = false

const docs = ref<IsmptDocument[]>([])
const docsMeta = ref<PageMeta | null>(null)
const docsPage = ref(1)
const docsLoading = ref(true)

const connected = computed(() => isIsmptConnected())
const hasSelection = computed(() =>
  source.value === 'codes' ? codeIds.value.length > 0 : source.value === 'order' ? !!orderId.value : !!productId.value,
)
const toApply = computed(() => counts.printed + (includeReceived.value ? counts.received : 0))
const signHint = computed(() =>
  session.config?.sign_mode === 'emulate'
    ? 'Документы подписываются автоматически в режиме эмуляции.'
    : 'Каждый документ подписывается ключом ЭЦП в окне NCALayer.',
)

function filter(): { order_id?: number; product_id?: number } {
  if (source.value === 'order' && orderId.value) return { order_id: orderId.value }
  if (source.value === 'product' && productId.value) return { product_id: productId.value }
  return {}
}

async function countStatus(status: CodeStatus): Promise<number> {
  const res = await api.codes({ ...filter(), status, per_page: 1 })
  return res.meta.total
}

async function loadCounts() {
  if (source.value === 'codes' || !hasSelection.value) {
    counts.received = 0
    counts.printed = 0
    counts.applied = 0
    return
  }
  counts.loading = true
  try {
    const [r, p, a] = await Promise.all([countStatus('received'), countStatus('printed'), countStatus('applied')])
    counts.received = r
    counts.printed = p
    counts.applied = a
  } catch {
    void 0
  } finally {
    counts.loading = false
  }
}

watch([source, orderId, productId], () => {
  includeReceived.value = false
  loadCounts()
})

async function collectIds(statuses: CodeStatus[]): Promise<number[]> {
  const ids: number[] = []
  for (const status of statuses) {
    let page = 1
    for (;;) {
      const res = await api.codes({ ...filter(), status, page, per_page: 500 })
      ids.push(...res.data.map((c) => c.id))
      if (res.meta.current_page >= res.meta.last_page || !res.data.length) break
      page++
    }
  }
  return ids
}

async function selectionFor(type: DocumentType): Promise<DocumentSelection | null> {
  if (source.value === 'codes') return { code_ids: codeIds.value }
  const statuses: CodeStatus[] = type === 'application' ? (includeReceived.value ? ['received', 'printed'] : ['printed']) : ['applied']
  const ids = await collectIds(statuses)
  return ids.length ? { code_ids: ids } : null
}

const stepText: Record<string, string> = {
  prepare: 'Формирую документ',
  sign: 'Подпишите документ в окне ЭЦП',
  submit: 'Отправляю в ИС МПТ',
  wait: 'ИС МПТ проверяет документ',
  done: 'Готово',
  error: 'Ошибка',
}

async function runOne(type: DocumentType): Promise<IsmptDocument | null> {
  const entry = reactive<RunEntry>({ key: ++runSeq, type, step: 'prepare', document: null, error: '' })
  runs.value.unshift(entry)
  try {
    const selection = await selectionFor(type)
    if (!selection) {
      entry.step = 'error'
      entry.error =
        type === 'application'
          ? 'Нет напечатанных кодов для отчёта о нанесении'
          : 'Нет нанесённых кодов: сначала отправьте отчёт о нанесении'
      return null
    }
    const doc = await runDocument(
      type,
      selection,
      (p) => {
        entry.step = p.step
        entry.document = p.document
      },
      () => unmounted,
    )
    entry.document = doc
    return doc
  } catch (e) {
    entry.step = 'error'
    entry.error = errorMessage(e)
    return null
  }
}

async function run(types: DocumentType[]) {
  if (running.value) return
  running.value = true
  try {
    for (const type of types) {
      const doc = await runOne(type)
      if (!doc) break
      if (doc.status === 'accepted') {
        notify(`${documentTypes[type]}: документ принят, ${formatNumber(doc.codes_count)} ${codesWord(doc.codes_count)}`)
      } else if (doc.status === 'rejected' || doc.status === 'failed') {
        notify(`${documentTypes[type]}: ИС МПТ отклонила документ`, 'error')
        break
      } else {
        notify(`${documentTypes[type]}: документ ещё обрабатывается, статус появится в истории`, 'info')
        break
      }
    }
  } finally {
    running.value = false
    loadCounts()
    docsPage.value = 1
    loadDocs()
  }
}

async function loadDocs() {
  try {
    const res = await api.documents(docsPage.value)
    docs.value = res.data
    docsMeta.value = res.meta
  } catch {
    void 0
  } finally {
    docsLoading.value = false
  }
  if (docs.value.some((d) => d.status === 'sent' || d.status === 'processing')) docsPoller.ensure()
}

const docsPoller = usePoller(async () => {
  const res = await api.documents(docsPage.value)
  docs.value = res.data
  docsMeta.value = res.meta
  return res.data.some((d) => d.status === 'sent' || d.status === 'processing')
}, 4000)

function setDocsPage(p: number) {
  docsPage.value = p
  loadDocs()
}

async function loadOrders() {
  try {
    orders.value = (await api.orders({ status: 'ready' })).data
    if (orderId.value && !orders.value.some((o) => o.id === orderId.value)) {
      orders.value = [(await api.order(orderId.value)).data, ...orders.value]
    }
  } catch {
    void 0
  }
}

async function loadProduct() {
  if (!productId.value) return
  try {
    const p = (await api.product(productId.value)).data
    selectedProduct.value = { id: p.id, gtin: p.gtin, name: p.name }
  } catch {
    void 0
  }
}

onMounted(() => {
  loadOrders()
  loadProduct()
  loadCounts()
  loadDocs()
})

onBeforeUnmount(() => {
  unmounted = true
})

function clearCodes() {
  codeIds.value = []
  source.value = 'order'
  const { code_ids, ...rest } = route.query
  void code_ids
  router.replace({ query: rest })
}

function onPickProduct(p: Product | null) {
  selectedProduct.value = p ? { id: p.id, gtin: p.gtin, name: p.name } : null
}

const docErrorsOpen = ref<number | null>(null)
</script>

<template>
  <div class="page">
    <div class="page-head">
      <div>
        <h1>Ввод в оборот</h1>
        <p>
          После наклейки этикеток отправьте в ИС МПТ отчёт о нанесении, затем ввод в оборот. Только после этого товар можно
          продавать.
        </p>
      </div>
    </div>

    <section v-if="!connected" class="card">
      <div class="card-body"><IsmptConnect /></div>
    </section>

    <div class="circ-grid">
      <section class="card">
        <div class="card-head"><h2>Какие коды</h2></div>
        <div class="card-body stack">
          <div class="segmented">
            <button type="button" :class="{ active: source === 'order' }" @click="source = 'order'">По заказу</button>
            <button type="button" :class="{ active: source === 'product' }" @click="source = 'product'">По товару</button>
            <button v-if="codeIds.length" type="button" :class="{ active: source === 'codes' }" @click="source = 'codes'">
              Выбранные
            </button>
          </div>

          <div v-if="source === 'order'" class="field">
            <label for="ci-order">Заказ кодов</label>
            <select id="ci-order" v-model="orderId" class="select">
              <option :value="null">Выберите заказ</option>
              <option v-for="o in orders" :key="o.id" :value="o.id">{{ o.number }} · {{ o.product.name }}</option>
            </select>
          </div>
          <div v-if="source === 'product'" class="field">
            <label for="ci-product">Товар</label>
            <ProductPicker v-model="productId" input-id="ci-product" :selected="selectedProduct" @pick="onPickProduct" />
          </div>
          <div v-if="source === 'codes'" class="alert tone-info">
            <Icon name="info" :size="16" />
            <span class="break">
              Выбрано на странице «Коды»: {{ formatNumber(codeIds.length) }} {{ codesWord(codeIds.length) }}. Для отчёта о
              нанесении подходят напечатанные коды, для ввода в оборот нанесённые.
            </span>
            <button class="btn btn-sm btn-ghost" type="button" @click="clearCodes">Сбросить</button>
          </div>

          <div v-if="source !== 'codes' && hasSelection" class="tiles">
            <div class="tile">
              <span class="badge tone-info">Получен</span>
              <strong>{{ counts.loading ? '...' : formatNumber(counts.received) }}</strong>
              <span class="small muted">не напечатаны</span>
            </div>
            <div class="tile">
              <span class="badge tone-accent">Напечатан</span>
              <strong>{{ counts.loading ? '...' : formatNumber(counts.printed) }}</strong>
              <span class="small muted">ждут отчёта о нанесении</span>
            </div>
            <div class="tile">
              <span class="badge tone-warning">Нанесён</span>
              <strong>{{ counts.loading ? '...' : formatNumber(counts.applied) }}</strong>
              <span class="small muted">ждут ввода в оборот</span>
            </div>
          </div>

          <label v-if="source !== 'codes' && counts.received > 0" class="check">
            <input v-model="includeReceived" type="checkbox" />
            Включить в отчёт о нанесении ещё не напечатанные коды ({{ formatNumber(counts.received) }})
          </label>
          <div v-if="includeReceived" class="alert tone-warning">
            <Icon name="alert" :size="16" />
            <span>
              Отчитывайтесь только о кодах, которые действительно наклеены на изделия. Если этикетка напечатана не через
              кабинет, убедитесь, что она уже на товаре.
            </span>
          </div>

          <div class="divider" />

          <div class="actions">
            <button
              class="btn btn-primary"
              type="button"
              :disabled="running || !hasSelection || !connected"
              @click="run(['application', 'introduction'])"
            >
              <span v-if="running" class="spinner" />
              <Icon v-else name="send" :size="16" />
              Нанести и ввести в оборот
            </button>
            <button
              class="btn"
              type="button"
              :disabled="running || !hasSelection || !connected || (source !== 'codes' && toApply === 0)"
              @click="run(['application'])"
            >
              Отчёт о нанесении
            </button>
            <button
              class="btn"
              type="button"
              :disabled="running || !hasSelection || !connected || (source !== 'codes' && counts.applied === 0)"
              @click="run(['introduction'])"
            >
              Ввод в оборот
            </button>
          </div>
          <p class="small muted">{{ signHint }}</p>
        </div>
      </section>

      <section class="card">
        <div class="card-head"><h2>Ход отправки</h2></div>
        <div v-if="!runs.length" class="empty small">Здесь появится ход подписания и проверки документов</div>
        <ul v-else class="runs">
          <li v-for="r in runs" :key="r.key" class="run">
            <div class="row-between">
              <strong>{{ documentTypes[r.type] }}</strong>
              <StatusBadge v-if="r.document && r.step === 'done'" :map="documentStatuses" :value="r.document.status" />
              <span v-else-if="r.step === 'error'" class="badge tone-danger">Ошибка</span>
              <span v-else class="row small text-2"><span class="spinner" /> {{ stepText[r.step] }}</span>
            </div>
            <div v-if="r.document" class="m-meta">
              <span>Документ № {{ r.document.id }}</span>
              <span>{{ formatNumber(r.document.codes_count) }} {{ codesWord(r.document.codes_count) }}</span>
              <span v-if="r.document.ismpt_document_id" class="mono break">{{ r.document.ismpt_document_id }}</span>
            </div>
            <div v-if="r.error" class="small err break">{{ r.error }}</div>
            <div v-if="r.document?.errors?.length" class="code-errors">
              <div class="small"><strong>Ошибки по кодам: {{ r.document.errors.length }}</strong></div>
              <ul>
                <li v-for="(er, i) in r.document.errors.slice(0, 50)" :key="i">
                  <span class="mono truncate code">{{ er.code }}</span>
                  <span class="break">{{ er.message }}</span>
                </li>
              </ul>
            </div>
          </li>
        </ul>
      </section>
    </div>

    <section class="card">
      <div class="card-head">
        <h2>Документы в ИС МПТ</h2>
        <button class="btn btn-sm btn-ghost" type="button" @click="loadDocs">
          <Icon name="refresh" :size="15" />
          Обновить
        </button>
      </div>
      <div v-if="docsLoading" class="loading-line">Загружаю...</div>
      <div v-else-if="!docs.length" class="empty">Документов пока нет</div>
      <template v-else>
        <div class="table-wrap desktop-table">
          <table class="table">
            <thead>
              <tr>
                <th>№</th>
                <th>Тип</th>
                <th class="num">Кодов</th>
                <th>Статус</th>
                <th>Создан</th>
                <th>Обработан</th>
                <th>Номер в ИС МПТ</th>
              </tr>
            </thead>
            <tbody>
              <template v-for="d in docs" :key="d.id">
                <tr>
                  <td>{{ d.id }}</td>
                  <td>{{ d.type_label || documentTypes[d.type] }}</td>
                  <td class="num">{{ formatNumber(d.codes_count) }}</td>
                  <td>
                    <span class="row nowrap">
                      <StatusBadge :map="documentStatuses" :value="d.status" />
                      <button
                        v-if="d.errors?.length"
                        class="btn btn-sm btn-ghost"
                        type="button"
                        @click="docErrorsOpen = docErrorsOpen === d.id ? null : d.id"
                      >
                        Ошибки: {{ d.errors.length }}
                      </button>
                    </span>
                  </td>
                  <td class="nowrap">{{ formatDateTime(d.created_at) }}</td>
                  <td class="nowrap">{{ formatDateTime(d.processed_at) }}</td>
                  <td class="mono small"><span class="truncate doc-id" :title="d.ismpt_document_id ?? ''">{{ d.ismpt_document_id || '-' }}</span></td>
                </tr>
                <tr v-if="docErrorsOpen === d.id && d.errors?.length">
                  <td colspan="7">
                    <ul class="doc-errors">
                      <li v-for="(er, i) in d.errors" :key="i">
                        <span class="mono truncate code">{{ er.code }}</span>
                        <span class="break">{{ er.message }}</span>
                      </li>
                    </ul>
                  </td>
                </tr>
              </template>
            </tbody>
          </table>
        </div>
        <div class="mobile-cards">
          <div v-for="d in docs" :key="d.id" class="m-card">
            <div class="row-between">
              <strong>{{ d.type_label || documentTypes[d.type] }}</strong>
              <StatusBadge :map="documentStatuses" :value="d.status" />
            </div>
            <div class="m-meta">
              <span>№ {{ d.id }}</span>
              <span>{{ formatNumber(d.codes_count) }} {{ codesWord(d.codes_count) }}</span>
              <span>{{ formatDateTime(d.created_at) }}</span>
            </div>
            <ul v-if="d.errors?.length" class="doc-errors">
              <li v-for="(er, i) in d.errors.slice(0, 10)" :key="i">
                <span class="mono truncate code">{{ er.code }}</span>
                <span class="break">{{ er.message }}</span>
              </li>
            </ul>
          </div>
        </div>
        <Pagination :meta="docsMeta" @change="setDocsPage" />
      </template>
    </section>
  </div>
</template>

<style scoped>
@media (max-width: 1180px) {
  .desktop-table {
    display: none;
  }

  .mobile-cards {
    display: block;
  }
}

.circ-grid {
  display: grid;
  grid-template-columns: minmax(0, 1.1fr) minmax(0, 1fr);
  gap: 16px;
  align-items: start;
}

.tiles {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 8px;
}

.tile {
  display: flex;
  flex-direction: column;
  gap: 4px;
  align-items: flex-start;
  padding: 10px 12px;
  border: 1px solid var(--c-border);
  border-radius: var(--radius-sm);
  background: var(--c-surface-2);
  min-width: 0;
}

.tile strong {
  font-size: 20px;
  font-variant-numeric: tabular-nums;
}

.actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.runs {
  list-style: none;
  margin: 0;
  padding: 0;
}

.run {
  padding: 12px 18px;
  border-bottom: 1px solid var(--c-border);
  display: flex;
  flex-direction: column;
  gap: 5px;
}

.run:last-child {
  border-bottom: 0;
}

.err {
  color: var(--c-danger);
}

.code-errors {
  background: var(--c-danger-soft);
  border-radius: var(--radius-sm);
  padding: 8px 10px;
  max-height: 240px;
  overflow-y: auto;
}

.code-errors ul,
.doc-errors {
  list-style: none;
  margin: 4px 0 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 4px;
  font-size: 13px;
}

.code-errors li,
.doc-errors li {
  display: grid;
  grid-template-columns: minmax(0, 220px) minmax(0, 1fr);
  gap: 8px;
}

.code {
  display: block;
}

.doc-id {
  display: block;
  max-width: 180px;
}

@media (max-width: 1100px) {
  .circ-grid {
    grid-template-columns: minmax(0, 1fr);
  }
}

@media (max-width: 560px) {
  .tiles {
    grid-template-columns: minmax(0, 1fr);
  }

  .tile {
    flex-direction: row;
    align-items: center;
    flex-wrap: wrap;
  }

  .actions .btn {
    flex: 1 1 100%;
  }

  .code-errors li,
  .doc-errors li {
    grid-template-columns: minmax(0, 1fr);
    gap: 0;
  }

  .run {
    padding: 12px 14px;
  }
}
</style>
