<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api, ApiError } from '@/api'
import type { PageMeta, Product } from '@/api'
import { session } from '@/stores/session'
import { notify, notifyError } from '@/stores/toast'
import { nktStatuses } from '@/utils/status'
import { formatDateTime } from '@/utils/format'
import { errorMessage } from '@/utils/errors'
import Icon from '@/components/Icon.vue'
import StatusBadge from '@/components/StatusBadge.vue'
import CodeCountsInline from '@/components/CodeCountsInline.vue'
import Pagination from '@/components/Pagination.vue'
import ProductFormModal from '@/components/ProductFormModal.vue'
import ImportModal from '@/components/ImportModal.vue'

const route = useRoute()
const router = useRouter()

const items = ref<Product[]>([])
const meta = ref<PageMeta | null>(null)
const categories = ref<string[]>([])
const loading = ref(true)
const error = ref('')
const search = ref(typeof route.query.search === 'string' ? route.query.search : '')
const category = ref(typeof route.query.category === 'string' ? route.query.category : '')
const page = ref(Number(route.query.page) || 1)
const editing = ref<Product | null>(null)
const formOpen = ref(false)
const importOpen = ref(false)
const busyId = ref<number | null>(null)

const canRegister = computed(() => session.config?.ismpt_mode === 'emulator')

let seq = 0
async function load() {
  const my = ++seq
  loading.value = true
  error.value = ''
  try {
    const res = await api.products({
      search: search.value.trim() || undefined,
      category: category.value || undefined,
      page: page.value,
      per_page: 25,
    })
    if (my !== seq) return
    items.value = res.data
    meta.value = res.meta
  } catch (e) {
    if (my === seq) error.value = errorMessage(e)
  } finally {
    if (my === seq) loading.value = false
  }
}

async function loadCategories() {
  try {
    categories.value = (await api.productCategories()).data
  } catch {
    categories.value = []
  }
}

function syncQuery() {
  router.replace({
    query: {
      ...(search.value.trim() ? { search: search.value.trim() } : {}),
      ...(category.value ? { category: category.value } : {}),
      ...(page.value > 1 ? { page: String(page.value) } : {}),
    },
  })
}

let timer: ReturnType<typeof setTimeout> | null = null
watch(search, () => {
  if (timer) clearTimeout(timer)
  timer = setTimeout(() => {
    page.value = 1
    syncQuery()
    load()
  }, 300)
})
watch(category, () => {
  page.value = 1
  syncQuery()
  load()
})

function setPage(p: number) {
  page.value = p
  syncQuery()
  load()
}

onMounted(() => {
  load()
  loadCategories()
  if (route.query.create === '1') openCreate()
})

function openCreate() {
  editing.value = null
  formOpen.value = true
}

function openEdit(p: Product) {
  editing.value = p
  formOpen.value = true
}

function onSaved(p: Product) {
  const wasEdit = !!editing.value
  formOpen.value = false
  notify(wasEdit ? 'Товар сохранён' : 'Товар добавлен')
  const index = items.value.findIndex((x) => x.id === p.id)
  if (index >= 0) items.value[index] = p
  else load()
  loadCategories()
}

function replace(p: Product) {
  const index = items.value.findIndex((x) => x.id === p.id)
  if (index >= 0) items.value[index] = p
}

async function checkNkt(p: Product) {
  busyId.value = p.id
  try {
    const res = await api.nktCheck(p.id)
    replace(res.data)
    if (res.data.nkt_status === 'found') notify('Карточка найдена в НКТ')
    else notify('Карточка в НКТ не найдена. Зарегистрируйте товар в НКТ по этому GTIN', 'info', 7000)
  } catch (e) {
    notifyError(e)
  } finally {
    busyId.value = null
  }
}

async function registerNkt(p: Product) {
  busyId.value = p.id
  try {
    const res = await api.nktRegister(p.id)
    replace(res.data)
    notify('Карточка зарегистрирована в НКТ эмулятора')
  } catch (e) {
    notifyError(e)
  } finally {
    busyId.value = null
  }
}

function totalCodes(p: Product): number {
  const c = p.codes
  return c ? c.received + c.printed + c.applied + c.introduced + c.annulled : 0
}

async function remove(p: Product) {
  if (!window.confirm(`Удалить товар «${p.name}»?`)) return
  busyId.value = p.id
  try {
    await api.deleteProduct(p.id)
    notify('Товар удалён')
    load()
  } catch (e) {
    if (e instanceof ApiError && e.status === 409) notify('У товара есть коды, удалить его нельзя', 'error')
    else notifyError(e)
  } finally {
    busyId.value = null
  }
}

function orderCodes(p: Product) {
  router.push({ name: 'orders', query: { create: '1', product_id: String(p.id) } })
}
</script>

<template>
  <div class="page">
    <div class="page-head">
      <div>
        <h1>Товары</h1>
        <p>Карточки изделий с GTIN из Национального каталога. Коды заказываются только на товары, найденные в НКТ.</p>
      </div>
      <div class="page-actions">
        <button class="btn" type="button" @click="importOpen = true">
          <Icon name="upload" :size="16" />
          Импорт из Excel
        </button>
        <button class="btn btn-primary" type="button" @click="openCreate">
          <Icon name="plus" :size="16" />
          Добавить товар
        </button>
      </div>
    </div>

    <section class="card">
      <div class="card-body">
        <div class="filters">
          <div class="field grow">
            <label for="f-search">Поиск</label>
            <input id="f-search" v-model="search" class="input" type="search" placeholder="Наименование, модель или GTIN" />
          </div>
          <div class="field">
            <label for="f-cat">Категория</label>
            <select id="f-cat" v-model="category" class="select">
              <option value="">Все категории</option>
              <option v-for="c in categories" :key="c" :value="c">{{ c }}</option>
            </select>
          </div>
        </div>
      </div>

      <div v-if="error" class="card-body">
        <div class="alert tone-danger" role="alert">
          <Icon name="alert" :size="16" />
          <span class="break">{{ error }}</span>
        </div>
      </div>
      <div v-else-if="loading && !items.length" class="loading-line">Загружаю товары...</div>
      <div v-else-if="!items.length" class="empty">
        <p v-if="search || category">По фильтру ничего не найдено</p>
        <template v-else>
          <p>Товаров пока нет. Добавьте первый товар или загрузите список из Excel.</p>
          <button class="btn btn-primary" type="button" @click="openCreate">Добавить товар</button>
        </template>
      </div>
      <template v-else>
        <div class="table-wrap desktop-table">
          <table class="table">
            <thead>
              <tr>
                <th>Товар</th>
                <th>GTIN</th>
                <th>Категория</th>
                <th>НКТ</th>
                <th>Коды</th>
                <th class="actions">Действия</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="p in items" :key="p.id">
                <td>
                  <div class="cell-title clickable" @click="openEdit(p)">{{ p.name }}</div>
                  <div class="cell-sub">{{ [p.model, p.size && `р. ${p.size}`, p.color].filter(Boolean).join(' · ') }}</div>
                </td>
                <td class="mono nowrap">{{ p.gtin }}</td>
                <td class="break">{{ p.category || '-' }}</td>
                <td>
                  <StatusBadge :map="nktStatuses" :value="p.nkt_status" />
                  <div v-if="p.nkt_checked_at" class="cell-sub nowrap">{{ formatDateTime(p.nkt_checked_at) }}</div>
                </td>
                <td><CodeCountsInline :counts="p.codes" /></td>
                <td class="actions">
                  <div class="row-actions">
                    <button
                      v-if="p.nkt_status === 'not_found' && canRegister"
                      class="btn btn-sm"
                      type="button"
                      :disabled="busyId === p.id"
                      @click="registerNkt(p)"
                    >
                      Зарегистрировать в НКТ
                    </button>
                    <button
                      v-else-if="p.nkt_status !== 'found'"
                      class="btn btn-sm"
                      type="button"
                      :disabled="busyId === p.id"
                      @click="checkNkt(p)"
                    >
                      Проверить в НКТ
                    </button>
                    <button v-else class="btn btn-sm" type="button" @click="orderCodes(p)">Заказать коды</button>
                    <button class="btn btn-sm btn-ghost btn-icon" type="button" title="Изменить" aria-label="Изменить" @click="openEdit(p)">
                      <Icon name="edit" :size="15" />
                    </button>
                    <button
                      v-if="p.nkt_status === 'found'"
                      class="btn btn-sm btn-ghost btn-icon"
                      type="button"
                      title="Проверить в НКТ ещё раз"
                      aria-label="Проверить в НКТ"
                      :disabled="busyId === p.id"
                      @click="checkNkt(p)"
                    >
                      <Icon name="refresh" :size="15" />
                    </button>
                    <button
                      v-if="totalCodes(p) === 0"
                      class="btn btn-sm btn-ghost btn-icon"
                      type="button"
                      title="Удалить"
                      aria-label="Удалить"
                      :disabled="busyId === p.id"
                      @click="remove(p)"
                    >
                      <Icon name="trash" :size="15" />
                    </button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>

        <div class="mobile-cards">
          <div v-for="p in items" :key="p.id" class="m-card">
            <div class="row-between">
              <strong class="break" @click="openEdit(p)">{{ p.name }}</strong>
              <StatusBadge :map="nktStatuses" :value="p.nkt_status" />
            </div>
            <div class="m-meta">
              <span class="mono">{{ p.gtin }}</span>
              <span v-if="p.size">р. {{ p.size }}</span>
              <span v-if="p.color">{{ p.color }}</span>
              <span v-if="p.category">{{ p.category }}</span>
            </div>
            <CodeCountsInline :counts="p.codes" />
            <div class="row">
              <button
                v-if="p.nkt_status === 'not_found' && canRegister"
                class="btn btn-sm"
                type="button"
                :disabled="busyId === p.id"
                @click="registerNkt(p)"
              >
                Зарегистрировать в НКТ
              </button>
              <button
                v-else-if="p.nkt_status !== 'found'"
                class="btn btn-sm"
                type="button"
                :disabled="busyId === p.id"
                @click="checkNkt(p)"
              >
                Проверить в НКТ
              </button>
              <button v-else class="btn btn-sm" type="button" @click="orderCodes(p)">Заказать коды</button>
              <button class="btn btn-sm" type="button" @click="openEdit(p)">Изменить</button>
            </div>
          </div>
        </div>
        <Pagination :meta="meta" @change="setPage" />
      </template>
    </section>

    <ProductFormModal
      v-if="formOpen"
      :product="editing"
      :categories="categories"
      @close="formOpen = false"
      @saved="onSaved"
    />
    <ImportModal
      v-if="importOpen"
      @close="importOpen = false"
      @imported="
        () => {
          load()
          loadCategories()
        }
      "
    />
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

.row-actions {
  display: inline-flex;
  gap: 4px;
  align-items: center;
  justify-content: flex-end;
}
</style>
