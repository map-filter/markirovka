<script setup lang="ts">
import { ref } from 'vue'
import { api, saveBlob } from '@/api'
import type { ImportResult } from '@/api'
import { errorMessage } from '@/utils/errors'
import { notifyError } from '@/stores/toast'
import Modal from './Modal.vue'
import Icon from './Icon.vue'

const emit = defineEmits<{ close: []; imported: [] }>()

const file = ref<File | null>(null)
const busy = ref(false)
const error = ref('')
const result = ref<ImportResult | null>(null)
const dragging = ref(false)

function onPick(e: Event) {
  const input = e.target as HTMLInputElement
  file.value = input.files?.[0] ?? null
  result.value = null
  error.value = ''
}

function onDrop(e: DragEvent) {
  dragging.value = false
  const f = e.dataTransfer?.files?.[0]
  if (f) {
    file.value = f
    result.value = null
    error.value = ''
  }
}

async function downloadTemplate() {
  try {
    const { blob, filename } = await api.importTemplate()
    saveBlob(blob, filename ?? 'shablon-tovarov.csv')
  } catch (e) {
    notifyError(e)
  }
}

async function upload() {
  if (!file.value) {
    error.value = 'Выберите файл CSV или XLSX'
    return
  }
  busy.value = true
  error.value = ''
  try {
    result.value = await api.importProducts(file.value)
    if (result.value.created || result.value.updated) emit('imported')
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <Modal title="Импорт товаров из Excel или CSV" :width="640" @close="emit('close')">
    <div class="stack">
      <p class="small text-2">
        Загрузите таблицу с колонками как в шаблоне. Товары сопоставляются по GTIN: новые добавятся, существующие обновятся.
      </p>
      <div>
        <button class="btn btn-sm" type="button" @click="downloadTemplate">
          <Icon name="download" :size="15" />
          Скачать шаблон CSV
        </button>
      </div>
      <label
        class="drop"
        :class="{ dragging }"
        @dragover.prevent="dragging = true"
        @dragleave.prevent="dragging = false"
        @drop.prevent="onDrop"
      >
        <input class="visually-hidden" type="file" accept=".csv,.xlsx,.xls,text/csv" @change="onPick" />
        <Icon name="upload" :size="24" />
        <span v-if="file" class="break"><strong>{{ file.name }}</strong></span>
        <span v-else>Перетащите файл сюда или нажмите, чтобы выбрать</span>
        <span class="small muted">CSV (разделитель ;) или XLSX</span>
      </label>
      <div v-if="error" class="alert tone-danger" role="alert">
        <Icon name="alert" :size="16" />
        <span class="break">{{ error }}</span>
      </div>
      <div v-if="result" class="stack">
        <div class="alert" :class="result.errors.length ? 'tone-warning' : 'tone-success'">
          <Icon :name="result.errors.length ? 'alert' : 'check'" :size="16" />
          <span>
            Добавлено: <strong>{{ result.created }}</strong>, обновлено: <strong>{{ result.updated }}</strong>, с ошибками:
            <strong>{{ result.errors.length }}</strong>
          </span>
        </div>
        <div v-if="result.errors.length" class="table-wrap errors">
          <table class="table">
            <thead>
              <tr>
                <th>Строка</th>
                <th>Ошибка</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="(er, i) in result.errors" :key="i">
                <td class="nowrap">{{ er.row }}</td>
                <td class="break">{{ er.message }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </div>
    <template #footer>
      <button class="btn" type="button" @click="emit('close')">{{ result ? 'Закрыть' : 'Отмена' }}</button>
      <button class="btn btn-primary" type="button" :disabled="busy || !file" @click="upload">
        <span v-if="busy" class="spinner" />
        Загрузить
      </button>
    </template>
  </Modal>
</template>

<style scoped>
.drop {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
  text-align: center;
  padding: 26px 16px;
  border: 2px dashed var(--c-border-strong);
  border-radius: var(--radius);
  cursor: pointer;
  color: var(--c-text-2);
  background: var(--c-surface-2);
}

.drop.dragging,
.drop:hover {
  border-color: var(--c-accent);
  background: var(--c-accent-soft);
}

.errors {
  max-height: 260px;
  overflow-y: auto;
  border: 1px solid var(--c-border);
  border-radius: var(--radius-sm);
}
</style>
