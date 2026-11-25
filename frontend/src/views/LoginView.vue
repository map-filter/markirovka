<script setup lang="ts">
import { reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { login } from '@/stores/session'
import { errorMessage, fieldErrors } from '@/utils/errors'
import Logo from '@/components/Logo.vue'
import Icon from '@/components/Icon.vue'

const router = useRouter()
const route = useRoute()

const form = reactive({ email: '', password: '' })
const errors = ref<Record<string, string>>({})
const error = ref('')
const busy = ref(false)

async function submit() {
  errors.value = {}
  error.value = ''
  if (!form.email.trim()) errors.value.email = 'Укажите email'
  if (!form.password) errors.value.password = 'Укажите пароль'
  if (Object.keys(errors.value).length) return
  busy.value = true
  try {
    await login(form.email.trim(), form.password)
    const redirect = typeof route.query.redirect === 'string' && route.query.redirect.startsWith('/') ? route.query.redirect : '/'
    await router.replace(redirect)
  } catch (e) {
    errors.value = fieldErrors(e)
    if (!Object.keys(errors.value).length) error.value = errorMessage(e)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div class="login-page">
    <header class="login-header">
      <Logo :size="32" />
      <strong>Маркировка за день</strong>
    </header>
    <main class="login-main">
      <form class="card login-card" novalidate @submit.prevent="submit">
        <div class="stack">
          <div>
            <h1>Вход в кабинет</h1>
            <p class="text-2 small promise">Первая партия этикеток за день: от карточки товара до ввода в оборот.</p>
          </div>
          <div class="field">
            <label for="email">Email</label>
            <input
              id="email"
              v-model="form.email"
              class="input"
              :class="{ invalid: errors.email }"
              type="email"
              autocomplete="username"
              inputmode="email"
              autofocus
            />
            <span v-if="errors.email" class="field-error">{{ errors.email }}</span>
          </div>
          <div class="field">
            <label for="password">Пароль</label>
            <input
              id="password"
              v-model="form.password"
              class="input"
              :class="{ invalid: errors.password }"
              type="password"
              autocomplete="current-password"
            />
            <span v-if="errors.password" class="field-error">{{ errors.password }}</span>
          </div>
          <div v-if="error" class="alert tone-danger" role="alert">
            <Icon name="alert" :size="16" />
            <span class="break">{{ error }}</span>
          </div>
          <button class="btn btn-primary submit" type="submit" :disabled="busy">
            <span v-if="busy" class="spinner" />
            Войти
          </button>
        </div>
      </form>
    </main>
  </div>
</template>

<style scoped>
.login-page {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
  background: linear-gradient(180deg, #e8f1f0 0%, var(--c-bg) 320px);
}

.login-header {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 18px 24px;
  font-size: 17px;
  color: var(--c-accent-text);
}

.login-main {
  flex: 1;
  display: flex;
  align-items: flex-start;
  justify-content: center;
  padding: 6vh 16px 40px;
}

.login-card {
  width: 100%;
  max-width: 400px;
  padding: 26px;
}

.promise {
  margin-top: 6px;
}

.submit {
  width: 100%;
  min-height: 42px;
}

@media (max-width: 480px) {
  .login-header {
    padding: 14px 16px;
  }

  .login-card {
    padding: 20px 16px;
  }
}
</style>
