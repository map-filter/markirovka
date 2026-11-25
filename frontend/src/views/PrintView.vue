<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api, saveBlob } from '@/api'
import type { CodeOrder, LabelTemplate, PageMeta, PrintAgent, PrintFormat, PrintJob, PrintJobInput, PrintTarget, Product, ProductShort } from '@/api'
import { notify, notifyError } from '@/stores/toast'
import { printJobStatuses } from '@/utils/status'
import { codesWord, formatDateTime, formatNumber } from '@/utils/format'
import { errorMessage, fieldErrors } from '@/utils/errors'
import { usePoller } from '@/utils/poll'
import Icon from '@/components/Icon.vue'
import StatusBadge from '@/components/StatusBadge.vue'
import Pagination from '@/components/Pagination.vue'
import ProductPicker from '@/components/ProductPicker.vue'

type Source = 'product' | 'order' | 'codes'

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
const source = ref<Source>(codeIds.value.length ? 'codes' : route.query.order_id ? 'order' : 'product')
const productId = ref<number | null>(Number(route.query.product_id) || null)
const orderId = ref<number | null>(Number(route.query.order_id) || null)
const selectedProduct = ref<ProductShort | null>(null)
const product = ref<Product | null>(null)
const quantity = ref<number | null>(null)
const templateId = ref<number | null>(null)
const format = ref<PrintFormat>('pdf')
const target = ref<PrintTarget>('download')
const agentId = ref<number | null>(null)
const reprint = ref(false)

const templates = ref<LabelTemplate[]>([])
const agents = ref<PrintAgent[]>([])
const orders = ref<CodeOrder[]>([])
const available = ref<number | null>(null)
const errors = ref<Record<string, string>>({})
const formError = ref('')
const busy = ref(false)
const lastJob = ref<PrintJob | null>(null)

const jobs = ref<PrintJob[]>([])
const jobsMeta = ref<PageMeta | null>(null)
const jobsPage = ref(1)
const jobsLoading = ref(true)

const currentOrder = computed(() => orders.value.find((o) => o.id === orderId.value) ?? null)
const onlineAgents = computed(() => agents.value.filter((a) => a.online))
const currentAgent = computed(() => agents.value.find((a) => a.id === agentId.value) ?? null)
const template = computed(() => templates.value.find((t) => t.id === templateId.value) ?? null)

async function loadRefs() {
  try {
    const [t, s, o] = await Promise.all([api.labelTemplates(), api.settings(), api.orders({ status: 'ready' })])
    templates.value = t.data
    agents.value = s.agents
    orders.value = o.data
    if (!templateId.value) templateId.value = (t.data.find((x) => x.is_default) ?? t.data[0])?.id ?? null
    if (!agentId.value) agentId.value = (s.agents.find((a) => a.online) ?? s.agents[0])?.id ?? null
    if (orderId.value && !o.data.some((x) => x.id === orderId.value)) {
      try {
        orders.value = [(await api.order(orderId.value)).data, ...orders.value]
      } catch {
        void 0
      }
    }
  } catch (e) {
    notifyError(e)
  }
}

async function loadProduct() {
  if (!productId.value) return
  try {
    product.value = (await api.product(productId.value)).data
    selectedProduct.value = product.value
  } catch {
    void 0
  }
}

async function updateAvailable() {
  available.value = null
  try {
    if (source.value === 'product' && productId.value) {
      const res = await api.codes({ product_id: productId.value, status: reprint.value ? 'printed' : 'received', per_page: 1 })
      available.value = res.meta.total
    } else if (source.value === 'order' && orderId.value) {
      const res = await api.codes({ order_id: orderId.value, status: reprint.value ? 'printed' : 'received', per_page: 1 })
      available.value = res.meta.total
    }
  } catch {
    available.value = null
  }
  if (available.value !== null && (quantity.value === null || quantity.value > available.value)) {
    quantity.value = available.value || null
  }
}

watch([source, productId, orderId, reprint], updateAvailable)

async function loadJobs() {
  try {
    const res = await api.printJobs(jobsPage.value)
    jobs.value = res.data
    jobsMeta.value = res.meta
  } catch (e) {
    notifyError(e)
  } finally {
    jobsLoading.value = false
  }
  if (jobs.value.some((j) => j.status === 'queued' || j.status === 'sent')) jobPoller.ensure()
}

const jobPoller = usePoller(async () => {
  const res = await api.printJobs(jobsPage.value)
  jobs.value = res.data
  jobsMeta.value = res.meta
  return res.data.some((j) => j.status === 'queued' || j.status === 'sent')
}, 3000)

function setJobsPage(p: number) {
  jobsPage.value = p
  loadJobs()
}

onMounted(() => {
  loadRefs()
  loadProduct()
  loadJobs()
  updateAvailable()
})

function sourceLabel(): string {
  if (source.value === 'codes') return `${formatNumber(codeIds.value.length)} ${codesWord(codeIds.value.length)}`
  if (source.value === 'order') return currentOrder.value ? `заказ ${currentOrder.value.number}` : 'заказ'
  return product.value?.name ?? 'товар'
}

async function download(job: PrintJob) {
  try {
    const { blob, filename } = await api.downloadPrintJob(job.id)
    saveBlob(blob, filename ?? `labels-${job.id}.${job.format === 'zpl' ? 'zpl' : 'pdf'}`)
  } catch (e) {
    notifyError(e)
  }
}

async function waitReady(job: PrintJob): Promise<PrintJob> {
  let j = job
  for (let i = 0; i < 60 && j.status !== 'ready' && j.status !== 'failed' && j.status !== 'printed'; i++) {
    await new Promise((r) => setTimeout(r, 1500))
    j = (await api.printJob(j.id)).data
  }
  return j
}

async function submit() {
  errors.value = {}
  formError.value = ''
  if (!templateId.value) errors.value.template_id = 'Выберите шаблон этикетки'
  if (source.value === 'product' && !productId.value) errors.value.product_id = 'Выберите товар'
  if (source.value === 'order' && !orderId.value) errors.value.order_id = 'Выберите заказ'
  if (source.value === 'codes' && !codeIds.value.length) errors.value.code_ids = 'Коды не выбраны'
  if (source.value !== 'codes') {
    const q = Number(quantity.value)
    if (!Number.isInteger(q) || q < 1) errors.value.quantity = 'Укажите количество этикеток'
    else if (available.value !== null && q > available.value) errors.value.quantity = `Доступно только ${formatNumber(available.value)}`
  }
  if (target.value === 'agent' && !agentId.value) errors.value.agent_id = 'Выберите агент печати'
  if (Object.keys(errors.value).length || !templateId.value) return

  const input: PrintJobInput = {
    template_id: templateId.value,
    format: format.value,
    target: target.value,
    agent_id: target.value === 'agent' ? agentId.value : null,
    reprint: reprint.value,
  }
  if (source.value === 'codes') input.code_ids = codeIds.value
  else {
    if (source.value === 'product' && productId.value) input.product_id = productId.value
    if (source.value === 'order' && orderId.value) input.order_id = orderId.value
    input.quantity = Number(quantity.value)
  }

  busy.value = true
  try {
    let job = (await api.createPrintJob(input)).data
    if (job.target === 'download') {
      job = await waitReady(job)
      if (job.status === 'failed') throw new Error(job.error || 'Не удалось сформировать файл')
      await download(job)
      notify(`Файл с ${formatNumber(job.codes_count)} этикетками скачан`)
    } else {
      notify(`Задание на ${formatNumber(job.codes_count)} этикеток передано агенту «${job.agent?.name ?? currentAgent.value?.name ?? ''}»`)
    }
    lastJob.value = job
    jobsPage.value = 1
    await loadJobs()
    updateAvailable()
  } catch (e) {
    errors.value = fieldErrors(e)
    if (!Object.keys(errors.value).length) formError.value = errorMessage(e)
  } finally {
    busy.value = false
  }
}

function clearCodes() {
  codeIds.value = []
  source.value = 'product'
  const { code_ids, ...rest } = route.query
  void code_ids
  router.replace({ query: rest })
}

function goCirculation(job: PrintJob) {
  void job
  if (source.value === 'codes') router.push({ name: 'circulation', query: { code_ids: codeIds.value.join(',') } })
  else if (source.value === 'order' && orderId.value) router.push({ name: 'circulation', query: { order_id: String(orderId.value) } })
  else if (productId.value) router.push({ name: 'circulation', query: { product_id: String(productId.value) } })
  else router.push({ name: 'circulation' })
}
</script>

<template>
  <div class="page">
    <div class="page-head">
      <div>
        <h1>Печать этикеток</h1>
        <p>Выберите шаблон и коды. Файл PDF печатается на любом принтере, ZPL отправляется прямо на термопринтер.</p>
      </div>
    </div>

    <div class="print-grid">
      <section class="card">
        <div class="card-head"><h2>Новая печать</h2></div>
        <form class="card-body stack" novalidate @submit.prevent="submit">
          <div class="field">
            <label for="pr-template">Шаблон этикетки</label>
            <select id="pr-template" v-model="templateId" class="select" :class="{ invalid: errors.template_id }">
              <option v-if="!templates.length" :value="null">Шаблонов нет</option>
              <option v-for="t in templates" :key="t.id" :value="t.id">{{ t.name }} ({{ t.size }} мм)</option>
            </select>
            <span v-if="errors.template_id" class="field-error">{{ errors.template_id }}</span>
            <span v-else-if="!templates.length" class="field-hint">
              Сначала создайте шаблон на странице <RouterLink :to="{ name: 'labels' }">«Этикетки»</RouterLink>.
            </span>
          </div>

          <div class="field">
            <span class="field-label">Какие коды печатать</span>
            <div class="segmented">
              <button type="button" :class="{ active: source === 'product' }" @click="source = 'product'">По товару</button>
              <button type="button" :class="{ active: source === 'order' }" @click="source = 'order'">По заказу</button>
              <button v-if="codeIds.length" type="button" :class="{ active: source === 'codes' }" @click="source = 'codes'">
                Выбранные
              </button>
            </div>
          </div>

          <div v-if="source === 'product'" class="field">
            <label for="pr-product">Товар</label>
            <ProductPicker
              v-model="productId"
              input-id="pr-product"
              :selected="selectedProduct"
              :invalid="!!errors.product_id"
              @pick="(p) => (product = p)"
            />
            <span v-if="errors.product_id" class="field-error">{{ errors.product_id }}</span>
          </div>

          <div v-if="source === 'order'" class="field">
            <label for="pr-order">Заказ кодов</label>
            <select id="pr-order" v-model="orderId" class="select" :class="{ invalid: errors.order_id }">
              <option :value="null">Выберите заказ</option>
              <option v-for="o in orders" :key="o.id" :value="o.id">{{ o.number }} · {{ o.product.name }}</option>
            </select>
            <span v-if="errors.order_id" class="field-error">{{ errors.order_id }}</span>
          </div>

          <div v-if="source === 'codes'" class="alert tone-info">
            <Icon name="info" :size="16" />
            <span class="break">
              Будут напечатаны выбранные на странице «Коды»: {{ formatNumber(codeIds.length) }} {{ codesWord(codeIds.length) }}.
            </span>
            <button class="btn btn-sm btn-ghost" type="button" @click="clearCodes">Сбросить</button>
          </div>
          <span v-if="errors.code_ids" class="field-error">{{ errors.code_ids }}</span>

          <div v-if="source !== 'codes'" class="field">
            <label for="pr-qty">Количество этикеток</label>
            <input
              id="pr-qty"
              v-model.number="quantity"
              class="input qty"
              :class="{ invalid: errors.quantity }"
              type="number"
              min="1"
              :max="available ?? undefined"
              inputmode="numeric"
            />
            <span v-if="errors.quantity" class="field-error">{{ errors.quantity }}</span>
            <span v-else-if="available !== null" class="field-hint">
              {{ reprint ? 'Уже напечатанных' : 'Ненапечатанных' }} кодов: {{ formatNumber(available) }}
            </span>
          </div>

          <div class="field">
            <span class="field-label">Формат</span>
            <div class="segmented">
              <button type="button" :class="{ active: format === 'pdf' }" @click="format = 'pdf'">PDF</button>
              <button type="button" :class="{ active: format === 'zpl' }" @click="format = 'zpl'">ZPL</button>
            </div>
            <span v-if="format === 'zpl'" class="field-hint">
              Для принтеров Zebra и совместимых (Xprinter, Godex в режиме ZPL). Символ GS передаётся как FNC1, сканер не
              потеряет его.
            </span>
            <span v-else class="field-hint">
              Одна этикетка на страницу размера {{ template?.size ?? '58x40' }} мм. Печатайте в масштабе 100%, без «подогнать
              по размеру».
            </span>
          </div>

          <div class="field">
            <span class="field-label">Куда</span>
            <div class="segmented">
              <button type="button" :class="{ active: target === 'download' }" @click="target = 'download'">Скачать файл</button>
              <button type="button" :class="{ active: target === 'agent' }" @click="target = 'agent'">Отправить на агент печати</button>
            </div>
          </div>

          <div v-if="target === 'agent'" class="field">
            <label for="pr-agent">Агент печати</label>
            <select id="pr-agent" v-model="agentId" class="select" :class="{ invalid: errors.agent_id }">
              <option v-if="!agents.length" :value="null">Агентов нет</option>
              <option v-for="a in agents" :key="a.id" :value="a.id">
                {{ a.name }}{{ a.printer ? `, ${a.printer}` : '' }} ({{ a.online ? 'на связи' : 'не в сети' }})
              </option>
            </select>
            <span v-if="errors.agent_id" class="field-error">{{ errors.agent_id }}</span>
            <span v-else-if="!agents.length" class="field-hint">
              Подключите агент на компьютере у принтера: <RouterLink :to="{ name: 'settings' }">Настройки</RouterLink>.
            </span>
            <span v-else-if="currentAgent && !currentAgent.online" class="field-hint warn">
              Агент сейчас не в сети. Задание дождётся его в очереди.
            </span>
            <span v-else-if="onlineAgents.length" class="field-hint ok">Агент на связи</span>
          </div>

          <label class="check">
            <input v-model="reprint" type="checkbox" />
            Повторная печать (уже напечатанные коды, например если этикетка испортилась)
          </label>

          <div v-if="formError" class="alert tone-danger" role="alert">
            <Icon name="alert" :size="16" />
            <span class="break">{{ formError }}</span>
          </div>

          <div class="row">
            <button class="btn btn-primary" type="submit" :disabled="busy">
              <span v-if="busy" class="spinner" />
              <Icon v-else :name="target === 'agent' ? 'send' : 'download'" :size="16" />
              {{ target === 'agent' ? 'Отправить на печать' : 'Сформировать и скачать' }}
            </button>
            <span class="small muted">Печатается: {{ sourceLabel() }}</span>
          </div>

          <div v-if="lastJob" class="alert tone-success">
            <Icon name="check" :size="16" />
            <span class="break">
              Этикетки готовы. После наклейки на изделия отправьте отчёт о нанесении и введите товар в оборот.
            </span>
            <button class="btn btn-sm" type="button" @click="goCirculation(lastJob)">Ввести в оборот</button>
          </div>
        </form>
      </section>

      <section class="card">
        <div class="card-head">
          <h2>История печати</h2>
          <button class="btn btn-sm btn-ghost" type="button" @click="loadJobs">
            <Icon name="refresh" :size="15" />
            Обновить
          </button>
        </div>
        <div v-if="jobsLoading" class="loading-line">Загружаю...</div>
        <div v-else-if="!jobs.length" class="empty">Заданий печати пока нет</div>
        <ul v-else class="jobs">
          <li v-for="j in jobs" :key="j.id" class="job">
            <div class="job-main">
              <div class="row-between">
                <strong>№ {{ j.id }} · {{ formatNumber(j.codes_count) }} шт. · {{ j.format.toUpperCase() }}</strong>
                <StatusBadge :map="printJobStatuses" :value="j.status" />
              </div>
              <div class="m-meta">
                <span>{{ formatDateTime(j.created_at) }}</span>
                <span v-if="j.template">{{ j.template.name }}</span>
                <span v-if="j.target === 'agent'">агент: {{ j.agent?.name ?? '-' }}</span>
              </div>
              <span v-if="j.error" class="small err break">{{ j.error }}</span>
            </div>
            <button
              v-if="j.target === 'download' && j.status !== 'failed'"
              class="btn btn-sm"
              type="button"
              @click="download(j)"
            >
              <Icon name="download" :size="15" />
              Скачать
            </button>
          </li>
        </ul>
        <Pagination :meta="jobsMeta" @change="setJobsPage" />
      </section>
    </div>
  </div>
</template>

<style scoped>
.print-grid {
  display: grid;
  grid-template-columns: minmax(0, 1.1fr) minmax(0, 1fr);
  gap: 16px;
  align-items: start;
}

.qty {
  max-width: 200px;
}

.warn {
  color: var(--c-warning);
}

.ok {
  color: var(--c-success);
}

.err {
  color: var(--c-danger);
}

.jobs {
  list-style: none;
  margin: 0;
  padding: 0;
}

.job {
  display: flex;
  gap: 12px;
  align-items: center;
  padding: 12px 18px;
  border-bottom: 1px solid var(--c-border);
}

.job:last-child {
  border-bottom: 0;
}

.job-main {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 3px;
}

@media (max-width: 1100px) {
  .print-grid {
    grid-template-columns: minmax(0, 1fr);
  }
}

@media (max-width: 520px) {
  .job {
    flex-direction: column;
    align-items: stretch;
    padding: 12px 14px;
  }

  .qty {
    max-width: none;
  }

  .segmented {
    display: flex;
    width: 100%;
  }

  .segmented button {
    flex: 1 1 auto;
  }
}
</style>
