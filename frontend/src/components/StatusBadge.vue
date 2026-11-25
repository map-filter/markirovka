<script setup lang="ts">
import { computed } from 'vue'
import { statusOf, type StatusInfo } from '@/utils/status'

const props = defineProps<{ map: Record<string, StatusInfo>; value: string | null | undefined }>()
const info = computed(() => statusOf(props.map, props.value))
</script>

<template>
  <span class="badge" :class="`tone-${info.tone}`">{{ info.label }}</span>
</template>
