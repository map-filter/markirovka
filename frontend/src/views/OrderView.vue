<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api } from '@/api'
import type { CodeOrder } from '@/api'
import { notify, notifyError } from '@/stores/toast'
import { codeStatuses, codeStatusOrder, orderStatuses } from '@/utils/status'
import { daysUntil, daysWord, formatDate, formatDateTime, formatNumber } from '@/utils/format'
import { errorMessage } from '@/utils/errors'
import { usePoller } from '@/utils/poll'
import Icon from '@/components/Icon.vue'
import StatusBadge from '@/components/StatusBadge.vue'

const route = useRoute()
const router = useRouter()
const id = computed(() => Number(route.params.id))

const order = ref<CodeOrder | null>(null)
const loading = ref(true)
const error = ref('')
const refreshing = ref(false)

const pending = computed(() => order.value?.status === 'sent' || order.value?.status === 'processing' || order.value?.status === 'new')
const days = computed(() => daysUntil(order.value?.expires_at))
const unused = computed(() => (order.value ? order.value.codes.received + order.value.codes.printed : 0))
const printable = computed(() => order.value?.codes.received ?? 0)
const toIntroduce = computed(() => (order.value ? order.value.codes.printed + order.value.codes.applied : 0))

async function load() {
  error.value = ''
  try {
    order.value = (await api.order(id.value)).data
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    loading.value = false
  }
  if (pending.value) poller.ensure()
}

const poller = usePoller(async () => {
  if (!order.value) return false
  const before = order.value.status
  const res = order.value.status === 'new' ? await api.order(id.value) : await api.refreshOrder(id.value)
  order.value = res.data
  if (before !== 'ready' && res.data.status === 'ready') notify(`Заказ ${res.data.number} готов: коды получены`)
  return pending.value
}, 3000)

async function refresh() {
  refreshing.value = true
  try {
    order.value = (await api.refreshOrder(id.value)).data
    if (pending.value) poller.ensure()
  } catch (e) {
    notifyError(e)
  } finally {
    refreshing.value = false
  }
}

onMounted(load)

function goPrint() {
  router.push({ name: 'print', query: { order_id: String(id.value) } })
}

function goCirculation() {
  router.push({ name: 'circulation', query: { order_id: String(id.value) } })
}
</script>

<template>
  <div class="page">
    <div>
      <RouterLink class="btn btn-sm btn-ghost back" :to="{ name: 'orders' }">
        <Icon name="back" :size="15" />
        Все заказы
      </RouterLink>
    </div>
    <div v-if="loading" class="card loading-line">Загружаю заказ...</div>
    <div v-else-if="error" class="alert tone-danger" role="alert">
      <Icon name="alert" :size="16" />
      <span class="break">{{ error }}</span>
    </div>
    <template v-else-if="order">
      <div class="page-head">
        <div>
          <h1 class="row">
            <span>Заказ {{ order.number }}</span>
            <StatusBadge :map="orderStatuses" :value="order.status" />
          </h1>
          <p class="break">{{ order.product.name }} · <span class="mono">{{ order.product.gtin }}</span></p>
        </div>
        <div class="page-actions">
          <button v-if="order.status !== 'ready'" class="btn" type="button" :disabled="refreshing" @click="refresh">
            <span v-if="refreshing" class="spinner" />
            <Icon v-else name="refresh" :size="16" />
            Обновить статус
          </button>
          <button class="btn" type="button" :disabled="order.status !== 'ready' || toIntroduce === 0" @click="goCirculation">
            <Icon name="send" :size="16" />
            Ввести в оборот
          </button>
          <button class="btn btn-primary" type="button" :disabled="order.status !== 'ready' || printable === 0" @click="goPrint">
            <Icon name="printer" :size="16" />
            Напечатать этикетки
          </button>
        </div>
      </div>

      <div v-if="pending" class="alert tone-info">
        <span class="spinner" />
        <span>Заказ обрабатывается в ИС МПТ. Статус обновляется автоматически каждые 3 секунды.</span>
      </div>
      <div v-if="order.status === 'failed'" class="alert tone-danger" role="alert">
        <Icon name="alert" :size="16" />
        <span class="break">ИС МПТ вернула ошибку: {{ order.error || 'без описания' }}</span>
      </div>
      <div v-if="order.status === 'ready' && days !== null && unused > 0" class="alert" :class="days <= 5 ? 'tone-danger' : 'tone-warning'">
        <Icon name="clock" :size="16" />
        <span>
          {{ formatNumber(unused) }} кодов без отчёта о нанесении. Коды аннулируются через {{ Math.max(days, 0) }}
          {{ daysWord(Math.max(days, 0)) }} без отчёта о нанесении ({{ formatDate(order.expires_at) }}).
        </span>
      </div>

      <div class="grid grid-2">
        <section class="card">
          <div class="card-head"><h2>Сведения</h2></div>
          <div class="card-body">
            <dl class="kv">
              <dt>Количество</dt>
              <dd>{{ formatNumber(order.quantity) }}</dd>
              <dt>Создан</dt>
              <dd>{{ formatDateTime(order.created_at) }}</dd>
              <dt>Готов</dt>
              <dd>{{ formatDateTime(order.ready_at) }}</dd>
              <dt>Аннулирование</dt>
              <dd>{{ formatDate(order.expires_at) }}</dd>
              <dt>Номер в ИС МПТ</dt>
              <dd class="mono break">{{ order.ismpt_order_id || '-' }}</dd>
            </dl>
          </div>
        </section>
        <section class="card">
          <div class="card-head">
            <h2>Коды заказа</h2>
            <RouterLink class="btn btn-sm btn-ghost" :to="{ name: 'codes', query: { order_id: String(order.id) } }">
              Открыть список
            </RouterLink>
          </div>
          <div class="counts-list">
            <RouterLink
              v-for="key in codeStatusOrder"
              :key="key"
              class="count-row"
              :to="{ name: 'codes', query: { order_id: String(order.id), status: key } }"
            >
              <span class="badge" :class="`tone-${codeStatuses[key].tone}`">{{ codeStatuses[key].label }}</span>
              <strong>{{ formatNumber(order.codes[key]) }}</strong>
            </RouterLink>
          </div>
        </section>
      </div>
    </template>
  </div>
</template>

<style scoped>
.back {
  margin-left: -8px;
}

.counts-list {
  display: flex;
  flex-direction: column;
}

.count-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 10px 18px;
  border-bottom: 1px solid var(--c-border);
  color: inherit;
  text-decoration: none;
}

.count-row:last-child {
  border-bottom: 0;
}

.count-row:hover {
  background: var(--c-surface-2);
}

h1.row {
  gap: 10px;
}
</style>
