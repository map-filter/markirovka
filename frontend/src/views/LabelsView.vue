<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { api, openBlob } from '@/api'
import type { Code, LabelFields, LabelSize, LabelTemplate, LabelTemplateInput, Product } from '@/api'
import { session } from '@/stores/session'
import { notify, notifyError } from '@/stores/toast'
import { buildSample, defaultFields } from '@/utils/label'
import { errorMessage, fieldErrors } from '@/utils/errors'
import Icon from '@/components/Icon.vue'
import LabelPreview from '@/components/LabelPreview.vue'
import ProductPicker from '@/components/ProductPicker.vue'

const templates = ref<LabelTemplate[]>([])
const loading = ref(true)
const error = ref('')
const currentId = ref<number | null>(null)
const busy = ref(false)
const previewBusy = ref(false)
const errors = ref<Record<string, string>>({})

const form = reactive<LabelTemplateInput>({
  name: 'Этикетка 58x40',
  size: '58x40',
  fields: { ...defaultFields },
  font_scale: 1,
  is_default: false,
})
const savedSnapshot = ref('')

const productId = ref<number | null>(null)
const product = ref<Product | null>(null)
const sampleCode = ref<Code | null>(null)

const fieldList: { key: keyof LabelFields; label: string; large?: boolean }[] = [
  { key: 'datamatrix', label: 'Data Matrix' },
  { key: 'name', label: 'Наименование' },
  { key: 'size', label: 'Размер' },
  { key: 'gtin', label: 'GTIN' },
  { key: 'composition', label: 'Состав', large: true },
  { key: 'manufacturer', label: 'Производитель', large: true },
  { key: 'serial', label: 'Серийный номер под кодом' },
]

const sizes: { value: LabelSize; label: string; hint: string }[] = [
  { value: '58x40', label: '58 x 40 мм', hint: 'Код 22 мм, наименование, размер, состав, GTIN, производитель' },
  { value: '43x25', label: '43 x 25 мм', hint: 'Компактная: код 17 мм, наименование, размер, GTIN' },
]

const snapshot = () => JSON.stringify(form)
const dirty = computed(() => snapshot() !== savedSnapshot.value)
const current = computed(() => templates.value.find((t) => t.id === currentId.value) ?? null)
const sample = computed(() => buildSample(product.value, sampleCode.value, session.organization))

function fill(t: LabelTemplate | null) {
  if (t) {
    form.name = t.name
    form.size = t.size
    form.fields = { ...defaultFields, ...t.fields }
    form.font_scale = t.font_scale ?? 1
    form.is_default = !!t.is_default
  } else {
    form.name = 'Новая этикетка'
    form.size = '58x40'
    form.fields = { ...defaultFields }
    form.font_scale = 1
    form.is_default = templates.value.length === 0
  }
  errors.value = {}
  savedSnapshot.value = t ? snapshot() : ''
}

async function load(selectId?: number) {
  error.value = ''
  try {
    templates.value = (await api.labelTemplates()).data
    const pickId = selectId ?? currentId.value ?? templates.value.find((t) => t.is_default)?.id ?? templates.value[0]?.id ?? null
    select(pickId)
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    loading.value = false
  }
}

function select(id: number | null) {
  if (dirty.value && currentId.value !== id && savedSnapshot.value && !window.confirm('Изменения шаблона не сохранены. Перейти без сохранения?')) {
    return
  }
  currentId.value = id
  fill(templates.value.find((t) => t.id === id) ?? null)
}

function createNew() {
  currentId.value = null
  fill(null)
}

async function save(): Promise<LabelTemplate | null> {
  errors.value = {}
  if (!form.name.trim()) {
    errors.value.name = 'Укажите название шаблона'
    return null
  }
  busy.value = true
  try {
    const input: LabelTemplateInput = {
      name: form.name.trim(),
      size: form.size,
      fields: { ...form.fields },
      font_scale: Math.round(form.font_scale * 100) / 100,
      is_default: form.is_default,
    }
    const res = currentId.value ? await api.updateLabelTemplate(currentId.value, input) : await api.createLabelTemplate(input)
    notify('Шаблон сохранён')
    currentId.value = res.data.id
    savedSnapshot.value = ''
    await load(res.data.id)
    return res.data
  } catch (e) {
    errors.value = fieldErrors(e)
    if (!Object.keys(errors.value).length) notifyError(e)
    return null
  } finally {
    busy.value = false
  }
}

async function remove() {
  const t = current.value
  if (!t || !window.confirm(`Удалить шаблон «${t.name}»?`)) return
  try {
    await api.deleteLabelTemplate(t.id)
    notify('Шаблон удалён')
    currentId.value = null
    savedSnapshot.value = ''
    await load()
  } catch (e) {
    notifyError(e)
  }
}

async function pdfPreview() {
  let id = currentId.value
  if (!id || dirty.value) {
    const saved = await save()
    if (!saved) return
    id = saved.id
  }
  previewBusy.value = true
  try {
    const { blob } = await api.labelTemplatePreview(id, productId.value ?? undefined)
    openBlob(new Blob([blob], { type: 'application/pdf' }), `obrazec-${id}.pdf`)
  } catch (e) {
    notifyError(e)
  } finally {
    previewBusy.value = false
  }
}

async function loadSampleCode(id: number | null) {
  sampleCode.value = null
  if (!id) return
  try {
    const res = await api.codes({ product_id: id, per_page: 1 })
    if (productId.value === id) sampleCode.value = res.data[0] ?? null
  } catch {
    sampleCode.value = null
  }
}

watch(productId, (id) => loadSampleCode(id))

async function pickFirstProduct() {
  try {
    const res = await api.products({ per_page: 1 })
    const p = res.data[0]
    if (p && !productId.value) {
      product.value = p
      productId.value = p.id
    }
  } catch {
    void 0
  }
}

onMounted(() => {
  load()
  pickFirstProduct()
})

watch(
  () => form.size,
  (size, prev) => {
    if (prev && form.name === `Этикетка ${prev}`) form.name = `Этикетка ${size}`
  },
)
</script>

<template>
  <div class="page">
    <div class="page-head">
      <div>
        <h1>Этикетки</h1>
        <p>Шаблон определяет размер и что печатается рядом с кодом Data Matrix. Предпросмотр нарисован в масштабе.</p>
      </div>
      <div class="page-actions">
        <button class="btn" type="button" @click="createNew">
          <Icon name="plus" :size="16" />
          Новый шаблон
        </button>
      </div>
    </div>

    <div v-if="error" class="alert tone-danger" role="alert">
      <Icon name="alert" :size="16" />
      <span class="break">{{ error }}</span>
    </div>

    <div class="labels-grid">
      <section class="card tpl-list">
        <div class="card-head"><h2>Шаблоны</h2></div>
        <div v-if="loading" class="loading-line">Загружаю...</div>
        <div v-else-if="!templates.length" class="empty small">Шаблонов пока нет. Настройте первый справа и сохраните.</div>
        <ul v-else>
          <li v-for="t in templates" :key="t.id">
            <button type="button" class="tpl" :class="{ active: t.id === currentId }" @click="select(t.id)">
              <span class="tpl-name break">{{ t.name }}</span>
              <span class="row small muted">
                {{ t.size }} мм
                <span v-if="t.is_default" class="badge plain tone-accent">по умолчанию</span>
              </span>
            </button>
          </li>
        </ul>
      </section>

      <section class="card editor">
        <div class="card-head">
          <h2>{{ currentId ? 'Настройка шаблона' : 'Новый шаблон' }}</h2>
          <span v-if="dirty && currentId" class="badge tone-warning">Не сохранено</span>
        </div>
        <div class="card-body stack">
          <div class="field">
            <label for="t-name">Название</label>
            <input id="t-name" v-model="form.name" class="input" :class="{ invalid: errors.name }" maxlength="100" />
            <span v-if="errors.name" class="field-error">{{ errors.name }}</span>
          </div>
          <div class="field">
            <span class="field-label">Размер этикетки</span>
            <div class="sizes">
              <label v-for="s in sizes" :key="s.value" class="size-opt" :class="{ active: form.size === s.value }">
                <input v-model="form.size" type="radio" name="size" :value="s.value" class="visually-hidden" />
                <strong>{{ s.label }}</strong>
                <span class="small muted">{{ s.hint }}</span>
              </label>
            </div>
            <span v-if="errors.size" class="field-error">{{ errors.size }}</span>
          </div>
          <div class="field">
            <span class="field-label">Что печатать</span>
            <div class="toggles">
              <label
                v-for="f in fieldList"
                :key="f.key"
                class="check"
                :class="{ disabled: f.large && form.size === '43x25' }"
                :title="f.large && form.size === '43x25' ? 'Не помещается на этикетку 43 x 25 мм' : ''"
              >
                <input
                  v-model="form.fields[f.key]"
                  type="checkbox"
                  :disabled="(f.large && form.size === '43x25') || (f.key === 'serial' && !form.fields.datamatrix)"
                />
                <span>{{ f.label }}</span>
              </label>
            </div>
            <span v-if="!form.fields.datamatrix" class="field-hint warn">
              Без Data Matrix этикетка не считается маркировкой. Отключайте только для пробной печати.
            </span>
            <span v-if="form.size === '43x25'" class="field-hint">На 43 x 25 мм состав и производитель не печатаются.</span>
          </div>
          <div class="field">
            <label for="t-scale">Размер шрифта: {{ Math.round(form.font_scale * 100) }}%</label>
            <input id="t-scale" v-model.number="form.font_scale" type="range" min="0.7" max="1.4" step="0.05" class="range" />
            <span v-if="errors.font_scale" class="field-error">{{ errors.font_scale }}</span>
          </div>
          <label class="check">
            <input v-model="form.is_default" type="checkbox" />
            Использовать по умолчанию при печати
          </label>
          <div class="divider" />
          <div class="row">
            <button class="btn btn-primary" type="button" :disabled="busy || (!dirty && !!currentId)" @click="save">
              <span v-if="busy" class="spinner" />
              Сохранить
            </button>
            <button class="btn" type="button" :disabled="previewBusy || busy" @click="pdfPreview">
              <span v-if="previewBusy" class="spinner" />
              <Icon v-else name="file" :size="16" />
              Образец PDF
            </button>
            <button v-if="currentId" class="btn btn-danger" type="button" @click="remove">
              <Icon name="trash" :size="16" />
              Удалить
            </button>
          </div>
        </div>
      </section>

      <section class="card preview-card">
        <div class="card-head"><h2>Предпросмотр</h2></div>
        <div class="card-body stack">
          <div class="field">
            <label for="t-product">Данные товара для образца</label>
            <ProductPicker v-model="productId" input-id="t-product" :selected="product" @pick="(p) => (product = p)" />
          </div>
          <div class="preview-stage">
            <LabelPreview :size="form.size" :fields="form.fields" :font-scale="form.font_scale" :sample="sample" />
          </div>
          <p class="small muted">
            {{ sampleCode ? 'В образце настоящий код этого товара.' : 'В образце пробный код: сканер прочитает его, но ИС МПТ его не знает.' }}
            Итоговый вид проверьте кнопкой «Образец PDF».
          </p>
        </div>
      </section>
    </div>
  </div>
</template>

<style scoped>
.labels-grid {
  display: grid;
  grid-template-columns: minmax(0, 220px) minmax(0, 1fr) minmax(0, 1.15fr);
  gap: 16px;
  align-items: start;
}

.tpl-list ul {
  list-style: none;
  margin: 0;
  padding: 6px;
}

.tpl {
  display: flex;
  flex-direction: column;
  gap: 2px;
  width: 100%;
  text-align: left;
  border: 1px solid transparent;
  background: none;
  padding: 9px 10px;
  border-radius: var(--radius-sm);
  cursor: pointer;
}

.tpl:hover {
  background: var(--c-surface-2);
}

.tpl.active {
  background: var(--c-accent-soft);
  border-color: #c4dedb;
}

.tpl-name {
  font-weight: 600;
  font-size: 14px;
}

.sizes {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
}

.size-opt {
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 10px 12px;
  border: 1px solid var(--c-border-strong);
  border-radius: var(--radius-sm);
  cursor: pointer;
  min-width: 0;
}

.size-opt.active {
  border-color: var(--c-accent);
  background: var(--c-accent-soft);
  box-shadow: inset 0 0 0 1px var(--c-accent);
}

.toggles {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px 12px;
}

.check.disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.warn {
  color: var(--c-warning);
}

.range {
  width: 100%;
  accent-color: var(--c-accent);
}

.preview-stage {
  background: repeating-conic-gradient(#eef2f1 0% 25%, #f7f9f8 0% 50%) 50% / 16px 16px;
  border: 1px solid var(--c-border);
  border-radius: var(--radius-sm);
  padding: 22px 12px;
  min-width: 0;
}

@media (max-width: 1280px) {
  .labels-grid {
    grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
  }

  .tpl-list {
    grid-column: 1 / -1;
  }

  .tpl-list ul {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
  }

  .tpl-list li {
    flex: 0 1 220px;
    min-width: 0;
  }
}

@media (max-width: 900px) {
  .labels-grid {
    grid-template-columns: minmax(0, 1fr);
  }

  .preview-card {
    order: -1;
  }

  .tpl-list {
    order: -2;
  }
}

@media (max-width: 480px) {
  .sizes,
  .toggles {
    grid-template-columns: minmax(0, 1fr);
  }

  .tpl-list li {
    flex: 1 1 100%;
  }
}
</style>
