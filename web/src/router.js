import { createRouter, createWebHistory } from 'vue-router'
import { useSession } from './stores/session'
import Login from './views/Login.vue'
import Shell from './views/Shell.vue'
import Overview from './views/Overview.vue'
import Connections from './views/Connections.vue'
import Query from './views/Query.vue'
import Catalog from './views/Catalog.vue'
import Migrations from './views/Migrations.vue'
import Backups from './views/Backups.vue'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/login', component: Login, meta: { public: true } },
    {
      path: '/',
      component: Shell,
      children: [
        { path: '', component: Overview, meta: { title: '总览' } },
        { path: 'connections', component: Connections, meta: { title: '连接' } },
        { path: 'query', component: Query, meta: { title: '查询' } },
        { path: 'catalog', component: Catalog, meta: { title: '结构' } },
        { path: 'migrations', component: Migrations, meta: { title: '迁移' } },
        { path: 'backups', component: Backups, meta: { title: '备份' } }
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
