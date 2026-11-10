<script setup>
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api } from '../api'
import { useSession } from '../stores/session'

const session = useSession()
const route = useRoute()
const router = useRouter()
const passwordOpen = ref(false)
const currentPassword = ref('')
const nextPassword = ref('')
const message = ref('')
const error = ref('')
const links = [
  { to: '/', title: '总览', hint: '概况' },
  { to: '/connections', title: '连接', hint: '数据源' },
  { to: '/query', title: '查询', hint: 'SQL' },
  { to: '/catalog', title: '结构', hint: '目录' },
  { to: '/migrations', title: '迁移', hint: '版本' },
  { to: '/backups', title: '备份', hint: '恢复' }
]

const title = computed(() => route.meta.title || '总览')

function active(path) {
  return path === '/' ? route.path === '/' : route.path.startsWith(path)
}

async function logout() {
  await session.logout()
  await router.push('/login')
}

async function changePassword() {
  error.value = ''
  message.value = ''
  try {
    await api('/api/auth/password', {
      current_password: currentPassword.value,
      new_password: nextPassword.value
    })
    session.profile = null
    await router.push('/login')
  } catch (err) {
    error.value = err.message
  }
}

function onUnauthorized() {
  session.profile = null
  if (route.path !== '/login') {
    router.push('/login')
  }
}

onMounted(() => window.addEventListener('dbgo-unauthorized', onUnauthorized))
onBeforeUnmount(() => window.removeEventListener('dbgo-unauthorized', onUnauthorized))
</script>

<template>
  <div class="shell">
    <aside class="side">
      <div class="brand">
        <div class="mark">Db</div>
        <div>
          <strong>DBGo</strong>
          <small>数据库控制台</small>
        </div>
      </div>
      <nav class="nav">
        <router-link v-for="link in links" :key="link.to" :to="link.to" :class="{ active: active(link.to) }">
          {{ link.title }}
          <small>{{ link.hint }}</small>
        </router-link>
      </nav>
      <div class="side-user">
        <div>{{ session.profile?.display_name || session.profile?.username }}</div>
        <div class="actions">
          <button class="btn" type="button" @click="passwordOpen = !passwordOpen">修改密码</button>
          <button class="btn" type="button" @click="logout">退出</button>
        </div>
      </div>
    </aside>
    <section class="main">
      <header class="topbar">
        <div>
          <p class="muted">控制台</p>
          <h1>{{ title }}</h1>
        </div>
        <div class="actions">
          <button class="btn" type="button" @click="logout">退出</button>
        </div>
      </header>
      <div v-if="passwordOpen" class="page">
        <form class="card stack" @submit.prevent="changePassword">
          <h2>修改密码</h2>
          <p class="muted">保存后当前会话会失效，需要重新登录。</p>
          <p v-if="error" class="notice">{{ error }}</p>
          <p v-if="message" class="notice ok">{{ message }}</p>
          <label class="field"><span>当前密码</span><input v-model="currentPassword" type="password" required /></label>
          <label class="field"><span>新密码</span><input v-model="nextPassword" type="password" minlength="8" required /></label>
          <button class="btn btn-primary" type="submit">保存并重新登录</button>
        </form>
      </div>
      <router-view v-else />
    </section>
  </div>
</template>
