<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { api } from '@/api'
import type { Product, ProductInput } from '@/api'
import { validateGtin } from '@/utils/gtin'
import { errorMessage, fieldErrors } from '@/utils/errors'
import Modal from './Modal.vue'
import Icon from './Icon.vue'

const props = defineProps<{ product: Product | null; categories: string[] }>()
const emit = defineEmits<{ close: []; saved: [product: Product] }>()

type FieldKey = keyof ProductInput

const keys: FieldKey[] = ['gtin', 'name', 'model', 'size', 'color', 'composition', 'tnved', 'category', 'brand', 'country']

const form = reactive<Record<FieldKey, string>>(
  Object.fromEntries(keys.map((k) => [k, String(props.product?.[k] ?? (k === 'country' ? 'Казахстан' : ''))])) as Record<
    FieldKey,
    string
  >,
)
const errors = ref<Record<string, string>>({})
const error = ref('')
const busy = ref(false)

const isEdit = computed(() => !!props.product)
const hasCodes = computed(() => {
  const c = props.product?.codes
  return !!c && c.received + c.printed + c.applied + c.introduced + c.annulled > 0
})

function validate(): boolean {
  const e: Record<string, string> = {}
  const gtinError = validateGtin(form.gtin)
  if (gtinError) e.gtin = gtinError
  if (!form.name.trim()) e.name = 'Укажите наименование'
  const tnved = form.tnved.trim()
  if (tnved && !/^\d{10}$/.test(tnved)) e.tnved = 'Код ТН ВЭД состоит из 10 цифр'
  errors.value = e
  return !Object.keys(e).length
}

function onGtinBlur() {
  if (!form.gtin.trim()) return
  const msg = validateGtin(form.gtin)
  errors.value = { ...errors.value, gtin: msg ?? '' }
}

async function submit() {
  error.value = ''
  if (!validate()) return
  const input: ProductInput = { gtin: form.gtin.trim(), name: form.name.trim() }
  for (const k of keys) {
    if (k === 'gtin' || k === 'name') continue
    input[k] = form[k].trim()
  }
  busy.value = true
  try {
    const res = props.product ? await api.updateProduct(props.product.id, input) : await api.createProduct(input)
    emit('saved', res.data)
  } catch (e) {
    errors.value = fieldErrors(e)
    if (!Object.keys(errors.value).length) error.value = errorMessage(e)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <Modal :title="isEdit ? 'Карточка товара' : 'Новый товар'" :width="720" side @close="emit('close')">
    <form id="product-form" class="stack" novalidate @submit.prevent="submit">
      <p class="small text-2">
        GTIN берётся из Национального каталога товаров (НКТ). Если карточки ещё нет, заведите её в НКТ и укажите полученный
        GTIN.
      </p>
      <div class="form-grid">
        <div class="field">
          <label for="p-gtin">GTIN *</label>
          <input
            id="p-gtin"
            v-model="form.gtin"
            class="input mono"
            :class="{ invalid: errors.gtin }"
            inputmode="numeric"
            maxlength="14"
            :readonly="isEdit && hasCodes"
            @blur="onGtinBlur"
          />
          <span v-if="errors.gtin" class="field-error">{{ errors.gtin }}</span>
          <span v-else-if="isEdit && hasCodes" class="field-hint">По товару уже заказаны коды, GTIN менять нельзя</span>
          <span v-else class="field-hint">13 или 14 цифр, контрольная цифра проверяется</span>
        </div>
        <div class="field">
          <label for="p-cat">Категория</label>
          <input id="p-cat" v-model="form.category" class="input" list="p-cat-list" :class="{ invalid: errors.category }" />
          <datalist id="p-cat-list">
            <option v-for="c in categories" :key="c" :value="c" />
          </datalist>
          <span v-if="errors.category" class="field-error">{{ errors.category }}</span>
        </div>
        <div class="field span-2">
          <label for="p-name">Наименование *</label>
          <input id="p-name" v-model="form.name" class="input" :class="{ invalid: errors.name }" maxlength="255" />
          <span v-if="errors.name" class="field-error">{{ errors.name }}</span>
        </div>
        <div class="field">
          <label for="p-model">Модель</label>
          <input id="p-model" v-model="form.model" class="input" :class="{ invalid: errors.model }" />
          <span v-if="errors.model" class="field-error">{{ errors.model }}</span>
        </div>
        <div class="field">
          <label for="p-brand">Бренд</label>
          <input id="p-brand" v-model="form.brand" class="input" :class="{ invalid: errors.brand }" />
          <span v-if="errors.brand" class="field-error">{{ errors.brand }}</span>
        </div>
        <div class="field">
          <label for="p-size">Размер</label>
          <input id="p-size" v-model="form.size" class="input" :class="{ invalid: errors.size }" />
          <span v-if="errors.size" class="field-error">{{ errors.size }}</span>
        </div>
        <div class="field">
          <label for="p-color">Цвет</label>
          <input id="p-color" v-model="form.color" class="input" :class="{ invalid: errors.color }" />
          <span v-if="errors.color" class="field-error">{{ errors.color }}</span>
        </div>
        <div class="field span-2">
          <label for="p-comp">Состав</label>
          <input
            id="p-comp"
            v-model="form.composition"
            class="input"
            :class="{ invalid: errors.composition }"
            placeholder="Например: 95% хлопок, 5% эластан"
          />
          <span v-if="errors.composition" class="field-error">{{ errors.composition }}</span>
        </div>
        <div class="field">
          <label for="p-tnved">ТН ВЭД</label>
          <input
            id="p-tnved"
            v-model="form.tnved"
            class="input mono"
            :class="{ invalid: errors.tnved }"
            inputmode="numeric"
            maxlength="10"
          />
          <span v-if="errors.tnved" class="field-error">{{ errors.tnved }}</span>
          <span v-else class="field-hint">10 цифр, например 6109100000</span>
        </div>
        <div class="field">
          <label for="p-country">Страна производства</label>
          <input id="p-country" v-model="form.country" class="input" :class="{ invalid: errors.country }" />
          <span v-if="errors.country" class="field-error">{{ errors.country }}</span>
        </div>
      </div>
      <div v-if="error" class="alert tone-danger" role="alert">
        <Icon name="alert" :size="16" />
        <span class="break">{{ error }}</span>
      </div>
    </form>
    <template #footer>
      <button class="btn" type="button" @click="emit('close')">Отмена</button>
      <button class="btn btn-primary" type="submit" form="product-form" :disabled="busy">
        <span v-if="busy" class="spinner" />
        {{ isEdit ? 'Сохранить' : 'Добавить товар' }}
      </button>
    </template>
  </Modal>
</template>
