<script setup lang="ts">
import { ref } from 'vue'
import { copyText } from '@/utils/clipboard'
import Icon from './Icon.vue'

const props = withDefaults(defineProps<{ text: string; wrap?: boolean; label?: string }>(), { wrap: false, label: '' })
const copied = ref(false)

async function copy() {
  copied.value = await copyText(props.text)
  setTimeout(() => (copied.value = false), 2000)
}
</script>

<template>
  <div class="copy-block">
    <div v-if="label" class="field-label">{{ label }}</div>
    <div class="copy-row">
      <code class="code-box" :class="{ wrap }">{{ text }}</code>
      <button class="btn btn-sm" type="button" @click="copy">
        <Icon :name="copied ? 'check' : 'copy'" :size="15" />
        {{ copied ? 'Скопировано' : 'Копировать' }}
      </button>
    </div>
  </div>
</template>

<style scoped>
.copy-block {
  display: flex;
  flex-direction: column;
  gap: 5px;
  min-width: 0;
}

.copy-row {
  display: flex;
  gap: 8px;
  align-items: flex-start;
  min-width: 0;
}

.copy-row .code-box {
  flex: 1;
  min-width: 0;
}

@media (max-width: 520px) {
  .copy-row {
    flex-direction: column;
    align-items: stretch;
  }
}
</style>
