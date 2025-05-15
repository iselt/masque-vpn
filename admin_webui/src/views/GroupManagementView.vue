<template>
  <div class="group-management-container">
    <el-card>
      <template #header>
        <div class="card-header">
          <span>{{ t('groupManagement.title') }}</span>
          <div>
            <el-button type="primary" :icon="Plus" @click="openGroupDialog()">{{ t('groupManagement.addGroup') }}</el-button>
            <el-button :icon="Refresh" @click="handleRefresh" style="margin-left:10px;">{{ t('actions.refresh') }}</el-button>
          </div>
        </div>
      </template>

      <el-table :data="groups" v-loading="loading" stripe style="width: 100%">
        <el-table-column prop="group_name" :label="t('groupManagement.table.groupName')" sortable />
        <el-table-column :label="t('groupManagement.table.groupMembers')" min-width="250">
          <template #default="{ row }">
            <template v-if="getGroupClientNamesForTable(row.group_id).length">
              <el-tag
                v-for="clientName in getGroupClientNamesForTable(row.group_id)"
                :key="clientName"
                size="small"
                style="margin-right: 5px; margin-bottom: 5px;"
              >
                {{ clientName }}
              </el-tag>
            </template>
            <span v-else>{{ t('groupManagement.noMembers') }}</span>
          </template>
        </el-table-column>
        <el-table-column :label="t('groupManagement.table.actions')" width="300">
          <template #default="{ row }">
            <el-button size="small" type="primary" :icon="Edit" @click="openGroupDialog(row)">{{ t('actions.edit') }}</el-button>
            <el-button size="small" type="info" :icon="User" @click="openMembersDialog(row)">{{ t('groupManagement.manageMembers') }}</el-button>
            <el-button size="small" type="danger" :icon="Delete" @click="confirmDeleteGroup(row)">{{ t('actions.delete') }}</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- Add/Edit Group Dialog -->
    <el-dialog v-model="groupDialog.visible" :title="groupDialog.title" width="500px" @closed="resetGroupForm">
      <el-form ref="groupFormRef" :model="groupForm" :rules="groupRules" label-width="120px">
        <el-form-item :label="t('groupManagement.groupName')" prop="group_name">
          <el-input v-model="groupForm.group_name" :placeholder="t('groupManagement.groupNamePlaceholder')" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="groupDialog.visible = false">{{ t('actions.cancel') }}</el-button>
        <el-button type="primary" @click="submitGroupForm" :loading="groupDialog.loading">{{ t('actions.save') }}</el-button>
      </template>
    </el-dialog>

    <!-- Manage Members Dialog -->
    <el-dialog v-model="membersDialog.visible" :title="t('groupManagement.membersDialogTitle', { groupName: membersDialog.groupName })" width="700px">
      <div style="margin-bottom: 15px; display: flex; align-items: center;">
        <el-select
          v-model="membersDialog.selectedClientToAdd"
          :placeholder="t('groupManagement.selectClientPlaceholder')"
          filterable
          clearable
          style="width: 300px; margin-right: 10px;"
        >
          <el-option
            v-for="client in availableClientsForAdding"
            :key="client.client_id"
            :label="client.client_name"
            :value="client.client_id"
          />
        </el-select>
        <el-button type="primary" @click="addMemberToGroup" :disabled="!membersDialog.selectedClientToAdd">{{ t('groupManagement.addClientButton') }}</el-button>
      </div>

      <el-table :data="membersDialog.members" v-loading="membersDialog.loading" stripe height="300px">
        <el-table-column prop="client_name" :label="t('groupManagement.table.clientName')" />
        <el-table-column :label="t('groupManagement.table.actions')" width="120">
          <template #default="{ row }">
            <el-button size="small" type="danger" :icon="Delete" @click="removeMemberFromGroup(row.client_id)">{{ t('groupManagement.removeMemberButton') }}</el-button>
          </template>
        </el-table-column>
      </el-table>
      <template #footer>
        <el-button @click="membersDialog.visible = false">{{ t('actions.close') }}</el-button>
      </template>
    </el-dialog>

  </div>
</template>

<script lang="ts" setup>
import { ref, reactive, onMounted, computed } from 'vue'
import apiClient from '@/api'
import { ElMessage, ElMessageBox, FormInstance, FormRules } from 'element-plus'
import { Plus, Edit, Delete, User, Refresh } from '@element-plus/icons-vue'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

interface Group {
  group_id: string
  group_name: string
}

interface Client {
  client_id: string
  client_name: string
  created_at: string
  online: boolean
}

const groups = ref<Group[]>([])
const allClients = ref<Client[]>([])
const loading = ref(false)
const groupMembersMap = ref<Map<string, { client_id: string, client_name: string }[]>>(new Map())

const groupDialog = reactive({
  visible: false,
  title: t('groupManagement.addGroup'),
  loading: false,
  isEdit: false,
  editingGroupId: '' as string | undefined,
})
const groupFormRef = ref<FormInstance>()
const groupForm = reactive({
  group_name: '',
})
const groupRules = reactive<FormRules>({
  group_name: [{ required: true, message: t('groupManagement.groupNameRequired'), trigger: 'blur' }],
})

const membersDialog = reactive({
  visible: false,
  loading: false,
  groupId: '',
  groupName: '',
  members: [] as { client_id: string, client_name: string }[],
  selectedClientToAdd: '' as string | null,
})

const fetchGroups = async () => {
  loading.value = true
  try {
    const data = await apiClient.get<Group[]>('/groups')
    groups.value = Array.isArray(data) ? data : []
  } catch (error) {
    ElMessage.error(t('groupManagement.fetchGroupsError'))
    groups.value = []
  } finally {
    loading.value = false
  }
}

const fetchAllClients = async () => {
  try {
    const data = await apiClient.get<Client[]>('/clients')
    allClients.value = Array.isArray(data) ? data : []
  } catch (error) {
    console.error('Failed to fetch all clients for member management:', error)
    ElMessage.error(t('clientManagement.fetchError'))
    allClients.value = []
  }
}

const fetchAndMapAllGroupMembers = async () => {
  if (!groups.value.length || !allClients.value.length) {
    groupMembersMap.value.clear()
    if (!allClients.value.length) {
      await fetchAllClients()
    }
    if (!groups.value.length || !allClients.value.length) {
      groupMembersMap.value.clear()
      return
    }
  }
  const newMap = new Map<string, { client_id: string, client_name: string }[]>()
  try {
    for (const group of groups.value) {
      try {
        const memberIds = await apiClient.get<string[]>(`/groups/members?group_id=${group.group_id}`)
        if (Array.isArray(memberIds)) {
          const members = memberIds.map(id => {
            const client = allClients.value.find(c => c.client_id === id)
            return { client_id: id, client_name: client ? client.client_name : id }
          })
          newMap.set(group.group_id, members)
        } else {
          newMap.set(group.group_id, [])
        }
      } catch (memberError) {
        console.warn(`Failed to fetch members for group ${group.group_id}:`, memberError)
        newMap.set(group.group_id, [])
      }
    }
    groupMembersMap.value = newMap
  } catch (error) {
    console.error('Failed to fetch group members for table display:', error)
    ElMessage.error(t('groupManagement.fetchGroupMembersErrorOverall'))
  }
}

const getGroupClientNamesForTable = (groupId: string) => {
  return groupMembersMap.value.get(groupId)?.map(m => m.client_name).filter(name => name !== undefined && name !== null) || []
}

const openGroupDialog = (group?: Group) => {
  groupDialog.isEdit = !!group
  groupDialog.title = group ? t('groupManagement.editGroup') : t('groupManagement.addGroup')
  groupDialog.editingGroupId = group?.group_id
  if (group) {
    groupForm.group_name = group.group_name
  } else {
    groupForm.group_name = ''
  }
  groupDialog.visible = true
}

const resetGroupForm = () => {
  groupFormRef.value?.resetFields()
  groupForm.group_name = ''
}

const submitGroupForm = async () => {
  if (!groupFormRef.value) return
  await groupFormRef.value.validate(async (valid) => {
    if (valid) {
      groupDialog.loading = true
      try {
        if (groupDialog.isEdit && groupDialog.editingGroupId) {
          await apiClient.post('/groups/update', {
            GroupID: groupDialog.editingGroupId,
            GroupName: groupForm.group_name,
          })
          ElMessage.success(t('groupManagement.updateSuccess'))
        } else {
          await apiClient.post('/groups', { group_name: groupForm.group_name })
          ElMessage.success(t('groupManagement.addSuccess'))
        }
        groupDialog.visible = false
        await fetchGroups()
        await fetchAndMapAllGroupMembers()
      } catch (error) {
      } finally {
        groupDialog.loading = false
      }
    }
  })
}

const confirmDeleteGroup = (group: Group) => {
  ElMessageBox.confirm(
    t('groupManagement.confirmDeleteGroupMessage', { groupName: group.group_name }),
    t('groupManagement.confirmDeleteGroupTitle'),
    { confirmButtonText: t('actions.delete'), cancelButtonText: t('actions.cancel'), type: 'warning' }
  ).then(async () => {
    try {
      await apiClient.post(`/groups/delete?id=${group.group_id}`)
      ElMessage.success(t('groupManagement.deleteSuccess', { groupName: group.group_name }))
      await fetchGroups()
      await fetchAndMapAllGroupMembers()
    } catch (error) {
    }
  }).catch(() => {
    ElMessage.info(t('actions.cancel'))
  })
}

const fetchGroupMembers = async (groupId: string) => {
  membersDialog.loading = true
  try {
    const memberIds = await apiClient.get<string[]>(`/groups/members?group_id=${groupId}`)
    if (Array.isArray(memberIds)) {
      membersDialog.members = memberIds.map(id => {
        const client = allClients.value.find(c => c.client_id === id)
        return { client_id: id, client_name: client ? client.client_name : id }
      })
    } else {
      membersDialog.members = []
    }
  } catch (error) {
    ElMessage.error(t('groupManagement.fetchMembersError'))
    membersDialog.members = []
  } finally {
    membersDialog.loading = false
  }
}

const openMembersDialog = async (group: Group) => {
  membersDialog.groupId = group.group_id
  membersDialog.groupName = group.group_name
  membersDialog.selectedClientToAdd = null
  await fetchGroupMembers(group.group_id)
  if (!allClients.value.length) {
    await fetchAllClients()
  }
  membersDialog.visible = true
}

const availableClientsForAdding = computed(() => {
  const currentMemberIds = new Set(membersDialog.members.map(m => m.client_id))
  return allClients.value.filter(client => !currentMemberIds.has(client.client_id))
})

const addMemberToGroup = async () => {
  if (!membersDialog.selectedClientToAdd || !membersDialog.groupId) return; // selectedClientToAdd is now client_id
  membersDialog.loading = true;
  try {
    await apiClient.post('/groups/members', {
      GroupID: membersDialog.groupId,
      ClientID: membersDialog.selectedClientToAdd, // This is now client_id
    });
    const client = allClients.value.find(c => c.client_id === membersDialog.selectedClientToAdd);
    const clientNameToDisplay = client ? client.client_name : membersDialog.selectedClientToAdd; // Fallback to ID if name not found

    ElMessage.success(t('groupManagement.addMemberSuccess', { clientName: clientNameToDisplay, groupName: membersDialog.groupName }));
    await fetchGroupMembers(membersDialog.groupId); // Refresh members in the dialog
    await fetchAndMapAllGroupMembers(); // Refresh members in the main table
    membersDialog.selectedClientToAdd = null; // Clear selection
  } catch (error) {
    // Consider adding an error message to the user
    ElMessage.error(t('groupManagement.fetchMembersError')) // Example, might need a specific error message key for add failure
  } finally {
    membersDialog.loading = false;
  }
};

const removeMemberFromGroup = async (clientId: string) => {
  if (!membersDialog.groupId) return
  const clientToRemove = membersDialog.members.find(m => m.client_id === clientId)
  const clientNameToDisplay = clientToRemove ? clientToRemove.client_name : clientId

  ElMessageBox.confirm(
    t('groupManagement.confirmRemoveMemberMessage', { clientName: clientNameToDisplay, groupName: membersDialog.groupName }),
    t('groupManagement.confirmRemoveMemberTitle'),
    { confirmButtonText: t('groupManagement.removeMemberButton'), cancelButtonText: t('actions.cancel'), type: 'warning' }
  ).then(async () => {
    membersDialog.loading = true
    try {
      await apiClient.post('/groups/members/remove', {
        GroupID: membersDialog.groupId,
        ClientID: clientId,
      })
      ElMessage.success(t('groupManagement.removeMemberSuccess', { clientName: clientNameToDisplay, groupName: membersDialog.groupName }))
      await fetchGroupMembers(membersDialog.groupId)
      await fetchAndMapAllGroupMembers()
    } catch (error) {
      ElMessage.error(t('groupManagement.fetchMembersError'))
    } finally {
      membersDialog.loading = false
    }
  }).catch(() => {
    ElMessage.info(t('actions.cancel'))
  })
}

const handleRefresh = async () => {
  loading.value = true
  try {
    await fetchAllClients()
    await fetchGroups()
    await fetchAndMapAllGroupMembers()
    // ElMessage.success(t('groupManagement.refreshSuccess'))
  } catch (error) {
    ElMessage.error(t('groupManagement.refreshError'))
  } finally {
    loading.value = false
  }
}

onMounted(async () => {
  await handleRefresh()
})
</script>

<style scoped>
.group-management-container {
  padding: 20px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
</style>
