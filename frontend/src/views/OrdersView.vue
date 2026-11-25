<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api } from '@/api'
import type { CodeOrder, OrderStatus, PageMeta } from '@/api'
import { notify } from '@/stores/toast'
import { orderStatuses } from '@/utils/status'
import { formatDate, formatDateTime, formatNumber } from '@/utils/format'
import { errorMessage } from '@/utils/errors'
import { usePoller } from '@/utils/poll'
import Icon from '@/components/Icon.vue'
import StatusBadge from '@/components/StatusBadge.vue'
import CodeCountsInline from '@/components/CodeCountsInline.vue'
import Pagination from '@/components/Pagination.vue'
import OrderCreateModal from '@/components/OrderCreateModal.vue'

const route = useRoute()
const router = useRouter()

const items = ref<CodeOrder[]>([])
const meta = ref<PageMeta | null>(null)
const loading = ref(true)
const error = ref('')
const status = ref<OrderStatus | ''>((typeof route.query.status === 'string' ? route.query.status : '') as OrderStatus | '')
const page = ref(Number(route.query.page) || 1)
const createOpen = ref(false)
const createProductId = ref<number | null>(null)

const statusOptions: OrderStatus[] = ['new', 'sent', 'processing', 'ready', 'failed']

function isPending(o: CodeOrder): boolean {
  return o.status === 'sent' || o.status === 'processing' || o.status === 'new'
}

async function load() {
  error.value = ''
  try {
    const res = await api.orders({ status: status.value || undefined, page: page.value })
    items.value = res.data
    meta.value = res.meta
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    loading.value = false
  }
  if (items.value.some(isPending)) poller.ensure()
}

const poller = usePoller(async () => {
  const pending = items.value.filter((o) => o.status === 'sent' || o.status === 'processing')
  if (!pending.length) {
    if (items.value.some((o) => o.status === 'new')) {
      await load()
      return items.value.some(isPending)
    }
    return false
  }
  await Promise.all(
    pending.map(async (o) => {
      try {
        const res = await api.refreshOrder(o.id)
        const index = items.value.findIndex((x) => x.id === o.id)
        if (index >= 0) {
          const before = items.value[index].status
          items.value[index] = res.data
          if (before !== 'ready' && res.data.status === 'ready') notify(`Заказ ${res.data.number} готов: коды получены`)
          if (before !== 'failed' && res.data.status === 'failed') notify(`Заказ ${res.data.number}: ошибка ИС МПТ`, 'error')
        }
      } catch {
        void 0
      }
    }),
  )
  return items.value.some(isPending)
}, 3000)

watch(status, () => {
  page.value = 1
  router.replace({ query: status.value ? { status: status.value } : {} })
  load()
})

function setPage(p: number) {
  page.value = p
  router.replace({ query: { ...route.query, page: String(p) } })
  load()
}

onMounted(() => {
  load()
  if (route.query.create === '1') {
    createProductId.value = Number(route.query.product_id) || null
    createOpen.value = true
  }
})

function openCreate() {
  createProductId.value = null
  createOpen.value = true
}

function closeCreate() {
  createOpen.value = false
  if (route.query.create) {
    const { create, product_id, ...rest } = route.query
    void create
    void product_id
    router.replace({ query: rest })
  }
}

function onCreated(o: CodeOrder) {
  closeCreate()
  notify(`Заказ ${o.number} отправлен в ИС МПТ`)
  if (status.value && status.value !== o.status) status.value = ''
  else load()
}

function open(o: CodeOrder) {
  router.push({ name: 'order', params: { id: String(o.id) } })
}
</script>

<template>
  <div class="page">
    <div class="page-head">
      <div>
        <h1>Заказы кодов</h1>
        <p>Коды Data Matrix заказываются в ИС МПТ на конкретный товар. Обычно заказ готов за несколько минут.</p>
      </div>
      <div class="page-actions">
        <button class="btn btn-primary" type="button" @click="openCreate">
          <Icon name="plus" :size="16" />
          Заказать коды
        </button>
      </div>
    </div>

    <section class="card">
      <div class="card-body">
        <div class="filters">
          <div class="field">
            <label for="o-status">Статус</label>
            <select id="o-status" v-model="status" class="select">
              <option value="">Все заказы</option>
              <option v-for="s in statusOptions" :key="s" :value="s">{{ orderStatuses[s].label }}</option>
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
      <div v-else-if="loading" class="loading-line">Загружаю заказы...</div>
      <div v-else-if="!items.length" class="empty">
        <p>{{ status ? 'Заказов с таким статусом нет' : 'Заказов пока нет' }}</p>
        <button v-if="!status" class="btn btn-primary" type="button" @click="openCreate">Заказать коды</button>
      </div>
      <template v-else>
        <div class="table-wrap desktop-table">
          <table class="table">
            <thead>
              <tr>
                <th>Номер</th>
                <th>Товар</th>
                <th class="num">Кол-во</th>
                <th>Статус</th>
                <th>Коды</th>
                <th>Создан</th>
                <th>Аннулирование</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="o in items" :key="o.id" class="clickable" @click="open(o)">
                <td class="nowrap">
                  <RouterLink :to="{ name: 'order', params: { id: String(o.id) } }" @click.stop>{{ o.number }}</RouterLink>
                </td>
                <td>
                  <div class="cell-title">{{ o.product.name }}</div>
                  <div class="cell-sub mono">{{ o.product.gtin }}</div>
                </td>
                <td class="num">{{ formatNumber(o.quantity) }}</td>
                <td>
                  <span class="row nowrap">
                    <StatusBadge :map="orderStatuses" :value="o.status" />
                    <span v-if="o.status === 'sent' || o.status === 'processing'" class="spinner small-spin" />
                  </span>
                  <div v-if="o.error" class="cell-sub err">{{ o.error }}</div>
                </td>
                <td><CodeCountsInline :counts="o.codes" /></td>
                <td class="nowrap">{{ formatDateTime(o.created_at) }}</td>
                <td class="nowrap">{{ formatDate(o.expires_at) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
        <div class="mobile-cards">
          <RouterLink
            v-for="o in items"
            :key="o.id"
            :to="{ name: 'order', params: { id: String(o.id) } }"
            class="m-card plain-link"
          >
            <div class="row-between">
              <strong>{{ o.number }}</strong>
              <StatusBadge :map="orderStatuses" :value="o.status" />
            </div>
            <span class="break">{{ o.product.name }}</span>
            <div class="m-meta">
              <span>{{ formatNumber(o.quantity) }} шт.</span>
              <span>{{ formatDateTime(o.created_at) }}</span>
              <span v-if="o.expires_at">аннулирование {{ formatDate(o.expires_at) }}</span>
            </div>
            <span v-if="o.error" class="small err break">{{ o.error }}</span>
          </RouterLink>
        </div>
        <Pagination :meta="meta" @change="setPage" />
      </template>
    </section>

    <OrderCreateModal v-if="createOpen" :product-id="createProductId" @close="closeCreate" @created="onCreated" />
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

.err {
  color: var(--c-danger);
}

.small-spin {
  width: 12px;
  height: 12px;
  color: var(--c-muted);
}

.plain-link {
  color: inherit;
  text-decoration: none;
}
</style>
