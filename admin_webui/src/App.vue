<script setup lang="ts">
import { ref, onMounted, computed, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useUserStore } from './store/user'
import {
  Setting, House, Tickets, InfoFilled, User, Lock, 
  Expand, Fold, ArrowDown, Operation
} from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox, ElConfigProvider } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { elementPlusLocales } from './i18n'

const router = useRouter()
const userStore = useUserStore()
const isCollapse = ref(false)
const { t, locale } = useI18n()

const toggleSidebar = () => {
  isCollapse.value = !isCollapse.value
}

const currentLanguage = ref(locale.value)

const handleLanguageChange = (lang: string) => {
  if (locale.value !== lang) {
    locale.value = lang
    localStorage.setItem('preferredLanguage', lang)
    currentLanguage.value = lang
  }
}

const handleCommand = (command: string) => {
  if (command === 'logout') {
    ElMessageBox.confirm(t('header.confirmLogoutMessage'), t('header.confirmLogoutTitle'), {
      confirmButtonText: t('actions.ok'),
      cancelButtonText: t('actions.cancel'),
      type: 'warning',
    }).then(() => {
      userStore.logout()
      ElMessage.success(t('header.logoutSuccess'))
      router.push('/login')
    }).catch(() => {
      // Catch cancellation
    });
  } else if (command === 'lang-en') {
    handleLanguageChange('en')
  } else if (command === 'lang-zh') {
    handleLanguageChange('zh')
  }
}

onMounted(() => {
  userStore.checkAuthStatus()
  currentLanguage.value = locale.value
})

const elLocale = computed(() => elementPlusLocales[locale.value as 'en' | 'zh'] || elementPlusLocales.en)

watch(locale, (newLocale) => {
  currentLanguage.value = newLocale
})
</script>

<template>
  <el-config-provider :locale="elLocale">
    <el-container class="app-container">
      <el-aside width="200px" class="sidebar" v-if="userStore.isLoggedIn">
        <el-menu
          :default-active="$route.path"
          class="el-menu-vertical-demo"
          router
          :collapse="isCollapse"
        >
          <div class="logo-container">
            <img src="@/assets/vpn.svg" alt="Logo" class="logo-img" v-if="!isCollapse"/>
            <span v-if="!isCollapse" class="system-title">{{ t('appName') }}</span>
          </div>
          <el-menu-item index="/">
            <el-icon><House /></el-icon>
            <span>{{ t('navigation.home') }}</span>
          </el-menu-item>
          <el-menu-item index="/server-list">
            <el-icon><Tickets /></el-icon>
            <span>{{ t('navigation.clientManagement') }}</span>
          </el-menu-item>
          <el-menu-item index="/groups">
            <el-icon><User /></el-icon>
            <span>{{ t('navigation.groupManagement') }}</span>
          </el-menu-item>
          <el-menu-item index="/policies">
            <el-icon><Lock /></el-icon>
            <span>{{ t('navigation.policyManagement') }}</span>
          </el-menu-item>
          <el-menu-item index="/settings">
            <el-icon><Setting /></el-icon>
            <span>{{ t('navigation.serverSettings') }}</span>
          </el-menu-item>
          <el-menu-item index="/about">
            <el-icon><InfoFilled /></el-icon>
            <span>{{ t('navigation.about') }}</span>
          </el-menu-item>
        </el-menu>
      </el-aside>

      <el-container class="main-column-container">
        <el-header class="app-header" v-if="userStore.isLoggedIn">
          <div class="header-left">
            <el-icon @click="toggleSidebar" class="collapse-icon">
              <Expand v-if="isCollapse" />
              <Fold v-else />
            </el-icon>
            <span>{{ t('header.welcome', { user: userStore.username || 'User' }) }}</span>
          </div>
          <div class="header-right">
            <el-dropdown @command="handleCommand" style="margin-right: 20px;">
              <span class="el-dropdown-link">
                <el-icon><Operation /></el-icon> {{ t('header.language') }}
                <el-icon class="el-icon--right"><ArrowDown /></el-icon>
              </span>
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item command="lang-en" :disabled="currentLanguage === 'en'">{{ t('header.english') }}</el-dropdown-item>
                  <el-dropdown-item command="lang-zh" :disabled="currentLanguage === 'zh'">{{ t('header.chinese') }}</el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
            <el-dropdown @command="handleCommand">
              <span class="el-dropdown-link">
                <el-avatar icon="UserFilled" size="small" />
                <el-icon class="el-icon--right"><ArrowDown /></el-icon>
              </span>
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item command="logout">{{ t('header.logout') }}</el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
          </div>
        </el-header>
        
        <el-main :class="{ 'content-guest': !userStore.isLoggedIn, 'content-loggedin': userStore.isLoggedIn }">
          <router-view />
        </el-main>
      </el-container>
    </el-container>
  </el-config-provider>
</template>

<style scoped>
.app-container {
  height: 100vh;
  display: flex; 
}

.main-column-container {
  flex: 1; 
  display: flex; 
  flex-direction: column; 
}

.sidebar {
  background-color: #304156; 
  color: #bfcbd9; 
  transition: width 0.3s;
}

.el-menu-vertical-demo:not(.el-menu--collapse) {
  width: 200px;
  min-height: 100%;
}
.el-menu {
  border-right: none; 
}

.logo-container {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 15px 0;
  height: 60px; 
  box-sizing: border-box;
  background-color: #2b3a4a; 
}

.logo-img {
  height: 32px;
  width: 32px;
  margin-right: 10px;
}

.system-title {
  font-size: 18px;
  font-weight: bold;
  color: #fff;
}

.app-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 0 20px;
  height: 60px;
  background-color: #fff;
  border-bottom: 1px solid #dcdfe6;
  line-height: 60px;
}

.header-left {
  display: flex;
  align-items: center;
}

.collapse-icon {
  font-size: 22px;
  cursor: pointer;
  margin-right: 15px;
}

.header-right {
  display: flex;
  align-items: center;
}

.el-dropdown-link {
  cursor: pointer;
  display: flex;
  align-items: center;
}

.el-main {
  background-color: #f0f2f5;
  padding: 20px;
  overflow-y: auto; 
}

.content-guest {
  padding: 0;
  flex: 1; 
  display: flex; 
  flex-direction: column; 
  background-color: #f0f2f5; 
}

.el-menu-item,
.el-sub-menu__title {
  color: #383c40; 
}

.el-menu-item [class^="el-icon"],
.el-sub-menu__title [class^="el-icon"] {
  color: #383c40;
}

.el-menu-item.is-active {
  background-color: #409EFF !important; 
  color: #ffffff !important;
}
.el-menu-item.is-active [class^="el-icon"] {
  color: #ffffff !important;
}

.el-menu-item:hover {
  background-color: #818992 !important; 
}
</style>
