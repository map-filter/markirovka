<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { api, ApiError } from '@/api'
import type { CodeOrder, Product, ProductShort } from '@/api'
import { isIsmptConnected, session } from '@/stores/session'
import { errorMessage, fieldErrors } from '@/utils/errors'
import Modal from './Modal.vue'
import Icon from './Icon.vue'
import ProductPicker from './ProductPicker.vue'
import IsmptConnect from './IsmptConnect.vue'

const props = withDefaults(defineProps<{ productId?: number | null }>(), { productId: null })
const emit = defineEmits<{ close: []; created: [order: CodeOrder] }>()

const productId = ref<number | null>(props.productId)
const product = ref<Product | null>(null)
const selected = ref<ProductShort | null>(null)
const quantity = ref<number | null>(100)
const errors = ref<Record<string, string>>({})
const error = ref('')
const busy = ref(false)

const connected = computed(() => isIsmptConnected())
const annulDays = computed(() => session.config?.annul_days ?? 30)

onMounted(async () => {
  if (props.productId) {
    try {
      const res = await api.product(props.productId)
      product.value = res.data
      selected.value = { id: res.data.id, gtin: res.data.gtin, name: res.data.name }
    } catch {
      productId.value = null
    }
  }
})

function onPick(p: Product | null) {
  product.value = p
}

async function submit() {
  errors.value = {}
  error.value = ''
  if (!productId.value) errors.value.product_id = 'Выберите товар'
  const q = Number(quantity.value)
  if (!Number.isInteger(q) || q < 1 || q > 10000) errors.value.quantity = 'Количество от 1 до 10 000'
  if (Object.keys(errors.value).length || !productId.value) return
  busy.value = true
  try {
    const res = await api.createOrder(productId.value, q)
    emit('created', res.data)
  } catch (e) {
    errors.value = fieldErrors(e)
    if (!Object.keys(errors.value).length) {
      error.value =
        e instanceof ApiError && e.status === 409 && !e.message
          ? 'Нет подключения к ИС МПТ или товар не найден в НКТ'
          : errorMessage(e)
    }
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <Modal title="Заказать коды маркировки" :width="560" @close="emit('close')">
    <form id="order-form" class="stack" novalidate @submit.prevent="submit">
      <div v-if="!connected" class="card-body subtle">
        <IsmptConnect compact />
      </div>
      <div class="field">
        <label for="o-product">Товар</label>
        <ProductPicker
          v-model="productId"
          input-id="o-product"
          :selected="selected"
          :invalid="!!errors.product_id"
          @pick="onPick"
        />
        <span v-if="errors.product_id" class="field-error">{{ errors.product_id }}</span>
        <span v-else-if="product && product.nkt_status !== 'found'" class="field-hint warn">
          Товар не подтверждён в НКТ. Сначала проверьте карточку на странице «Товары», иначе ИС МПТ откажет в заказе.
        </span>
      </div>
      <div class="field">
        <label for="o-qty">Количество кодов</label>
        <input
          id="o-qty"
          v-model.number="quantity"
          class="input"
          :class="{ invalid: errors.quantity }"
          type="number"
          min="1"
          max="10000"
          step="1"
          inputmode="numeric"
        />
        <span v-if="errors.quantity" class="field-error">{{ errors.quantity }}</span>
        <span v-else class="field-hint">
          Один код на одно изделие. Заказывайте под ближайшую партию: неиспользованные коды аннулируются через
          {{ annulDays }} дней.
        </span>
      </div>
      <div v-if="error" class="alert tone-danger" role="alert">
        <Icon name="alert" :size="16" />
        <span class="break">{{ error }}</span>
      </div>
    </form>
    <template #footer>
      <button class="btn" type="button" @click="emit('close')">Отмена</button>
      <button class="btn btn-primary" type="submit" form="order-form" :disabled="busy">
        <span v-if="busy" class="spinner" />
        Заказать
      </button>
    </template>
  </Modal>
</template>

<style scoped>
.subtle {
  background: var(--c-surface-2);
  border: 1px solid var(--c-border);
  border-radius: var(--radius-sm);
  padding: 12px;
}

.warn {
  color: var(--c-warning);
}
</style>
