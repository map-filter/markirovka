<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter, type LocationQueryRaw } from 'vue-router'
import { api } from '@/api'
import type { Dashboard, TodoItem } from '@/api'
import { setIsmpt } from '@/stores/session'
import { codeStatuses, codeStatusOrder, levelStatuses, severityTone } from '@/utils/status'
import { codesWord, daysWord, formatDate, formatDateTime, formatNumber } from '@/utils/format'
import { errorMessage } from '@/utils/errors'
import Icon from '@/components/Icon.vue'
import IsmptConnect from '@/components/IsmptConnect.vue'
import StatusBadge from '@/components/StatusBadge.vue'

const router = useRouter()
const data = ref<Dashboard | null>(null)
const loading = ref(true)
const error = ref('')

async function load() {
  error.value = ''
  try {
    data.value = await api.dashboard()
    if (data.value.ismpt) setIsmpt(data.value.ismpt)
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    loading.value = false
  }
}

onMounted(load)

const counts = computed(() => data.value?.stats.codes ?? null)
const totalCodes = computed(() => {
  const c = counts.value
  if (!c) return 0
  return c.received + c.printed + c.applied + c.introduced + c.annulled
})

const steps = computed(() => {
  const d = data.value
  const c = counts.value
  const products = d?.stats.products ?? 0
  const nktMissing = d?.todo.some((t) => t.kind === 'nkt_missing') ?? false
  const ordersInProgress = d?.stats.orders_in_progress ?? 0
  const printed = c ? c.printed + c.applied + c.introduced : 0
  const introduced = c?.introduced ?? 0
  const list = [
    {
      title: 'Товар в НКТ',
      hint: 'Карточка с GTIN есть в Национальном каталоге',
      done: totalCodes.value > 0 || (products > 0 && !nktMissing),
      route: 'products',
    },
    {
      title: 'Заказ кодов',
      hint: ordersInProgress > 0 ? `В работе заказов: ${ordersInProgress}` : 'Коды Data Matrix из ИС МПТ',
      done: totalCodes.value > 0,
      route: 'orders',
    },
    { title: 'Печать этикеток', hint: 'PDF или ZPL на термопринтер', done: printed > 0, route: 'print' },
    { title: 'Ввод в оборот', hint: 'Отчёт о нанесении и ввод в оборот', done: introduced > 0, route: 'circulation' },
  ]
  const current = list.findIndex((s) => !s.done)
  return list.map((s, i) => ({ ...s, current: i === current }))
})

function hasRoute(item: TodoItem): boolean {
  return !!item.action?.route && router.hasRoute(item.action.route)
}

function toQuery(q: Record<string, unknown> | undefined): LocationQueryRaw {
  const out: LocationQueryRaw = {}
  if (!q) return out
  for (const [k, v] of Object.entries(q)) {
    if (v === null || v === undefined) continue
    out[k] = Array.isArray(v) ? v.join(',') : String(v)
  }
  return out
}

function runAction(item: TodoItem) {
  if (!item.action || !hasRoute(item)) return
  router.push({ name: item.action.route, query: toQuery(item.action.query) })
}

function goCodes(status: string) {
  router.push({ name: 'codes', query: { status } })
}

const todoIcon: Record<string, string> = {
  info: 'info',
  warning: 'alert',
  danger: 'alert',
}
</script>

<template>
  <div class="page">
    <div class="page-head">
      <div>
        <h1>Что сделать сегодня</h1>
        <p>Здесь собраны дела по маркировке: что заказать, напечатать и отправить в ИС МПТ.</p>
      </div>
      <div class="page-actions">
        <RouterLink class="btn" :to="{ name: 'orders', query: { create: '1' } }">
          <Icon name="plus" :size="16" />
          Заказать коды
        </RouterLink>
        <RouterLink class="btn btn-primary" :to="{ name: 'print' }">
          <Icon name="printer" :size="16" />
          Напечатать этикетки
        </RouterLink>
      </div>
    </div>

    <div v-if="error" class="alert tone-danger" role="alert">
      <Icon name="alert" :size="16" />
      <span class="break">{{ error }}</span>
      <button class="btn btn-sm" type="button" @click="load">Повторить</button>
    </div>

    <section class="card">
      <div class="card-head"><h2>Путь к первой партии</h2></div>
      <ol class="stepper">
        <li v-for="(s, i) in steps" :key="s.title" :class="{ done: s.done, current: s.current }">
          <RouterLink :to="{ name: s.route }" class="step">
            <span class="num">
              <Icon v-if="s.done" name="check" :size="16" />
              <template v-else>{{ i + 1 }}</template>
            </span>
            <span class="step-text">
              <strong>{{ s.title }}</strong>
              <span class="small muted">{{ s.hint }}</span>
            </span>
          </RouterLink>
        </li>
      </ol>
    </section>

    <div class="dash-grid">
      <div class="stack col-main">
        <section class="card">
          <div class="card-head">
            <h2>Дела</h2>
            <span v-if="data" class="muted small">{{ data.todo.length ? `Задач: ${data.todo.length}` : '' }}</span>
          </div>
          <div v-if="loading" class="loading-line">Загружаю...</div>
          <div v-else-if="data && !data.todo.length" class="empty">
            <Icon name="check" :size="28" />
            <p>Срочных дел нет. Всё отправлено и в порядке.</p>
          </div>
          <ul v-else-if="data" class="todo">
            <li v-for="(t, i) in data.todo" :key="`${t.kind}-${i}`" class="todo-item" :class="`sev-${t.severity}`">
              <span class="todo-ico" :class="`tone-${severityTone[t.severity]}`">
                <Icon :name="todoIcon[t.severity] ?? 'info'" :size="16" />
              </span>
              <div class="todo-text">
                <strong class="break">
                  {{ t.title }}
                  <span v-if="t.count" class="badge plain" :class="`tone-${severityTone[t.severity]}`">{{ formatNumber(t.count) }}</span>
                </strong>
                <span v-if="t.description" class="small text-2 break">{{ t.description }}</span>
              </div>
              <button
                v-if="t.action && hasRoute(t)"
                class="btn btn-sm"
                :class="{ 'btn-primary': t.severity === 'danger' || t.kind === 'connect_ismpt' }"
                type="button"
                @click="runAction(t)"
              >
                {{ t.action.label }}
              </button>
            </li>
          </ul>
        </section>

        <section v-if="data && data.expiring.length" class="card">
          <div class="card-head">
            <h2>Коды скоро аннулируются</h2>
          </div>
          <ul class="expiring">
            <li v-for="x in data.expiring" :key="x.order_id">
              <div class="alert" :class="x.days_left <= 3 ? 'tone-danger' : 'tone-warning'">
                <Icon name="clock" :size="16" />
                <div class="exp-text">
                  <strong class="break">{{ x.product.name }}</strong>
                  <span class="small break">
                    Заказ {{ x.order_number }}: {{ formatNumber(x.count) }} {{ codesWord(x.count) }}.
                    Коды аннулируются через {{ x.days_left }} {{ daysWord(x.days_left) }} без отчёта о нанесении
                    ({{ formatDate(x.expires_at) }}).
                  </span>
                </div>
                <RouterLink class="btn btn-sm" :to="{ name: 'circulation', query: { order_id: String(x.order_id) } }">
                  Отчитаться
                </RouterLink>
              </div>
            </li>
          </ul>
        </section>

        <section class="card">
          <div class="card-head">
            <h2>Последние операции</h2>
            <RouterLink class="btn btn-sm btn-ghost" :to="{ name: 'journal' }">Весь журнал</RouterLink>
          </div>
          <div v-if="loading" class="loading-line">Загружаю...</div>
          <div v-else-if="!data?.recent_operations.length" class="empty">Операций пока нет</div>
          <ul v-else class="ops">
            <li v-for="op in data.recent_operations" :key="op.id">
              <span class="op-time small muted nowrap">{{ formatDateTime(op.created_at) }}</span>
              <span class="op-msg break">{{ op.message }}</span>
              <StatusBadge v-if="op.level !== 'info'" :map="levelStatuses" :value="op.level" />
            </li>
          </ul>
        </section>
      </div>

      <div class="stack col-side">
        <section class="card">
          <div class="card-head"><h2>Подключение к ИС МПТ</h2></div>
          <div class="card-body">
            <IsmptConnect @connected="load" />
          </div>
        </section>

        <section class="card">
          <div class="card-head">
            <h2>Коды по статусам</h2>
            <span class="muted small">{{ formatNumber(totalCodes) }}</span>
          </div>
          <div class="counters">
            <button
              v-for="key in codeStatusOrder"
              :key="key"
              type="button"
              class="counter"
              @click="goCodes(key)"
            >
              <span class="badge" :class="`tone-${codeStatuses[key].tone}`">{{ codeStatuses[key].label }}</span>
              <strong>{{ formatNumber(counts?.[key] ?? 0) }}</strong>
            </button>
          </div>
          <div v-if="data" class="card-body side-stats small text-2">
            <span>Товаров: <strong>{{ formatNumber(data.stats.products) }}</strong></span>
            <span>Заказов в работе: <strong>{{ formatNumber(data.stats.orders_in_progress) }}</strong></span>
          </div>
        </section>
      </div>
    </div>
  </div>
</template>

<style scoped>
.stepper {
  list-style: none;
  margin: 0;
  padding: 14px;
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 10px;
}

.stepper li {
  min-width: 0;
}

.step {
  display: flex;
  gap: 10px;
  align-items: flex-start;
  padding: 10px 12px;
  border-radius: var(--radius-sm);
  border: 1px solid var(--c-border);
  text-decoration: none;
  color: inherit;
  height: 100%;
  min-width: 0;
}

.step:hover {
  border-color: var(--c-border-strong);
  background: var(--c-surface-2);
}

.num {
  width: 28px;
  height: 28px;
  flex: none;
  border-radius: 50%;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-weight: 700;
  background: var(--c-neutral-soft);
  color: var(--c-text-2);
  font-size: 14px;
}

.done .num {
  background: var(--c-success-soft);
  color: var(--c-success);
}

.current .step {
  border-color: var(--c-accent);
  background: var(--c-accent-soft);
}

.current .num {
  background: var(--c-accent);
  color: #fff;
}

.step-text {
  display: flex;
  flex-direction: column;
  min-width: 0;
  overflow-wrap: anywhere;
}

.dash-grid {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 360px);
  gap: 18px;
  align-items: start;
}

.todo,
.expiring,
.ops {
  list-style: none;
  margin: 0;
  padding: 0;
}

.todo-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 13px 18px;
  border-bottom: 1px solid var(--c-border);
  border-left: 3px solid transparent;
}

.todo-item:last-child {
  border-bottom: 0;
}

.sev-warning {
  border-left-color: #e3b04b;
}

.sev-danger {
  border-left-color: var(--c-danger);
}

.sev-info {
  border-left-color: #8bb2dc;
}

.todo-ico {
  width: 30px;
  height: 30px;
  border-radius: 50%;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex: none;
}

.todo-text {
  display: flex;
  flex-direction: column;
  gap: 2px;
  flex: 1;
  min-width: 0;
}

.todo-text .badge {
  margin-left: 6px;
  vertical-align: 1px;
}

.expiring {
  padding: 12px 14px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.expiring .alert {
  align-items: center;
}

.exp-text {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 2px;
  color: var(--c-text);
}

.ops li {
  display: flex;
  gap: 12px;
  align-items: baseline;
  padding: 10px 18px;
  border-bottom: 1px solid var(--c-border);
  font-size: 14px;
}

.ops li:last-child {
  border-bottom: 0;
}

.op-msg {
  flex: 1;
  min-width: 0;
}

.counters {
  display: flex;
  flex-direction: column;
}

.counter {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 10px 18px;
  border: 0;
  border-bottom: 1px solid var(--c-border);
  background: none;
  cursor: pointer;
  text-align: left;
}

.counter:hover {
  background: var(--c-surface-2);
}

.counter strong {
  font-size: 17px;
  font-variant-numeric: tabular-nums;
}

.side-stats {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

@media (max-width: 1100px) {
  .dash-grid {
    grid-template-columns: minmax(0, 1fr);
  }

  .col-side {
    order: -1;
  }

  .counters {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(170px, 1fr));
  }
}

@media (max-width: 860px) {
  .stepper {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 560px) {
  .stepper {
    grid-template-columns: minmax(0, 1fr);
    padding: 12px;
  }

  .todo-item {
    flex-wrap: wrap;
    padding: 12px 14px;
  }

  .todo-text {
    flex: 1 1 calc(100% - 42px);
  }

  .todo-item .btn {
    margin-left: 42px;
  }

  .exp-text {
    flex: 1 1 calc(100% - 30px);
  }

  .expiring .btn {
    margin-left: 26px;
  }

  .ops li {
    flex-direction: column;
    gap: 2px;
    padding: 10px 14px;
  }

  .expiring .alert {
    flex-wrap: wrap;
  }
}
</style>
