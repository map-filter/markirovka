<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { api } from '@/api'
import type { Product, ProductShort } from '@/api'

const props = withDefaults(
  defineProps<{ modelValue: number | null; selected?: ProductShort | null; invalid?: boolean; placeholder?: string; inputId?: string }>(),
  { selected: null, invalid: false, placeholder: 'Начните вводить название или GTIN', inputId: undefined },
)
const emit = defineEmits<{ 'update:modelValue': [value: number | null]; pick: [product: Product | null] }>()

const query = ref('')
const items = ref<Product[]>([])
const open = ref(false)
const loading = ref(false)
const active = ref(-1)
const root = ref<HTMLElement | null>(null)
let timer: ReturnType<typeof setTimeout> | null = null
let seq = 0

function label(p: ProductShort): string {
  return `${p.name} · ${p.gtin}`
}

watch(
  () => props.selected,
  (p) => {
    if (p && p.id === props.modelValue) query.value = label(p)
  },
  { immediate: true },
)

async function load(search: string) {
  const my = ++seq
  loading.value = true
  try {
    const res = await api.products({ search: search || undefined, per_page: 20 })
    if (my === seq) items.value = res.data
  } catch {
    if (my === seq) items.value = []
  } finally {
    if (my === seq) loading.value = false
  }
}

function onInput() {
  open.value = true
  active.value = -1
  if (props.modelValue !== null) {
    emit('update:modelValue', null)
    emit('pick', null)
  }
  if (timer) clearTimeout(timer)
  timer = setTimeout(() => load(query.value.trim()), 250)
}

function onFocus() {
  open.value = true
  if (!items.value.length) load(props.modelValue ? '' : query.value.trim())
}

function choose(p: Product) {
  query.value = label(p)
  open.value = false
  emit('update:modelValue', p.id)
  emit('pick', p)
}

function onKey(e: KeyboardEvent) {
  if (!open.value && (e.key === 'ArrowDown' || e.key === 'ArrowUp')) {
    open.value = true
    return
  }
  if (e.key === 'ArrowDown') {
    e.preventDefault()
    active.value = Math.min(items.value.length - 1, active.value + 1)
  } else if (e.key === 'ArrowUp') {
    e.preventDefault()
    active.value = Math.max(0, active.value - 1)
  } else if (e.key === 'Enter' && open.value && active.value >= 0) {
    e.preventDefault()
    choose(items.value[active.value])
  } else if (e.key === 'Escape') {
    open.value = false
  }
}

function onDocClick(e: MouseEvent) {
  if (root.value && !root.value.contains(e.target as Node)) open.value = false
}

onMounted(() => document.addEventListener('mousedown', onDocClick))
onBeforeUnmount(() => {
  document.removeEventListener('mousedown', onDocClick)
  if (timer) clearTimeout(timer)
})
</script>

<template>
  <div ref="root" class="picker">
    <input
      :id="inputId"
      v-model="query"
      class="input"
      :class="{ invalid }"
      type="search"
      autocomplete="off"
      role="combobox"
      :aria-expanded="open"
      :placeholder="placeholder"
      @input="onInput"
      @focus="onFocus"
      @keydown="onKey"
    />
    <div v-if="open" class="dropdown" role="listbox">
      <div v-if="loading && !items.length" class="hint">Ищу товары...</div>
      <div v-else-if="!items.length" class="hint">Ничего не найдено</div>
      <button
        v-for="(p, i) in items"
        :key="p.id"
        type="button"
        class="option"
        :class="{ active: i === active, selected: p.id === modelValue }"
        role="option"
        @mousedown.prevent="choose(p)"
      >
        <span class="name break">{{ p.name }}</span>
        <span class="meta">
          <span class="mono">{{ p.gtin }}</span>
          <span v-if="p.size">· {{ p.size }}</span>
          <span v-if="p.nkt_status !== 'found'" class="warn">· не подтверждён в НКТ</span>
        </span>
      </button>
    </div>
  </div>
</template>

<style scoped>
.picker {
  position: relative;
  min-width: 0;
}

.dropdown {
  position: absolute;
  z-index: 30;
  left: 0;
  right: 0;
  top: calc(100% + 4px);
  background: var(--c-surface);
  border: 1px solid var(--c-border-strong);
  border-radius: var(--radius-sm);
  box-shadow: var(--shadow-lg);
  max-height: 280px;
  overflow-y: auto;
}

.hint {
  padding: 10px 12px;
  color: var(--c-muted);
  font-size: 14px;
}

.option {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 2px;
  width: 100%;
  padding: 8px 12px;
  border: 0;
  border-bottom: 1px solid var(--c-border);
  background: none;
  text-align: left;
  cursor: pointer;
}

.option:last-child {
  border-bottom: 0;
}

.option.active,
.option:hover {
  background: var(--c-accent-soft);
}

.option.selected .name {
  color: var(--c-accent-text);
}

.name {
  font-weight: 600;
  font-size: 14px;
}

.meta {
  font-size: 12.5px;
  color: var(--c-muted);
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}

.warn {
  color: var(--c-warning);
}
</style>
