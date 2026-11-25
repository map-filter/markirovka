<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api } from '@/api'
import type { Code, CodeOrder, CodeStatus, PageMeta, ProductShort } from '@/api'
import { codeStatuses, codeStatusOrder } from '@/utils/status'
import { codesWord, formatDate, formatDateTime, formatNumber } from '@/utils/format'
import { errorMessage } from '@/utils/errors'
import { copyText } from '@/utils/clipboard'
import { notify } from '@/stores/toast'
import Icon from '@/components/Icon.vue'
import StatusBadge from '@/components/StatusBadge.vue'
import Pagination from '@/components/Pagination.vue'
import ProductPicker from '@/components/ProductPicker.vue'

const route = useRoute()
const router = useRouter()

function qNum(v: unknown): number | null {
  const n = Number(v)
  return Number.isInteger(n) && n > 0 ? n : null
}

const filters = reactive({
  status: (typeof route.query.status === 'string' ? route.query.status : '') as CodeStatus | '',
  productId: qNum(route.query.product_id),
  orderId: qNum(route.query.order_id),
  search: typeof route.query.search === 'string' ? route.query.search : '',
})
const page = ref(Number(route.query.page) || 1)
const items = ref<Code[]>([])
const meta = ref<PageMeta | null>(null)
const loading = ref(true)
const error = ref('')
const orders = ref<CodeOrder[]>([])
const selectedProduct = ref<ProductShort | null>(null)
const selected = ref(new Set<number>())

const pageIds = computed(() => items.value.map((c) => c.id))
const allOnPage = computed(() => pageIds.value.length > 0 && pageIds.value.every((id) => selected.value.has(id)))
const selectedCount = computed(() => selected.value.size)

let seq = 0
async function load() {
  const my = ++seq
  loading.value = true
  error.value = ''
  try {
    const res = await api.codes({
      status: filters.status || undefined,
      product_id: filters.productId ?? undefined,
      order_id: filters.orderId ?? undefined,
      search: filters.search.trim() || undefined,
      page: page.value,
      per_page: 50,
    })
    if (my !== seq) return
    items.value = res.data
    meta.value = res.meta
    if (filters.productId && !selectedProduct.value && res.data[0]?.product.id === filters.productId) {
      selectedProduct.value = res.data[0].product
    }
  } catch (e) {
    if (my === seq) error.value = errorMessage(e)
  } finally {
    if (my === seq) loading.value = false
  }
}

async function loadOrders() {
  try {
    orders.value = (await api.orders({ page: 1 })).data
  } catch {
    orders.value = []
  }
}

async function loadSelectedProduct() {
  if (!filters.productId) return
  try {
    const p = (await api.product(filters.productId)).data
    selectedProduct.value = { id: p.id, gtin: p.gtin, name: p.name }
  } catch {
    void 0
  }
}

function syncQuery() {
  const q: Record<string, string> = {}
  if (filters.status) q.status = filters.status
  if (filters.productId) q.product_id = String(filters.productId)
  if (filters.orderId) q.order_id = String(filters.orderId)
  if (filters.search.trim()) q.search = filters.search.trim()
  if (page.value > 1) q.page = String(page.value)
  router.replace({ query: q })
}

let timer: ReturnType<typeof setTimeout> | null = null
watch(
  () => filters.search,
  () => {
    if (timer) clearTimeout(timer)
    timer = setTimeout(() => {
      page.value = 1
      syncQuery()
      load()
    }, 300)
  },
)
watch(
  () => [filters.status, filters.productId, filters.orderId],
  () => {
    page.value = 1
    syncQuery()
    load()
  },
)

function setPage(p: number) {
  page.value = p
  syncQuery()
  load()
}

onMounted(() => {
  load()
  loadOrders()
  loadSelectedProduct()
})

function toggle(id: number) {
  const s = new Set(selected.value)
  if (s.has(id)) s.delete(id)
  else s.add(id)
  selected.value = s
}

function toggleAll() {
  const s = new Set(selected.value)
  if (allOnPage.value) pageIds.value.forEach((id) => s.delete(id))
  else pageIds.value.forEach((id) => s.add(id))
  selected.value = s
}

function clearSelection() {
  selected.value = new Set()
}

function resetFilters() {
  filters.status = ''
  filters.productId = null
  filters.orderId = null
  filters.search = ''
  selectedProduct.value = null
}

const idsParam = computed(() => Array.from(selected.value).join(','))

function printSelected() {
  router.push({ name: 'print', query: { code_ids: idsParam.value } })
}

function introduceSelected() {
  router.push({ name: 'circulation', query: { code_ids: idsParam.value } })
}

async function copyCode(c: Code) {
  if (await copyText(c.code_hr)) notify('Код скопирован')
}

const hasFilters = computed(() => !!(filters.status || filters.productId || filters.orderId || filters.search))
</script>

<template>
  <div class="page">
    <div class="page-head">
      <div>
        <h1>Коды маркировки</h1>
        <p>Все полученные из ИС МПТ коды Data Matrix и их статусы. Отметьте коды, чтобы напечатать или ввести их в оборот.</p>
      </div>
    </div>

    <section class="card">
      <div class="card-body">
        <div class="filters">
          <div class="field">
            <label for="c-status">Статус</label>
            <select id="c-status" v-model="filters.status" class="select">
              <option value="">Все статусы</option>
              <option v-for="s in codeStatusOrder" :key="s" :value="s">{{ codeStatuses[s].label }}</option>
            </select>
          </div>
          <div class="field grow">
            <label for="c-product">Товар</label>
            <ProductPicker
              v-model="filters.productId"
              input-id="c-product"
              :selected="selectedProduct"
              placeholder="Все товары"
              @pick="(p) => (selectedProduct = p ? { id: p.id, gtin: p.gtin, name: p.name } : null)"
            />
          </div>
          <div class="field">
            <label for="c-order">Заказ</label>
            <select id="c-order" v-model="filters.orderId" class="select">
              <option :value="null">Все заказы</option>
              <option v-if="filters.orderId && !orders.some((o) => o.id === filters.orderId)" :value="filters.orderId">
                Заказ #{{ filters.orderId }}
              </option>
              <option v-for="o in orders" :key="o.id" :value="o.id">{{ o.number }} · {{ o.product.name }}</option>
            </select>
          </div>
          <div class="field">
            <label for="c-search">Поиск</label>
            <input id="c-search" v-model="filters.search" class="input" type="search" placeholder="Серийный номер или GTIN" />
          </div>
          <button v-if="hasFilters" class="btn btn-ghost" type="button" @click="resetFilters">Сбросить</button>
        </div>
      </div>

      <div v-if="selectedCount" class="selection-bar">
        <span>
          Выбрано: <strong>{{ formatNumber(selectedCount) }}</strong> {{ codesWord(selectedCount) }}
        </span>
        <div class="row">
          <button class="btn btn-sm btn-ghost" type="button" @click="clearSelection">Снять выбор</button>
          <button class="btn btn-sm" type="button" @click="introduceSelected">
            <Icon name="send" :size="15" />
            Ввести в оборот выбранные
          </button>
          <button class="btn btn-sm btn-primary" type="button" @click="printSelected">
            <Icon name="printer" :size="15" />
            Печать выбранных
          </button>
        </div>
      </div>

      <div v-if="error" class="card-body">
        <div class="alert tone-danger" role="alert">
          <Icon name="alert" :size="16" />
          <span class="break">{{ error }}</span>
        </div>
      </div>
      <div v-else-if="loading && !items.length" class="loading-line">Загружаю коды...</div>
      <div v-else-if="!items.length" class="empty">
        <p>{{ hasFilters ? 'По фильтру кодов нет' : 'Кодов пока нет. Закажите коды на странице «Заказы кодов».' }}</p>
        <RouterLink v-if="!hasFilters" class="btn btn-primary" :to="{ name: 'orders', query: { create: '1' } }">
          Заказать коды
        </RouterLink>
      </div>
      <template v-else>
        <div class="table-wrap desktop-table">
          <table class="table">
            <thead>
              <tr>
                <th class="check-col">
                  <input type="checkbox" class="cb" :checked="allOnPage" aria-label="Выбрать все на странице" @change="toggleAll" />
                </th>
                <th>Код</th>
                <th>Товар</th>
                <th>Заказ</th>
                <th>Статус</th>
                <th>Аннулирование</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="c in items" :key="c.id" :class="{ sel: selected.has(c.id) }">
                <td class="check-col">
                  <input type="checkbox" class="cb" :checked="selected.has(c.id)" :aria-label="`Выбрать ${c.serial}`" @change="toggle(c.id)" />
                </td>
                <td class="code-cell">
                  <button class="code-hr mono" type="button" :title="`${c.code_hr}\nНажмите, чтобы скопировать`" @click="copyCode(c)">
                    {{ c.code_hr }}
                  </button>
                </td>
                <td>
                  <div class="cell-title">{{ c.product.name }}</div>
                  <div class="cell-sub mono">{{ c.gtin }}</div>
                </td>
                <td class="nowrap">
                  <RouterLink :to="{ name: 'order', params: { id: String(c.order_id) } }">{{ c.order_number }}</RouterLink>
                </td>
                <td>
                  <StatusBadge :map="codeStatuses" :value="c.status" />
                  <div v-if="c.introduced_at" class="cell-sub nowrap">{{ formatDateTime(c.introduced_at) }}</div>
                  <div v-else-if="c.applied_at" class="cell-sub nowrap">{{ formatDateTime(c.applied_at) }}</div>
                  <div v-else-if="c.printed_at" class="cell-sub nowrap">{{ formatDateTime(c.printed_at) }}</div>
                </td>
                <td class="nowrap">{{ c.status === 'received' || c.status === 'printed' ? formatDate(c.expires_at) : '-' }}</td>
              </tr>
            </tbody>
          </table>
        </div>
        <div class="mobile-cards">
          <div class="m-card m-head">
            <label class="check">
              <input type="checkbox" :checked="allOnPage" @change="toggleAll" />
              Выбрать все на странице
            </label>
          </div>
          <div v-for="c in items" :key="c.id" class="m-card" :class="{ sel: selected.has(c.id) }">
            <div class="m-row">
              <input type="checkbox" class="cb" :checked="selected.has(c.id)" :aria-label="`Выбрать ${c.serial}`" @change="toggle(c.id)" />
              <div class="m-main">
                <div class="row-between">
                  <span class="truncate mono small code-mobile" @click="copyCode(c)">{{ c.code_hr }}</span>
                  <StatusBadge :map="codeStatuses" :value="c.status" />
                </div>
                <span class="break small">{{ c.product.name }}</span>
                <div class="m-meta">
                  <span class="mono">(21){{ c.serial }}</span>
                  <span>{{ c.order_number }}</span>
                  <span v-if="c.status === 'received' || c.status === 'printed'">до {{ formatDate(c.expires_at) }}</span>
                </div>
              </div>
            </div>
          </div>
        </div>
        <Pagination :meta="meta" @change="setPage" />
      </template>
    </section>
  </div>
</template>

<style scoped>
.selection-bar {
  position: sticky;
  top: var(--header-h);
  z-index: 20;
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 8px 12px;
  padding: 10px 18px;
  background: var(--c-accent-soft);
  border-top: 1px solid #c4dedb;
  border-bottom: 1px solid #c4dedb;
}

.cb {
  width: 17px;
  height: 17px;
  accent-color: var(--c-accent);
  margin: 0;
  flex: none;
}

.code-cell {
  max-width: 340px;
}

.code-hr {
  display: block;
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  border: 0;
  background: none;
  padding: 0;
  cursor: copy;
  text-align: left;
  font-size: 12.5px;
  color: var(--c-text);
}

.code-hr:hover {
  color: var(--c-accent);
}

tr.sel td,
.m-card.sel {
  background: #f1f8f7;
}

.m-head {
  background: var(--c-surface-2);
}

.m-row {
  display: flex;
  gap: 10px;
  align-items: flex-start;
  min-width: 0;
}

.m-row .cb {
  margin-top: 2px;
}

.m-main {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.code-mobile {
  flex: 1 1 0;
}

@media (max-width: 900px) {
  .selection-bar {
    top: var(--header-h);
  }
}

@media (max-width: 700px) {
  .selection-bar {
    padding: 10px 14px;
  }

  .selection-bar .row {
    width: 100%;
  }

  .selection-bar .row .btn {
    flex: 1 1 auto;
  }
}
</style>
