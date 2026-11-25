<script setup lang="ts">
import type { PageMeta } from '@/api'
import { formatNumber } from '@/utils/format'

defineProps<{ meta: PageMeta | null }>()
const emit = defineEmits<{ change: [page: number] }>()
</script>

<template>
  <div v-if="meta && meta.last_page > 1" class="pager">
    <span class="muted small">Всего: {{ formatNumber(meta.total) }}</span>
    <div class="row">
      <button class="btn btn-sm" :disabled="meta.current_page <= 1" @click="emit('change', meta.current_page - 1)">
        Назад
      </button>
      <span class="small">{{ meta.current_page }} из {{ meta.last_page }}</span>
      <button
        class="btn btn-sm"
        :disabled="meta.current_page >= meta.last_page"
        @click="emit('change', meta.current_page + 1)"
      >
        Далее
      </button>
    </div>
  </div>
  <div v-else-if="meta && meta.total > 0" class="pager">
    <span class="muted small">Всего: {{ formatNumber(meta.total) }}</span>
  </div>
</template>

<style scoped>
.pager {
  display: flex;
  justify-content: space-between;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
  padding: 12px 16px;
  border-top: 1px solid var(--c-border);
}
</style>
