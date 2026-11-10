import { createRouter, createWebHistory } from 'vue-router'
import { useSession } from './stores/session'
import Login from './views/Login.vue'
import Shell from './views/Shell.vue'

const outlet = { name: 'RouteOutlet', render: () => null }

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/login', component: Login, meta: { public: true } },
    {
      path: '/',
      component: Shell,
      children: [
        { path: '', component: outlet, meta: { title: 'SQL 编辑器', tool: 'script' } },
        { path: 'overview', component: outlet, meta: { title: '总览', tool: 'overview' } },
        { path: 'connections', component: outlet, meta: { title: '连接', tool: 'connections' } },
        { path: 'query', component: outlet, meta: { title: 'SQL 编辑器', tool: 'script' } },
        { path: 'catalog', component: outlet, meta: { title: '结构', tool: 'catalog' } },
        { path: 'migrations', component: outlet, meta: { title: '迁移', tool: 'migrations' } },
        { path: 'backups', component: outlet, meta: { title: '备份', tool: 'backups' } }
      ]
    }
  ]
})

router.beforeEach(async (to) => {
  const session = useSession()
  if (!session.ready) {
    await session.load()
  }
  if (!to.meta.public && !session.profile) {
    return '/login'
  }
  if (to.path === '/login' && session.profile) {
    return '/'
  }
  return true
})

export default router
