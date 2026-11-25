<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { api } from '@/api'
import type { PrintAgent, Settings } from '@/api'
import { session } from '@/stores/session'
import { notify, notifyError } from '@/stores/toast'
import { formatDateTime } from '@/utils/format'
import { errorMessage, fieldErrors } from '@/utils/errors'
import Icon from '@/components/Icon.vue'
import IsmptConnect from '@/components/IsmptConnect.vue'
import CopyBlock from '@/components/CopyBlock.vue'
import Modal from '@/components/Modal.vue'

const settings = ref<Settings | null>(null)
const loading = ref(true)
const error = ref('')

const org = reactive({ name: '', address: '', phone: '' })
const orgErrors = ref<Record<string, string>>({})
const orgBusy = ref(false)

const tokenBusy = ref(false)
const newToken = ref<string | null>(null)

const agentName = ref('')
const agentBusy = ref(false)
const agentError = ref('')
const createdAgent = ref<{ agent: PrintAgent; token: string } | null>(null)

const origin = window.location.origin
const isAdmin = computed(() => session.user?.role !== 'operator')
const agentCommand = computed(() =>
  createdAgent.value
    ? `markirovka-agent run -server ${origin} -token ${createdAgent.value.token} -printer tcp://192.168.1.50:9100`
    : '',
)

async function load() {
  error.value = ''
  try {
    settings.value = await api.settings()
    org.name = settings.value.organization.name ?? ''
    org.address = settings.value.organization.address ?? ''
    org.phone = settings.value.organization.phone ?? ''
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    loading.value = false
  }
}

onMounted(load)

async function saveOrg() {
  orgErrors.value = {}
  if (!org.name.trim()) {
    orgErrors.value.name = 'Укажите наименование'
    return
  }
  orgBusy.value = true
  try {
    const res = await api.updateSettings({ name: org.name.trim(), address: org.address.trim(), phone: org.phone.trim() })
    session.organization = res.organization
    if (settings.value) settings.value.organization = res.organization
    notify('Реквизиты сохранены')
  } catch (e) {
    orgErrors.value = fieldErrors(e)
    if (!Object.keys(orgErrors.value).length) notifyError(e)
  } finally {
    orgBusy.value = false
  }
}

async function issueToken() {
  if (settings.value?.integration_token_set && !window.confirm('Выпустить новый токен? Старый перестанет работать, его нужно будет заменить в 1С.')) {
    return
  }
  tokenBusy.value = true
  try {
    const res = await api.issueIntegrationToken()
    newToken.value = res.token
    if (settings.value) settings.value.integration_token_set = true
  } catch (e) {
    notifyError(e)
  } finally {
    tokenBusy.value = false
  }
}

async function addAgent() {
  agentError.value = ''
  const name = agentName.value.trim()
  if (!name) {
    agentError.value = 'Укажите название, например «Цех, Xprinter»'
    return
  }
  agentBusy.value = true
  try {
    const res = await api.createAgent(name)
    createdAgent.value = { agent: res.data, token: res.token }
    agentName.value = ''
    if (settings.value) settings.value.agents = [...settings.value.agents, res.data]
  } catch (e) {
    const fe = fieldErrors(e)
    agentError.value = fe.name ?? errorMessage(e)
  } finally {
    agentBusy.value = false
  }
}

async function removeAgent(a: PrintAgent) {
  if (!window.confirm(`Отключить агент «${a.name}»? Он перестанет получать задания.`)) return
  try {
    await api.deleteAgent(a.id)
    if (settings.value) settings.value.agents = settings.value.agents.filter((x) => x.id !== a.id)
    notify('Агент отключён')
  } catch (e) {
    notifyError(e)
  }
}
</script>

<template>
  <div class="page">
    <div class="page-head">
      <div>
        <h1>Настройки</h1>
        <p>Реквизиты организации, подключение к ИС МПТ, интеграция с 1С и агенты печати.</p>
      </div>
    </div>

    <div v-if="error" class="alert tone-danger" role="alert">
      <Icon name="alert" :size="16" />
      <span class="break">{{ error }}</span>
    </div>
    <div v-if="loading" class="card loading-line">Загружаю...</div>

    <div v-else-if="settings" class="grid grid-2">
      <section class="card">
        <div class="card-head"><h2>Организация</h2></div>
        <form class="card-body stack" novalidate @submit.prevent="saveOrg">
          <p class="small text-2">Наименование и адрес печатаются на этикетке как производитель.</p>
          <div class="field">
            <label for="s-name">Наименование</label>
            <input id="s-name" v-model="org.name" class="input" :class="{ invalid: orgErrors.name }" :readonly="!isAdmin" />
            <span v-if="orgErrors.name" class="field-error">{{ orgErrors.name }}</span>
          </div>
          <div class="field">
            <label for="s-bin">БИН</label>
            <input id="s-bin" class="input mono" :value="settings.organization.bin" readonly />
            <span class="field-hint">БИН берётся из ключа ЭЦП и не меняется</span>
          </div>
          <div class="field">
            <label for="s-address">Адрес</label>
            <input id="s-address" v-model="org.address" class="input" :class="{ invalid: orgErrors.address }" :readonly="!isAdmin" />
            <span v-if="orgErrors.address" class="field-error">{{ orgErrors.address }}</span>
          </div>
          <div class="field">
            <label for="s-phone">Телефон</label>
            <input
              id="s-phone"
              v-model="org.phone"
              class="input"
              :class="{ invalid: orgErrors.phone }"
              type="tel"
              placeholder="+7 7xx xxx xx xx"
              :readonly="!isAdmin"
            />
            <span v-if="orgErrors.phone" class="field-error">{{ orgErrors.phone }}</span>
          </div>
          <div v-if="isAdmin">
            <button class="btn btn-primary" type="submit" :disabled="orgBusy">
              <span v-if="orgBusy" class="spinner" />
              Сохранить
            </button>
          </div>
        </form>
      </section>

      <section class="card">
        <div class="card-head"><h2>ИС МПТ</h2></div>
        <div class="card-body stack">
          <dl class="kv">
            <dt>Режим</dt>
            <dd>
              <span v-if="session.config?.ismpt_mode === 'emulator'" class="badge tone-info">Эмулятор ИС МПТ</span>
              <span v-else class="badge tone-success">Рабочая ИС МПТ</span>
            </dd>
            <dt>Адрес</dt>
            <dd class="mono small break">{{ session.config?.ismpt_base_url || '-' }}</dd>
            <dt>Подпись</dt>
            <dd>{{ session.config?.sign_mode === 'emulate' ? 'Эмуляция (без ключа ЭЦП)' : 'NCALayer, ключ ЭЦП организации' }}</dd>
            <template v-if="session.config?.emulator_console_url">
              <dt>Консоль</dt>
              <dd>
                <a :href="session.config.emulator_console_url" target="_blank" rel="noopener">
                  Открыть консоль эмулятора
                  <Icon name="external" :size="13" />
                </a>
              </dd>
            </template>
          </dl>
          <div class="divider" />
          <IsmptConnect />
          <p v-if="session.config?.sign_mode === 'ncalayer'" class="small muted">
            Нужен запущенный NCALayer. Если браузер не подключается к нему, один раз откройте
            <a href="https://127.0.0.1:13579" target="_blank" rel="noopener">https://127.0.0.1:13579</a> и разрешите
            подключение.
          </p>
        </div>
      </section>

      <section id="integration" class="card">
        <div class="card-head"><h2>Интеграция с 1С</h2></div>
        <div class="card-body stack">
          <p class="small text-2">
            Токен нужен обработке 1С и другим программам, чтобы забирать коды через REST API. Подробнее на странице
            <RouterLink :to="{ name: 'onec' }">«Выгрузка в 1С»</RouterLink>.
          </p>
          <div class="row">
            <span v-if="settings.integration_token_set" class="badge tone-success">Токен выпущен</span>
            <span v-else class="badge">Токен не выпущен</span>
          </div>
          <div v-if="newToken" class="alert tone-warning stack-alert">
            <Icon name="key" :size="16" />
            <div class="stack grow">
              <strong>Сохраните токен: он показывается один раз</strong>
              <CopyBlock :text="newToken" wrap />
            </div>
          </div>
          <div v-if="isAdmin">
            <button class="btn" type="button" :disabled="tokenBusy" @click="issueToken">
              <span v-if="tokenBusy" class="spinner" />
              <Icon v-else name="key" :size="16" />
              {{ settings.integration_token_set ? 'Выпустить новый токен' : 'Выпустить токен' }}
            </button>
          </div>
        </div>
      </section>

      <section class="card">
        <div class="card-head"><h2>Агенты печати</h2></div>
        <div class="card-body stack">
          <p class="small text-2">
            Агент ставится на компьютер рядом с термопринтером и печатает задания из кабинета без скачивания файлов.
          </p>
          <ul v-if="settings.agents.length" class="agents">
            <li v-for="a in settings.agents" :key="a.id" class="agent">
              <span class="dot" :class="{ on: a.online }" :title="a.online ? 'На связи' : 'Не в сети'" />
              <div class="agent-main">
                <strong class="break">{{ a.name }}</strong>
                <span class="small muted break">
                  {{ a.online ? 'На связи' : 'Не в сети' }}
                  <template v-if="a.last_seen_at">, был {{ formatDateTime(a.last_seen_at) }}</template>
                  <template v-if="a.hostname">, {{ a.hostname }}</template>
                  <template v-if="a.printer">, принтер {{ a.printer }}</template>
                  <template v-if="a.version">, версия {{ a.version }}</template>
                </span>
              </div>
              <button v-if="isAdmin" class="btn btn-sm btn-ghost btn-icon" type="button" aria-label="Отключить агент" title="Отключить" @click="removeAgent(a)">
                <Icon name="trash" :size="15" />
              </button>
            </li>
          </ul>
          <p v-else class="muted small">Агентов пока нет.</p>
          <form v-if="isAdmin" class="add-agent" novalidate @submit.prevent="addAgent">
            <div class="field grow">
              <label for="s-agent">Новый агент</label>
              <input id="s-agent" v-model="agentName" class="input" :class="{ invalid: agentError }" placeholder="Например: Цех, Xprinter" />
              <span v-if="agentError" class="field-error">{{ agentError }}</span>
            </div>
            <button class="btn" type="submit" :disabled="agentBusy">
              <span v-if="agentBusy" class="spinner" />
              <Icon v-else name="plus" :size="16" />
              Подключить агент
            </button>
          </form>
        </div>
      </section>
    </div>

    <Modal v-if="createdAgent" title="Агент подключён" :width="640" @close="createdAgent = null">
      <div class="stack">
        <div class="alert tone-warning">
          <Icon name="key" :size="16" />
          <span>Токен агента показывается один раз. Скопируйте команду и запустите её на компьютере у принтера.</span>
        </div>
        <CopyBlock label="Токен агента" :text="createdAgent.token" wrap />
        <CopyBlock label="Команда запуска" :text="agentCommand" wrap />
        <p class="small muted">
          Вместо tcp://192.168.1.50:9100 укажите адрес своего сетевого принтера (порт 9100) или имя принтера в системе.
        </p>
      </div>
      <template #footer>
        <button class="btn btn-primary" type="button" @click="createdAgent = null">Готово</button>
      </template>
    </Modal>
  </div>
</template>

<style scoped>
.agents {
  list-style: none;
  margin: 0;
  padding: 0;
  border: 1px solid var(--c-border);
  border-radius: var(--radius-sm);
}

.agent {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 12px;
  border-bottom: 1px solid var(--c-border);
}

.agent:last-child {
  border-bottom: 0;
}

.agent-main {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
}

.dot {
  width: 9px;
  height: 9px;
  border-radius: 50%;
  background: #b9c2c0;
  flex: none;
}

.dot.on {
  background: var(--c-success);
  box-shadow: 0 0 0 3px var(--c-success-soft);
}

.add-agent {
  display: flex;
  gap: 8px;
  align-items: flex-end;
  flex-wrap: wrap;
}

.grow {
  flex: 1 1 220px;
  min-width: 0;
}

.stack-alert {
  align-items: flex-start;
}
</style>
