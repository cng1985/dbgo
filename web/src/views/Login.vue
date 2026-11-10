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
    <div class="login-brand">
      <div>
        <div class="mark">Db</div>
        <h1>把库表放回一张工作台。</h1>
        <p>查询、结构、迁移和备份共用同一套连接。口令只留在服务端，页面只保存会话。</p>
        <div class="login-points">
          <div><span>01</span> SQLite、MySQL、PostgreSQL、SQL Server</div>
          <div><span>02</span> 默认只读，写入需要明确打开</div>
          <div><span>03</span> 迁移和备份都能回到可核对的状态</div>
        </div>
      </div>
      <small>DBGo</small>
    </div>
    <div class="login-panel">
      <form class="login-card stack" @submit.prevent="submit">
        <div>
          <h2>登录</h2>
          <p class="muted">使用管理员账号进入控制台。</p>
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
        <button class="btn btn-primary btn-block" type="submit" :disabled="pending">进入控制台</button>
      </form>
    </div>
  </section>
</template>
