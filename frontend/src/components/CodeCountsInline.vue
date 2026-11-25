<script setup lang="ts">
import { computed } from 'vue'
import type { CodeCounts } from '@/api'
import { codeStatuses, codeStatusOrder } from '@/utils/status'
import { formatNumber } from '@/utils/format'

const props = withDefaults(defineProps<{ counts: CodeCounts | null | undefined; showZero?: boolean }>(), {
  showZero: false,
})

const items = computed(() =>
  codeStatusOrder
    .map((key) => ({ key, label: codeStatuses[key].label, tone: codeStatuses[key].tone, value: props.counts?.[key] ?? 0 }))
    .filter((i) => props.showZero || i.value > 0),
)
</script>

<template>
  <span v-if="items.length" class="counts">
    <span v-for="i in items" :key="i.key" class="badge plain" :class="`tone-${i.tone}`" :title="i.label">
      {{ i.label }}: {{ formatNumber(i.value) }}
    </span>
  </span>
  <span v-else class="muted small">Кодов нет</span>
</template>
