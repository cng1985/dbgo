<script setup>
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useSession } from '../stores/session'

const session = useSession()
const router = useRouter()
const username = ref('admin')
const password = ref('')
const error = ref('')
const pending = ref(false)

async function submit() {
  error.value = ''
  pending.value = true
  try {
    await session.login(username.value.trim(), password.value)
    await router.push('/')
  } catch (err) {
    error.value = err.message || '登录失败'
  } finally {
    pending.value = false
  }
}
</script>

<template>
  <section class="login">
    <form class="login-card stack" @submit.prevent="submit">
      <div class="login-head">
        <div class="mark">Db</div>
        <div>
          <h1>DBGo</h1>
          <p class="muted">数据库工作台</p>
        </div>
      </div>
      <p v-if="error" class="notice">{{ error }}</p>
      <label class="field">
        <span>用户名</span>
        <input v-model="username" autocomplete="username" required />
      </label>
      <label class="field">
        <span>密码</span>
        <input v-model="password" type="password" autocomplete="current-password" required />
      </label>
      <button class="btn btn-primary btn-block" type="submit" :disabled="pending">登录</button>
      <p class="muted login-foot">SQLite、MySQL、PostgreSQL、SQL Server。查询默认只读。</p>
    </form>
  </section>
</template>
