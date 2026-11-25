<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import type { LabelFields, LabelSize } from '@/api'
import type { LabelSample } from '@/utils/label'
import DataMatrix from './DataMatrix.vue'

const props = withDefaults(
  defineProps<{ size: LabelSize; fields: LabelFields; fontScale: number; sample: LabelSample; maxPxPerMm?: number }>(),
  { maxPxPerMm: 7 },
)

const PT = 0.3528

const host = ref<HTMLElement | null>(null)
const hostWidth = ref(360)
let observer: ResizeObserver | null = null

onMounted(() => {
  if (!host.value) return
  hostWidth.value = host.value.clientWidth || 360
  observer = new ResizeObserver((entries) => {
    const w = entries[0]?.contentRect.width
    if (w) hostWidth.value = w
  })
  observer.observe(host.value)
})
onBeforeUnmount(() => observer?.disconnect())

const geometry = computed(() =>
  props.size === '58x40'
    ? { w: 58, h: 40, margin: 2, dm: 22, gap: 2, namePt: 8, textPt: 6.5, smallPt: 5.5, serialPt: 5, nameLines: 3 }
    : { w: 43, h: 25, margin: 1.5, dm: 17, gap: 1.5, namePt: 6.5, textPt: 5.5, smallPt: 5, serialPt: 4.5, nameLines: 2 },
)

const k = computed(() => {
  const g = geometry.value
  const fit = Math.max(2, (hostWidth.value - 2) / g.w)
  return Math.min(props.maxPxPerMm, fit)
})

const scale = computed(() => Math.min(1.4, Math.max(0.7, props.fontScale || 1)))

function px(mm: number): string {
  return `${(mm * k.value).toFixed(2)}px`
}

function font(pt: number): string {
  return px(pt * PT * scale.value)
}

const showDm = computed(() => props.fields.datamatrix)
const isLarge = computed(() => props.size === '58x40')

const labelStyle = computed(() => ({ width: px(geometry.value.w), height: px(geometry.value.h) }))

const dmStyle = computed(() => {
  const g = geometry.value
  return { left: px(g.margin), top: px(g.margin), width: px(g.dm), height: px(g.dm) }
})

const serialStyle = computed(() => {
  const g = geometry.value
  return {
    left: px(g.margin),
    top: px(g.margin + g.dm + 0.6),
    width: px(g.dm),
    fontSize: font(g.serialPt),
  }
})

const textStyle = computed(() => {
  const g = geometry.value
  const left = showDm.value ? g.margin + g.dm + g.gap : g.margin
  return {
    left: px(left),
    top: px(g.margin),
    width: px(g.w - left - g.margin),
    height: px(g.h - g.margin * 2),
    fontSize: font(g.textPt),
    gap: px(0.8),
  }
})

const nameStyle = computed(() => ({
  fontSize: font(geometry.value.namePt),
  WebkitLineClamp: String(geometry.value.nameLines),
}))

const smallStyle = computed(() => ({ fontSize: font(geometry.value.smallPt) }))
</script>

<template>
  <div ref="host" class="preview-host">
    <div class="label" :style="labelStyle" :aria-label="`Предпросмотр этикетки ${size} мм`">
      <div v-if="showDm" class="dm-box" :style="dmStyle">
        <DataMatrix :hr="sample.codeHr" :raw="sample.codeRaw" />
      </div>
      <div v-if="showDm && fields.serial" class="serial" :style="serialStyle">{{ sample.serial }}</div>
      <div class="text" :style="textStyle">
        <div v-if="fields.name" class="name" :style="nameStyle">{{ sample.name }}</div>
        <div v-if="fields.size && sample.size" class="line">Размер: {{ sample.size }}</div>
        <div v-if="isLarge && fields.composition && sample.composition" class="line clamp2">Состав: {{ sample.composition }}</div>
        <div v-if="fields.gtin" class="line">GTIN {{ sample.gtin }}</div>
        <div v-if="!showDm && fields.serial" class="line mono-line">{{ sample.serial }}</div>
        <div v-if="isLarge && fields.manufacturer && sample.manufacturer" class="maker" :style="smallStyle">
          {{ sample.manufacturer }}
        </div>
      </div>
    </div>
    <div class="caption">{{ geometry.w }} x {{ geometry.h }} мм, масштаб {{ Math.round((k / 3.78) * 100) }}%</div>
  </div>
</template>

<style scoped>
.preview-host {
  width: 100%;
  min-width: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
}

.label {
  position: relative;
  background: #fff;
  border-radius: 3px;
  box-shadow: 0 0 0 1px #cfd6d5, 0 6px 18px rgba(16, 40, 38, 0.12);
  color: #000;
  overflow: hidden;
  font-family: Arial, 'Helvetica Neue', Helvetica, sans-serif;
  flex: none;
}

.dm-box {
  position: absolute;
}

.serial {
  position: absolute;
  font-family: var(--mono);
  line-height: 1.1;
  text-align: center;
  overflow-wrap: anywhere;
  white-space: normal;
}

.text {
  position: absolute;
  display: flex;
  flex-direction: column;
  line-height: 1.18;
  overflow: hidden;
  min-width: 0;
}

.name {
  font-weight: 700;
  display: -webkit-box;
  -webkit-box-orient: vertical;
  overflow: hidden;
  overflow-wrap: anywhere;
}

.line {
  overflow-wrap: anywhere;
}

.clamp2 {
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.mono-line {
  font-family: var(--mono);
}

.maker {
  margin-top: auto;
  color: #222;
  overflow-wrap: anywhere;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.caption {
  font-size: 12.5px;
  color: var(--c-muted);
}
</style>
