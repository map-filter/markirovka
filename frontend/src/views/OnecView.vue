<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { api, ApiError, saveBlob } from '@/api'
import type { CodeOrder } from '@/api'
import { codeStatuses, codeStatusOrder } from '@/utils/status'
import { errorMessage } from '@/utils/errors'
import { notify } from '@/stores/toast'
import Icon from '@/components/Icon.vue'
import CopyBlock from '@/components/CopyBlock.vue'

const TOKEN_KEY = 'markirovka.integration-token'

function readToken(): string {
  try {
    return sessionStorage.getItem(TOKEN_KEY) ?? ''
  } catch {
    return ''
  }
}

function writeToken(value: string) {
  try {
    if (value) sessionStorage.setItem(TOKEN_KEY, value)
    else sessionStorage.removeItem(TOKEN_KEY)
  } catch {
    void 0
  }
}

const origin = window.location.origin
const orders = ref<CodeOrder[]>([])
const orderNumber = ref('')
const status = ref('')
const token = ref(readToken())
const tokenInput = ref('')
const askToken = ref(false)
const pendingFormat = ref<'csv' | 'json' | null>(null)
const busy = ref(false)
const error = ref('')
const tokenSet = ref<boolean | null>(null)

const query = computed(() => {
  const p = new URLSearchParams()
  p.set('format', 'csv')
  if (orderNumber.value) p.set('order', orderNumber.value)
  if (status.value) p.set('status', status.value)
  return p
})

const url = (format: 'csv' | 'json') => {
  const p = new URLSearchParams(query.value)
  p.set('format', format)
  return `${origin}/api/integration/v1/codes?${p.toString()}`
}

const curlCsv = computed(() => `curl -H "Authorization: Bearer <токен 1С>" \\\n  "${url('csv')}" -o codes.csv`)
const curlJson = computed(() => `curl -H "Authorization: Bearer <токен 1С>" \\\n  "${url('json')}"`)
const curlStatuses = computed(
  () =>
    `curl -X POST -H "Authorization: Bearer <токен 1С>" -H "Content-Type: application/json" \\\n  -d '{"codes":["(01)04870712001017(21)..."]}' \\\n  "${origin}/api/integration/v1/codes/statuses"`,
)

const curlOrders = `curl -H "Authorization: Bearer <токен 1С>" "${origin}/api/integration/v1/orders"`

onMounted(async () => {
  try {
    orders.value = (await api.orders({ page: 1 })).data
  } catch {
    orders.value = []
  }
  try {
    tokenSet.value = (await api.settings()).integration_token_set
  } catch {
    tokenSet.value = null
  }
})

async function fetchWith(format: 'csv' | 'json', t: string) {
  busy.value = true
  error.value = ''
  try {
    const { blob, filename } = await api.integrationCodes(t, {
      format,
      order: orderNumber.value || undefined,
      status: status.value || undefined,
    })
    const stamp = new Date().toISOString().slice(0, 10)
    saveBlob(blob, filename ?? `codes-${orderNumber.value || 'all'}-${stamp}.${format}`)
    notify('Файл с кодами скачан')
    askToken.value = false
  } catch (e) {
    if (e instanceof ApiError && e.status === 401) {
      token.value = ''
      writeToken('')
      pendingFormat.value = format
      askToken.value = true
      error.value = 'Токен не подошёл. Проверьте его или выпустите новый в настройках'
    } else {
      error.value = errorMessage(e)
    }
  } finally {
    busy.value = false
  }
}

function download(format: 'csv' | 'json') {
  if (!token.value) {
    pendingFormat.value = format
    askToken.value = true
    return
  }
  fetchWith(format, token.value)
}

function submitToken() {
  const t = tokenInput.value.trim()
  if (!t) {
    error.value = 'Вставьте токен интеграции'
    return
  }
  token.value = t
  writeToken(t)
  tokenInput.value = ''
  if (pendingFormat.value) fetchWith(pendingFormat.value, t)
}

function forgetToken() {
  token.value = ''
  writeToken('')
  notify('Токен убран из этой вкладки', 'info')
}
</script>

<template>
  <div class="page">
    <div class="page-head">
      <div>
        <h1>Выгрузка в 1С</h1>
        <p>
          Коды маркировки передаются в 1С, чтобы указывать их в реализации и счетах-фактурах. Работает с «Бухгалтерией для
          Казахстана» 3.0 и «Управлением нашей фирмой для Казахстана».
        </p>
      </div>
    </div>

    <div class="grid grid-2">
      <section class="card">
        <div class="card-head"><h2>Как это устроено</h2></div>
        <div class="card-body stack">
          <ol class="steps">
            <li>
              В <RouterLink :to="{ name: 'settings' }">настройках</RouterLink> выпустите токен для 1С. Он показывается один
              раз, сохраните его.
            </li>
            <li>Скачайте внешнюю обработку и откройте её в 1С: Файл, Открыть.</li>
            <li>В обработке укажите адрес кабинета <code class="break">{{ origin }}</code> и токен.</li>
            <li>Выберите заказ или GTIN, обработка загрузит коды и их статусы через REST API.</li>
          </ol>
          <div class="row">
            <a class="btn btn-primary" href="/downloads/onec-obrabotka.zip" download>
              <Icon name="download" :size="16" />
              Скачать обработку 1С (исходники)
            </a>
          </div>
          <div v-if="tokenSet === false" class="alert tone-warning">
            <Icon name="key" :size="16" />
            <span>
              Токен для 1С ещё не выпущен.
              <RouterLink :to="{ name: 'settings', hash: '#integration' }">Выпустить в настройках</RouterLink>
            </span>
          </div>
        </div>
      </section>

      <section class="card">
        <div class="card-head"><h2>Скачать коды файлом</h2></div>
        <div class="card-body stack">
          <div class="form-grid">
            <div class="field">
              <label for="x-order">Заказ</label>
              <select id="x-order" v-model="orderNumber" class="select">
                <option value="">Все заказы</option>
                <option v-for="o in orders" :key="o.id" :value="o.number">{{ o.number }} · {{ o.product.name }}</option>
              </select>
            </div>
            <div class="field">
              <label for="x-status">Статус кодов</label>
              <select id="x-status" v-model="status" class="select">
                <option value="">Все статусы</option>
                <option v-for="s in codeStatusOrder" :key="s" :value="s">{{ codeStatuses[s].label }}</option>
              </select>
            </div>
          </div>

          <form v-if="askToken" class="token-ask stack" @submit.prevent="submitToken">
            <div class="field">
              <label for="x-token">Токен интеграции 1С</label>
              <input id="x-token" v-model="tokenInput" class="input mono" type="password" autocomplete="off" placeholder="Вставьте токен" />
              <span class="field-hint">Токен запомнится только в этой вкладке браузера, до её закрытия.</span>
            </div>
            <div class="row">
              <button class="btn btn-primary btn-sm" type="submit" :disabled="busy">Продолжить</button>
              <button class="btn btn-sm" type="button" @click="askToken = false">Отмена</button>
            </div>
          </form>

          <div v-if="error" class="alert tone-danger" role="alert">
            <Icon name="alert" :size="16" />
            <span class="break">{{ error }}</span>
          </div>

          <div class="row">
            <button class="btn" type="button" :disabled="busy" @click="download('csv')">
              <span v-if="busy && pendingFormat !== 'json'" class="spinner" />
              <Icon v-else name="download" :size="16" />
              Скачать CSV
            </button>
            <button class="btn" type="button" :disabled="busy" @click="download('json')">
              <Icon name="download" :size="16" />
              Скачать JSON
            </button>
            <button v-if="token" class="btn btn-ghost btn-sm" type="button" @click="forgetToken">Забыть токен</button>
          </div>
          <p class="small muted">
            CSV в кодировке UTF-8, разделитель «;». Символ GS внутри кода сохраняется, дополнительно есть колонка с кодом в
            base64.
          </p>
        </div>
      </section>
    </div>

    <section class="card">
      <div class="card-head"><h2>REST API для 1С и других программ</h2></div>
      <div class="card-body stack">
        <CopyBlock label="Коды файлом CSV" :text="curlCsv" />
        <CopyBlock label="Коды в JSON" :text="curlJson" />
        <CopyBlock label="Статусы по списку кодов" :text="curlStatuses" />
        <CopyBlock label="Список заказов" :text="curlOrders" />
        <p class="small muted">
          Параметры выгрузки кодов: format (csv или json), order (номер заказа), gtin, status, updated_since (дата и время в
          формате ISO 8601).
        </p>
      </div>
    </section>
  </div>
</template>

<style scoped>
.steps {
  margin: 0;
  padding-left: 20px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.steps code {
  background: var(--c-surface-2);
  padding: 1px 5px;
  border-radius: 4px;
}

.token-ask {
  padding: 12px;
  border: 1px solid var(--c-border);
  border-radius: var(--radius-sm);
  background: var(--c-surface-2);
}
</style>
