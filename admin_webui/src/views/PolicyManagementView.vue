<template>
  <div class="policy-management-container">
    <el-card>
      <template #header>
        <div class="card-header">
          <span>{{ t('policyManagement.title') }}</span>
          <div>
            <el-select v-model="filterGroupId" :placeholder="t('policyManagement.filterByGroup')" clearable @change="fetchPolicies" style="width: 200px; margin-right: 10px;">
              <el-option :label="t('policyManagement.allGroups')" :value="null" />
              <el-option
                v-for="group in groups"
                :key="group.group_id"
                :label="group.group_name"
                :value="group.group_id"
              />
            </el-select>
            <el-button type="primary" :icon="Plus" @click="openPolicyDialog()">{{ t('policyManagement.addPolicy') }}</el-button>
            <el-button :icon="Refresh" @click="fetchPolicies" style="margin-left:10px;">{{ t('actions.refresh') }}</el-button>
          </div>
        </div>
      </template>

      <el-table :data="policies" v-loading="loading" stripe style="width: 100%">
        <el-table-column prop="group_id" :label="t('policyManagement.table.group')" width="200" :formatter="groupNameFormatter" sortable />
        <el-table-column prop="action" :label="t('policyManagement.table.action')" width="100" sortable>
            <template #default="{ row }">
                <el-tag :type="row.action === 'allow' ? 'success' : 'warning'">
                    {{ row.action === 'allow' ? t('policyManagement.allow') : t('policyManagement.deny') }}
                </el-tag>
            </template>
        </el-table-column>
        <el-table-column prop="ip_prefix" :label="t('policyManagement.table.ipPrefix')" width="180" sortable />
        <el-table-column prop="priority" :label="t('policyManagement.table.priority')" width="100" sortable />
        <el-table-column prop="remarks" :label="t('policyManagement.table.remarks')" width="180" sortable />
        <el-table-column :label="t('policyManagement.table.actions')" width="180" fixed="right">
          <template #default="{ row }">
            <el-button size="small" type="primary" :icon="Edit" @click="openPolicyDialog(row)">{{ t('actions.edit') }}</el-button>
            <el-button size="small" type="danger" :icon="Delete" @click="confirmDeletePolicy(row.policy_id)">{{ t('actions.delete') }}</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- Add/Edit Policy Dialog -->
    <el-dialog v-model="policyDialog.visible" :title="policyDialog.title" width="600px" @closed="resetPolicyForm">
      <el-form ref="policyFormRef" :model="policyForm" :rules="policyRules" label-width="120px">
        <el-form-item :label="t('policyManagement.group')" prop="group_id">
          <el-select v-model="policyForm.group_id" :placeholder="t('policyManagement.groupRequired')" style="width: 100%;">
            <el-option
              v-for="group in groups"
              :key="group.group_id"
              :label="group.group_name"
              :value="group.group_id"
            />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('policyManagement.action')" prop="action">
          <el-radio-group v-model="policyForm.action">
            <el-radio label="allow">{{ t('policyManagement.allow') }}</el-radio>
            <el-radio label="deny">{{ t('policyManagement.deny') }}</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item :label="t('policyManagement.ipPrefix')" prop="ip_prefix">
          <el-input v-model="policyForm.ip_prefix" :placeholder="t('policyManagement.ipPrefixInvalid')" />
        </el-form-item>
        <el-form-item :label="t('policyManagement.priority')" prop="priority">
          <el-input-number v-model="policyForm.priority" :min="0" />
        </el-form-item>
        <el-form-item :label="t('policyManagement.remarks')" prop="remarks">
          <el-input v-model="policyForm.remarks" type="textarea" :placeholder="t('policyManagement.remarksPlaceholder')" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="policyDialog.visible = false">{{ t('actions.cancel') }}</el-button>
        <el-button type="primary" @click="submitPolicyForm" :loading="policyDialog.loading">{{ t('actions.save') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script lang="ts" setup>
import { ref, reactive, onMounted } from 'vue'
import apiClient from '@/api'
import { ElMessage, ElMessageBox, FormInstance, FormRules } from 'element-plus'
import { Plus, Edit, Delete, Refresh } from '@element-plus/icons-vue'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

interface Policy {
  policy_id: string
  group_id: string
  action: 'allow' | 'deny'
  ip_prefix: string
  priority: number
  remarks?: string
}

interface Group {
  group_id: string
  group_name: string
}

const policies = ref<Policy[]>([])
const groups = ref<Group[]>([])
const loading = ref(false)
const filterGroupId = ref<string | null>(null)

const policyDialog = reactive({
  visible: false,
  title: t('policyManagement.addPolicy'),
  loading: false,
  isEdit: false,
  editingPolicyId: '' as string | undefined,
})
const policyFormRef = ref<FormInstance>()
const policyForm = reactive<Omit<Policy, 'policy_id'>>({
  group_id: '',
  action: 'allow',
  ip_prefix: '',
  priority: 1,
  remarks: '',
})

const validateIpPrefix = (_rule: any, value: string, callback: any) => {
  if (!value) {
    return callback(new Error(t('policyManagement.ipPrefixRequired')))
  }
  const cidrRegex = /^([0-9]{1,3}\.){3}[0-9]{1,3}(\/([0-9]|[1-2][0-9]|3[0-2]))?$/
  if (!cidrRegex.test(value)) {
    return callback(new Error(t('policyManagement.ipPrefixInvalid')))
  }
  callback()
}

const policyRules = reactive<FormRules>({
  group_id: [{ required: true, message: t('policyManagement.groupRequired'), trigger: 'change' }],
  action: [{ required: true, message: t('policyManagement.actionRequired'), trigger: 'change' }],
  ip_prefix: [
    { required: true, message: t('policyManagement.ipPrefixRequired'), trigger: 'blur' },
    { validator: validateIpPrefix, trigger: 'blur' }
  ],
  priority: [
    { required: true, message: t('policyManagement.priorityRequired'), trigger: 'blur' },
    { type: 'number', message: t('policyManagement.priorityType'), trigger: 'blur' },
  ],
})

const fetchGroups = async () => {
  try {
    const data = await apiClient.get<Group[]>('/groups')
    groups.value = Array.isArray(data) ? data : []
  } catch (error) {
    groups.value = []
  }
}

const fetchPolicies = async () => {
  loading.value = true
  try {
    let url = '/policies'
    if (filterGroupId.value) {
      const allPolicies = await apiClient.get<Policy[]>(url)
      if (Array.isArray(allPolicies)) {
        policies.value = allPolicies.filter(p => p.group_id === filterGroupId.value)
      } else {
        policies.value = []
      }
    } else {
      const data = await apiClient.get<Policy[]>(url)
      policies.value = Array.isArray(data) ? data : []
    }
  } catch (error) {
    policies.value = []
  } finally {
    loading.value = false
  }
}

const groupNameFormatter = (row: Policy) => {
  const group = groups.value.find(g => g.group_id === row.group_id)
  return group ? group.group_name : row.group_id
}

const openPolicyDialog = (policy?: Policy) => {
  policyDialog.isEdit = !!policy
  policyDialog.title = policy ? t('policyManagement.editPolicy') : t('policyManagement.addPolicy')
  policyDialog.editingPolicyId = policy?.policy_id

  if (policy) {
    policyForm.group_id = policy.group_id
    policyForm.action = policy.action
    policyForm.ip_prefix = policy.ip_prefix
    policyForm.priority = policy.priority
    policyForm.remarks = policy.remarks || ''
  } else {
    resetPolicyForm()
  }
  policyDialog.visible = true
}

const resetPolicyForm = () => {
  policyFormRef.value?.resetFields()
  policyForm.action = 'allow'
  policyForm.priority = 100
  policyForm.group_id = ''
  policyForm.ip_prefix = ''
  policyForm.remarks = ''
}

const submitPolicyForm = async () => {
  if (!policyFormRef.value) return
  await policyFormRef.value.validate(async (valid) => {
    if (valid) {
      policyDialog.loading = true
      try {
        const payload: Omit<Policy, 'policy_id'> & { policy_id?: string } = { ...policyForm }
        if (policyDialog.isEdit && policyDialog.editingPolicyId) {
          payload.policy_id = policyDialog.editingPolicyId
          await apiClient.post('/policies/update', payload)
          ElMessage.success(t('policyManagement.updateSuccess'))
        } else {
          await apiClient.post('/policies', payload)
          ElMessage.success(t('policyManagement.addSuccess'))
        }
        policyDialog.visible = false
        fetchPolicies()
      } catch (error) {
      } finally {
        policyDialog.loading = false
      }
    }
  })
}

const confirmDeletePolicy = (policyId: string) => {
  ElMessageBox.confirm(
    t('policyManagement.confirmDeletePolicyMessage'),
    t('policyManagement.confirmDeletePolicyTitle'),
    { confirmButtonText: t('actions.delete'), cancelButtonText: t('actions.cancel'), type: 'warning' }
  ).then(async () => {
    try {
      await apiClient.post(`/policies/delete?id=${policyId}`)
      ElMessage.success(t('policyManagement.deleteSuccess'))
      fetchPolicies()
    } catch (error) {
    }
  }).catch(() => {
    ElMessage.info(t('actions.cancel'))
  })
}

onMounted(() => {
  fetchGroups()
  fetchPolicies()
})
</script>

<style scoped>
.policy-management-container {
  padding: 20px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.table-toolbar {
  margin-bottom: 15px;
  display: flex;
  justify-content: space-between;
  align-items: center;
}
</style>
