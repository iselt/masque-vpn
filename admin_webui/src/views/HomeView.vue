<template>
  <div class="home-container">
    <el-card class="box-card">
      <template #header>
        <div class="card-header">
          <span>{{ t('navigation.home') }}</span>
        </div>
      </template>
      <div class="welcome-message">
        <h2>{{ t('header.welcome', { user: userStore.username || 'User' }) }}</h2>
        <p>{{ t('appName') }} - {{ t('home.tagline') }}</p>
      </div>

      <el-row :gutter="20" class="quick-access">
        <el-col :span="6">
          <el-card shadow="hover" class="quick-link-card" @click="navigateTo('/server-list')">
            <el-icon :size="30"><Tickets /></el-icon>
            <p>{{ t('navigation.clientManagement') }}</p>
          </el-card>
        </el-col>
        <el-col :span="6">
          <el-card shadow="hover" class="quick-link-card" @click="navigateTo('/groups')">
            <el-icon :size="30"><User /></el-icon>
            <p>{{ t('navigation.groupManagement') }}</p>
          </el-card>
        </el-col>
        <el-col :span="6">
          <el-card shadow="hover" class="quick-link-card" @click="navigateTo('/policies')">
            <el-icon :size="30"><Lock /></el-icon>
            <p>{{ t('navigation.policyManagement') }}</p>
          </el-card>
        </el-col>
        <el-col :span="6">
          <el-card shadow="hover" class="quick-link-card" @click="navigateTo('/settings')">
            <el-icon :size="30"><Setting /></el-icon>
            <p>{{ t('navigation.serverSettings') }}</p>
          </el-card>
        </el-col>
      </el-row>
    </el-card>
  </div>
</template>

<script lang="ts" setup>
import { useI18n } from 'vue-i18n'
import { useUserStore } from '@/store/user'
import { useRouter } from 'vue-router'
import { Tickets, User, Lock, Setting } from '@element-plus/icons-vue'

const { t } = useI18n()
const userStore = useUserStore()
const router = useRouter()

// Add a tagline to your language files, e.g., in en.json:
// "home": {
//   "tagline": "Secure and manage your network with ease."
// }
// And in zh.json:
// "home": {
//   "tagline": "轻松保护和管理您的网络。"
// }
// If not present, it will just show "MASQUE VPN - "

const navigateTo = (path: string) => {
  router.push(path)
}
</script>

<style scoped>
.home-container {
  padding: 20px;
}
.card-header span {
  font-size: 18px;
  font-weight: bold;
}
.welcome-message {
  text-align: center;
  margin-bottom: 30px;
}
.welcome-message h2 {
  font-size: 24px;
  margin-bottom: 10px;
}
.welcome-message p {
  font-size: 16px;
  color: #606266;
}
.quick-access {
  margin-top: 20px;
}
.quick-link-card {
  cursor: pointer;
  text-align: center;
  padding: 20px;
}
/* .quick-link-card .el-icon {
  margin-bottom: 10px;
  color: #409EFF;
} */
/* .quick-link-card p {
  font-size: 14px;
  color: #303133 !important;
} */
</style>
