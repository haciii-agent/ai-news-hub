<template>
  <div class="admin-view">
    <div class="page-title-bar">
      <h1 class="page-title">⚙️ 管理后台</h1>
    </div>
    <div class="admin-toolbar">
      <n-input v-model:value="search" placeholder="搜索用户名或邮箱" clearable style="max-width:300px" />
      <n-button @click="loadUsers">刷新</n-button>
    </div>
    <n-spin :show="loading">
      <n-data-table :columns="columns" :data="users" :bordered="false" :pagination="{ pageSize: 20 }" />
    </n-spin>

    <n-modal v-model:show="detailVisible" preset="card" title="用户详情" style="max-width:600px">
      <n-descriptions v-if="detail" :column="2" bordered>
        <n-descriptions-item label="ID">{{ detail.id }}</n-descriptions-item>
        <n-descriptions-item label="用户名">{{ detail.username }}</n-descriptions-item>
        <n-descriptions-item label="邮箱">{{ detail.email }}</n-descriptions-item>
        <n-descriptions-item label="角色">{{ detail.role }}</n-descriptions-item>
        <n-descriptions-item label="注册时间">{{ detail.created_at }}</n-descriptions-item>
        <n-descriptions-item label="最后活跃">{{ detail.last_seen_at }}</n-descriptions-item>
      </n-descriptions>
    </n-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, h } from 'vue'
import { NButton, NTag, useMessage } from 'naive-ui'
import { getAdminUsers, updateUserRole, updateUserStatus } from '../api'

const msg = useMessage()
const loading = ref(false)
const users = ref<any[]>([])
const search = ref('')
const detail = ref<any>(null)
const detailVisible = ref(false)

const columns = [
  { title: 'ID', key: 'id', width: 80 },
  { title: '用户名', key: 'username' },
  { title: '邮箱', key: 'email', ellipsis: true },
  {
    title: '角色', key: 'role', render: (row: any) =>
      h('select', {
        value: row.role, onChange: (e: any) => changeRole(row, e.target.value),
        style: 'padding:2px 6px;border-radius:4px;border:1px solid #ddd'
      }, ['viewer', 'editor', 'admin', 'anonymous'].map(r => h('option', { value: r }, r)))
  },
  {
    title: '状态', key: 'disabled', render: (row: any) =>
      h(NTag, { type: row.disabled ? 'error' : 'success', size: 'small' },
        () => row.disabled ? '已禁用' : '正常')
  },
  {
    title: '操作', key: 'actions', render: (row: any) =>
      h('div', { style: 'display:flex;gap:8px' }, [
        h(NButton, { size: 'small', onClick: () => showDetail(row) }, () => '详情'),
        h(NButton, { size: 'small', onClick: () => toggleStatus(row) },
          () => row.disabled ? '启用' : '禁用'),
      ])
  },
]

async function loadUsers() {
  loading.value = true
  try {
    const q = search.value ? '&q=' + encodeURIComponent(search.value) : ''
    users.value = await getAdminUsers(q)
  } finally { loading.value = false }
}

async function changeRole(row: any, role: string) {
  await updateUserRole(row.id, role)
  row.role = role
  msg.success('角色已更新')
}

async function toggleStatus(row: any) {
  await updateUserStatus(row.id, !row.disabled)
  row.disabled = !row.disabled
  msg.success(row.disabled ? '已禁用' : '已启用')
}

function showDetail(row: any) { detail.value = row; detailVisible.value = true }

onMounted(loadUsers)
</script>

<style scoped>
.admin-toolbar { display: flex; gap: 12px; align-items: center; margin-bottom: 16px; }
</style>
