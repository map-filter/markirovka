<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api } from '@/api'
import type { Operation, OperationLevel, PageMeta } from '@/api'
import { levelStatuses } from '@/utils/status'
import { formatDateTime } from '@/utils/format'
import { errorMessage } from '@/utils/errors'
import Icon from '@/components/Icon.vue'
import StatusBadge from '@/components/StatusBadge.vue'
import Pagination from '@/components/Pagination.vue'

const route = useRoute()
const router = useRouter()

const types: { value: string; label: string }[] = [
  { value: 'ismpt', label: 'Подключение к ИС МПТ' },
  { value: 'product', label: 'Товары и НКТ' },
  { value: 'order', label: 'Заказы кодов' },
  { value: 'print', label: 'Печать' },
  { value: 'document', label: 'Документы' },
  { value: 'agent', label: 'Агенты печати' },
  { value: 'integration', label: 'Интеграция 1С' },
  { value: 'settings', label: 'Настройки' },
]

const typeLabels: Record<string, string> = Object.fromEntries(types.map((t) => [t.value, t.label]))

const items = ref<Operation[]>([])
const meta = ref<PageMeta | null>(null)
const loading = ref(true)
const error = ref('')
const type = ref(typeof route.query.type === 'string' ? route.query.type : '')
const level = ref<OperationLevel | ''>((typeof route.query.level === 'string' ? route.query.level : '') as OperationLevel | '')
const page = ref(Number(route.query.page) || 1)
const knownTypes = ref<string[]>([])

function typeGroup(t: string): string {
  return typeLabels[t.split('.')[0]] ?? t
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const res = await api.operations({ type: type.value || undefined, level: level.value || undefined, page: page.value })
    items.value = res.data
    meta.value = res.meta
    const set = new Set(knownTypes.value)
    res.data.forEach((o) => set.add(o.type))
    knownTypes.value = Array.from(set).sort()
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    loading.value = false
  }
}

function syncQuery() {
  const q: Record<string, string> = {}
  if (type.value) q.type = type.value
  if (level.value) q.level = level.value
  if (page.value > 1) q.page = String(page.value)
  router.replace({ query: q })
}

watch([type, level], () => {
  page.value = 1
  syncQuery()
  load()
})

function setPage(p: number) {
  page.value = p
  syncQuery()
  load()
}

onMounted(load)
</script>

<template>
  <div class="page">
    <div class="page-head">
      <div>
        <h1>Журнал</h1>
        <p>Все действия в кабинете и ответы ИС МПТ. Пригодится, если нужно разобраться, что и когда произошло.</p>
      </div>
      <div class="page-actions">
        <button class="btn" type="button" @click="load">
          <Icon name="refresh" :size="16" />
          Обновить
        </button>
      </div>
    </div>

    <section class="card">
      <div class="card-body">
        <div class="filters">
          <div class="field">
            <label for="j-type">Тип операции</label>
            <select id="j-type" v-model="type" class="select">
              <option value="">Все операции</option>
              <optgroup label="Разделы">
                <option v-for="t in types" :key="t.value" :value="t.value">{{ t.label }}</option>
              </optgroup>
              <optgroup v-if="knownTypes.length" label="Точный тип">
                <option v-for="t in knownTypes" :key="t" :value="t">{{ t }}</option>
              </optgroup>
            </select>
          </div>
          <div class="field">
            <label for="j-level">Уровень</label>
            <select id="j-level" v-model="level" class="select">
              <option value="">Любой</option>
              <option value="info">Инфо</option>
              <option value="warning">Внимание</option>
              <option value="error">Ошибка</option>
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
      <div v-else-if="loading && !items.length" class="loading-line">Загружаю...</div>
      <div v-else-if="!items.length" class="empty">Записей нет</div>
      <template v-else>
        <div class="table-wrap desktop-table">
          <table class="table">
            <thead>
              <tr>
                <th>Время</th>
                <th>Уровень</th>
                <th>Тип</th>
                <th>Событие</th>
                <th>Пользователь</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="o in items" :key="o.id">
                <td class="nowrap">{{ formatDateTime(o.created_at) }}</td>
                <td><StatusBadge :map="levelStatuses" :value="o.level" /></td>
                <td>
                  <div class="small">{{ typeGroup(o.type) }}</div>
                  <div class="cell-sub mono">{{ o.type }}</div>
                </td>
                <td class="break">{{ o.message }}</td>
                <td class="break">{{ o.user || 'система' }}</td>
              </tr>
            </tbody>
          </table>
        </div>
        <div class="mobile-cards">
          <div v-for="o in items" :key="o.id" class="m-card">
            <div class="row-between">
              <span class="small muted">{{ formatDateTime(o.created_at) }}</span>
              <StatusBadge :map="levelStatuses" :value="o.level" />
            </div>
            <span class="break">{{ o.message }}</span>
            <div class="m-meta">
              <span>{{ typeGroup(o.type) }}</span>
              <span>{{ o.user || 'система' }}</span>
            </div>
          </div>
        </div>
        <Pagination :meta="meta" @change="setPage" />
      </template>
    </section>
  </div>
</template>
