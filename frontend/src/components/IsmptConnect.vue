<script setup lang="ts">
import { computed, ref } from 'vue'
import { connectIsmpt, isIsmptConnected, session } from '@/stores/session'
import { formatUntil } from '@/utils/format'
import { errorMessage } from '@/utils/errors'
import { notify } from '@/stores/toast'
import Icon from './Icon.vue'

withDefaults(defineProps<{ compact?: boolean }>(), { compact: false })
const emit = defineEmits<{ connected: [] }>()

const error = ref('')
const connected = computed(() => isIsmptConnected())
const until = computed(() => formatUntil(session.ismpt?.expires_at))
const signHint = computed(() =>
  session.config?.sign_mode === 'emulate'
    ? 'Подпись выполнится автоматически в режиме эмуляции.'
    : 'Откроется окно NCALayer: выберите ключ ЭЦП организации (файл .p12 или токен) и введите пароль.',
)

async function connect() {
  error.value = ''
  try {
    await connectIsmpt()
    notify('Подключение к ИС МПТ установлено')
    emit('connected')
  } catch (e) {
    error.value = errorMessage(e)
  }
}
</script>

<template>
  <div class="ismpt" :class="{ compact }">
    <div v-if="connected" class="state ok">
      <span class="dot" />
      <div class="txt">
        <strong>ИС МПТ подключена</strong>
        <span class="muted small">до {{ until }}<template v-if="session.ismpt?.mode === 'emulator'">, эмулятор</template></span>
      </div>
      <button class="btn btn-sm btn-ghost" type="button" :disabled="session.connecting" @click="connect">
        <span v-if="session.connecting" class="spinner" />
        Продлить
      </button>
    </div>
    <div v-else class="stack">
      <div class="state off">
        <span class="dot" />
        <div class="txt">
          <strong>ИС МПТ не подключена</strong>
          <span class="muted small">Без подключения нельзя заказать коды и отправить отчёты. Подключение действует 10 часов.</span>
        </div>
      </div>
      <div class="row">
        <button class="btn btn-primary" type="button" :disabled="session.connecting" @click="connect">
          <span v-if="session.connecting" class="spinner" />
          <Icon v-else name="key" :size="16" />
          {{ session.connecting ? 'Подписываю...' : 'Подключить ИС МПТ' }}
        </button>
      </div>
      <p v-if="!compact" class="muted small">{{ signHint }}</p>
    </div>
    <div v-if="error" class="alert tone-danger" role="alert">
      <Icon name="alert" :size="16" />
      <span class="break">{{ error }}</span>
    </div>
  </div>
</template>

<style scoped>
.ismpt {
  display: flex;
  flex-direction: column;
  gap: 10px;
  min-width: 0;
}

.state {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
  min-width: 0;
}

.txt {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-width: 160px;
}

.dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  flex: none;
}

.ok .dot {
  background: var(--c-success);
  box-shadow: 0 0 0 4px var(--c-success-soft);
}

.off .dot {
  background: var(--c-warning);
  box-shadow: 0 0 0 4px var(--c-warning-soft);
}
</style>
