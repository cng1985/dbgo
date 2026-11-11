<script setup>
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '../api'
import { useSession } from '../stores/session'

const session = useSession()
const router = useRouter()
const mode = ref('login')
const username = ref('admin')
const password = ref('')
const displayName = ref('')
const email = ref('')
const error = ref('')
const pending = ref(false)
const allowRegister = ref(true)

onMounted(async () => {
  try {
    const result = await api('/api/auth/options', {})
    allowRegister.value = result.data?.allow_register !== false
  } catch {
    allowRegister.value = true
  }
})

function switchMode(next) {
  mode.value = next
  error.value = ''
  if (next === 'register' && username.value === 'admin') {
    username.value = ''
  }
  if (next === 'login' && !username.value) {
    username.value = 'admin'
  }
}

async function submit() {
  error.value = ''
  pending.value = true
  try {
    if (mode.value === 'register') {
      const result = await api('/api/auth/register', {
        username: username.value.trim(),
        password: password.value,
        display_name: displayName.value.trim(),
        email: email.value.trim()
      })
      session.profile = result.data
      session.ready = true
    } else {
      await session.login(username.value.trim(), password.value)
    }
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
      <label v-if="mode === 'register'" class="field">
        <span>显示名</span>
        <input v-model="displayName" autocomplete="name" />
      </label>
      <label v-if="mode === 'register'" class="field">
        <span>邮箱</span>
        <input v-model="email" type="email" autocomplete="email" />
      </label>
      <label class="field">
        <span>密码</span>
        <input v-model="password" type="password" :autocomplete="mode === 'register' ? 'new-password' : 'current-password'" minlength="8" required />
      </label>
      <button class="btn btn-primary btn-block" type="submit" :disabled="pending">{{ mode === 'register' ? '注册并进入' : '登录' }}</button>
      <button v-if="allowRegister && mode === 'login'" class="btn btn-block" type="button" @click="switchMode('register')">注册账号</button>
      <button v-if="mode === 'register'" class="btn btn-block" type="button" @click="switchMode('login')">已有账号，去登录</button>
      <p class="muted login-foot">支持 MySQL、SQLite、PostgreSQL、SQL Server。每个账号只能看到自己创建的连接。</p>
    </form>
  </section>
</template>
