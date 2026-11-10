<script setup>
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { api, formatTime } from '../api'
import { useSession } from '../stores/session'

const session = useSession()
const router = useRouter()
const displayName = ref('')
const email = ref('')
const currentPassword = ref('')
const nextPassword = ref('')
const error = ref('')
const notice = ref('')

onMounted(() => {
  displayName.value = session.profile?.display_name || ''
  email.value = session.profile?.email || ''
})

async function saveProfile() {
  error.value = ''
  notice.value = ''
  try {
    const result = (await api('/api/auth/profile/update', {
      display_name: displayName.value,
      email: email.value
    })).data
    session.profile = { ...session.profile, ...result }
    notice.value = '个人资料已保存'
  } catch (err) {
    error.value = err.message
  }
}

async function changePassword() {
  error.value = ''
  notice.value = ''
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
</script>

<template>
  <div class="page account-grid">
    <p v-if="error" class="notice">{{ error }}</p>
    <p v-if="notice" class="notice ok">{{ notice }}</p>
    <form class="card stack" @submit.prevent="saveProfile">
      <h2>个人资料</h2>
      <p class="muted">用户名 {{ session.profile?.username }} · {{ session.profile?.role === 'admin' ? '管理员' : '成员' }} · 注册于 {{ formatTime(session.profile?.created_at) }}</p>
      <label class="field"><span>显示名</span><input v-model="displayName" required /></label>
      <label class="field"><span>邮箱</span><input v-model="email" type="email" placeholder="可留空" /></label>
      <button class="btn btn-primary" type="submit">保存资料</button>
    </form>
    <form class="card stack" @submit.prevent="changePassword">
      <h2>修改密码</h2>
      <p class="muted">保存后当前会话会失效，需要重新登录。</p>
      <label class="field"><span>当前密码</span><input v-model="currentPassword" type="password" required /></label>
      <label class="field"><span>新密码</span><input v-model="nextPassword" type="password" minlength="8" required /></label>
      <button class="btn" type="submit">保存并重新登录</button>
    </form>
  </div>
</template>
