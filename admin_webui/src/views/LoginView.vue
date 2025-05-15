<template>
  <div class="login-container">
    <el-card class="login-card">
      <template #header>
        <div class="card-header">
          <span>{{ t('login.title') }}</span>
        </div>
      </template>
      <el-form ref="loginFormRef" :model="loginForm" :rules="loginRules" @keyup.enter="handleLogin">
        <el-form-item prop="username">
          <el-input v-model="loginForm.username" :placeholder="t('login.usernamePlaceholder')" prefix-icon="User" />
        </el-form-item>
        <el-form-item prop="password">
          <el-input v-model="loginForm.password" type="password" :placeholder="t('login.passwordPlaceholder')" prefix-icon="Lock" show-password />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" style="width: 100%" :loading="loading" @click="handleLogin">
            {{ t('login.loginButton') }}
          </el-button>
        </el-form-item>
      </el-form>
    </el-card>
  </div>
</template>

<script lang="ts" setup>
import { ref, reactive } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { ElMessage, FormInstance, FormRules } from 'element-plus'
import apiClient from '../api'
import { useUserStore } from '../store/user'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()
const router = useRouter()
const route = useRoute()
const userStore = useUserStore()
const loginFormRef = ref<FormInstance>()
const loading = ref(false)

const loginForm = reactive({
  username: 'admin',
  password: 'admin',
})

const loginRules = reactive<FormRules>({
  username: [{ required: true, message: t('login.usernameRequired'), trigger: 'blur' }],
  password: [{ required: true, message: t('login.passwordRequired'), trigger: 'blur' }],
})

const handleLogin = async () => {
  if (!loginFormRef.value) return
  await loginFormRef.value.validate(async (valid) => {
    if (valid) {
      loading.value = true
      try {
        const response = await apiClient.post('/login', {
          username: loginForm.username,
          password: loginForm.password,
        })
        
        if (response && (response as any).success) {
          ElMessage.success(t('login.loginSuccess'))
          userStore.loginSuccess(loginForm.username)
          
          const redirectPath = route.query.redirect as string || '/';
          router.push(redirectPath); 
        } else {
          ElMessage.error((response as any).error || t('login.loginFailed'))
        }
      } catch (error: any) {
        console.error('Login error:', error)
      } finally {
        loading.value = false
      }
    } else {
      console.log('error submit!!')
    }
  })
}
</script>

<style scoped>
.login-container {
  display: flex;
  justify-content: center;
  align-items: center;
  height: 100%; 
  width: 100%; 
  background-color: #f0f2f5; 
}

.login-card {
  width: 400px;
}

.card-header {
  text-align: center;
  font-size: 20px;
}
</style>
