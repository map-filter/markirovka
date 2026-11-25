<script setup lang="ts">
import { computed } from 'vue'
import { renderDataMatrix } from '@/utils/datamatrix'

const props = defineProps<{ hr: string; raw?: string | null }>()

const svg = computed(() => {
  try {
    return { ok: true, html: renderDataMatrix(props.hr, props.raw) }
  } catch {
    return { ok: false, html: '' }
  }
})
</script>

<template>
  <div v-if="svg.ok" class="dm" role="img" aria-label="Код Data Matrix" v-html="svg.html" />
  <div v-else class="dm dm-error">Код не удалось нарисовать</div>
</template>

<style scoped>
.dm {
  width: 100%;
  height: 100%;
  line-height: 0;
}

.dm :deep(svg) {
  width: 100%;
  height: 100%;
  display: block;
}

.dm-error {
  display: flex;
  align-items: center;
  justify-content: center;
  text-align: center;
  font-size: 10px;
  line-height: 1.2;
  color: var(--c-danger);
  border: 1px dashed var(--c-danger);
}
</style>
