<script setup lang="ts">
withDefaults(defineProps<{ size?: number }>(), { size: 30 })
</script>

<template>
  <svg :width="size" :height="size" viewBox="0 0 10 10" shape-rendering="crispEdges" aria-hidden="true">
    <rect width="10" height="10" rx="1.6" fill="#0f5c5c" />
    <g fill="#fff">
      <rect x="1" y="1" width="1" height="8" />
      <rect x="1" y="8" width="8" height="1" />
      <rect x="3" y="1" width="1" height="1" />
      <rect x="5" y="1" width="1" height="1" />
      <rect x="7" y="1" width="1" height="1" />
      <rect x="8" y="2" width="1" height="1" />
      <rect x="8" y="4" width="1" height="1" />
      <rect x="8" y="6" width="1" height="1" />
      <rect x="3" y="3" width="2" height="1" />
      <rect x="6" y="3" width="1" height="2" />
      <rect x="3" y="5" width="1" height="2" />
      <rect x="5" y="6" width="2" height="1" />
    </g>
  </svg>
</template>
