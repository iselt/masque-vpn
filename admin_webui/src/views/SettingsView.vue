<template>
  <div class="settings-container">
    <el-card>
      <template #header>
        <div class="card-header">
          <span>{{ t('settings.title') }}</span>
        </div>
      </template>
      <el-form
        ref="settingsFormRef"
        :model="settingsForm"
        :rules="settingsRules"
        label-width="150px"
        v-loading="loading"
      >
        <el-form-item :label="t('settings.serverAddress')" prop="server_addr">
          <el-input v-model="settingsForm.server_addr" :placeholder="t('settings.serverAddressPlaceholder')" />
        </el-form-item>
        <el-form-item :label="t('settings.serverName')" prop="server_name">
          <el-input v-model="settingsForm.server_name" :placeholder="t('settings.serverNamePlaceholder')" />
        </el-form-item>
        <el-form-item :label="t('settings.mtu')" prop="mtu">
          <el-input-number v-model="settingsForm.mtu" :min="1280" :max="1500" :placeholder="t('settings.mtuPlaceholder')" />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="saveSettings" :loading="saving">{{ t('actions.save') }}</el-button>
          <el-button @click="resetForm">{{ t('actions.reset') }}</el-button>
        </el-form-item>
      </el-form>
    </el-card>
  </div>
</template>

<script lang="ts" setup>
import { ref, reactive, onMounted } from 'vue'
import apiClient from '@/api'
import { ElMessage, FormInstance, FormRules } from 'element-plus'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

interface ServerConfig {
  server_addr: string
  server_name: string
  mtu: number
}

const settingsFormRef = ref<FormInstance>()
const settingsForm = reactive<ServerConfig>({
  server_addr: '',
  server_name: '',
  mtu: 1413, // Default MTU
})
const originalSettings = reactive<ServerConfig>({ ...settingsForm }) // For reset functionality

const loading = ref(false)
const saving = ref(false)

const validateServerAddress = (_rule: any, value: string, callback: any) => {
  if (!value) {
    return callback(new Error(t('settings.validation.serverAddressRequired')))
  }
  // Basic regex for host:port or domain:port. More complex validation might be needed.
  const regex = /^([a-zA-Z0-9.-]+|\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}):\d{1,5}$/;
  if (!regex.test(value)) {
    return callback(new Error(t('settings.validation.serverAddressInvalid')))
  }
  callback()
}

const settingsRules = reactive<FormRules>({
  server_addr: [
    { required: true, validator: validateServerAddress, trigger: 'blur' }
  ],
  server_name: [ // SNI is usually a domain name
    { required: true, message: t('settings.validation.serverNameRequired'), trigger: 'blur' },
  ],
  mtu: [
    { required: true, message: t('settings.validation.mtuRequired'), trigger: 'blur' },
    { type: 'number', message: t('settings.validation.mtuType'), trigger: 'blur' },
    { type: 'number', min: 1280, max: 1500, message: t('settings.validation.mtuRange'), trigger: 'blur' },
  ],
})

const fetchServerConfig = async () => {
  loading.value = true
  try {
    const data = await apiClient.get<ServerConfig>('/server_config')
    if (data) {
      Object.assign(settingsForm, data)
      Object.assign(originalSettings, data) // Store fetched data as original
    }
  } catch (error) {
    ElMessage.error(t('settings.fetchError'))
  } finally {
    loading.value = false
  }
}

const saveSettings = async () => {
  if (!settingsFormRef.value) return
  await settingsFormRef.value.validate(async (valid) => {
    if (valid) {
      saving.value = true
      try {
        await apiClient.post('/server_config', settingsForm)
        ElMessage.success(t('settings.saveSuccess'))
        Object.assign(originalSettings, { ...settingsForm }) // Update original settings on successful save
      } catch (error) {
         ElMessage.error(t('settings.saveError'))
      } finally {
        saving.value = false
      }
    }
  })
}

const resetForm = () => {
  Object.assign(settingsForm, originalSettings) // Reset to original fetched/saved values
  settingsFormRef.value?.clearValidate() // Clear validation messages
  ElMessage.info(t('settings.resetMessage'))
}

onMounted(() => {
  fetchServerConfig()
})
</script>

<style scoped>
.settings-container {
  padding: 20px;
}
.card-header {
  font-size: 18px;
  font-weight: bold;
}
</style>
