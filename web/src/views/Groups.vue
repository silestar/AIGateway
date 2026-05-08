<template>
  <n-tabs type="line" animated>
    <!-- ====== 渠道分组 Tab ====== -->
    <n-tab-pane name="channel" :tab="t('groups.channelGroups')">
      <div class="group-layout">
        <!-- 左侧列表 -->
        <n-card class="group-list-panel" :bordered="false" size="small" content-style="padding: 8px">
          <div class="list-header">
            <n-button size="small" type="primary" block @click="startCreateCG">+ {{ t('groups.createChannelGroup') }}</n-button>
          </div>
          <n-empty v-if="channelGroups.length === 0 && !cgLoading" :description="t('groups.noGroups')">
            <template #extra>
              <n-button size="small" @click="startCreateCG">{{ t('groups.createFirst') }}</n-button>
            </template>
          </n-empty>
          <n-spin :show="cgLoading">
            <div class="list-items">
              <div
                v-for="cg in channelGroups"
                :key="cg.id"
                class="list-item"
                :class="{ selected: cgSelected?.id === cg.id }"
                @click="selectCG(cg)"
              >
                <div class="item-name">{{ cg.name }}</div>
                <div class="item-meta">
                  <n-tag size="small" :bordered="false">W{{ cg.weight }}</n-tag>
                  <span class="item-count">{{ cg.channel_count }} {{ t('channels.title') }}</span>
                </div>
              </div>
            </div>
          </n-spin>
        </n-card>

        <!-- 右侧编辑区 -->
        <div class="group-edit-panel">
          <div v-if="!cgSelected" class="empty-hint">
            <n-empty :description="t('groups.selectGroup')" />
            <p class="hint-sub">{{ t('groups.selectGroupHint') }}</p>
          </div>
          <n-spin v-else-if="cgDetailLoading" />
          <template v-else-if="cgDetail">
            <n-card :bordered="false" size="small" :title="t('common.detail')">
              <n-form :model="cgDetailForm" label-placement="left" label-width="80">
                <n-form-item :label="t('groups.name')">
                  <n-input v-model:value="cgDetailForm.name" />
                </n-form-item>
                <n-form-item :label="t('groups.description')">
                  <n-input v-model:value="cgDetailForm.description" type="textarea" />
                </n-form-item>
                <n-form-item :label="t('groups.weight')">
                  <n-input-number v-model:value="cgDetailForm.weight" :min="0" />
                  <span class="form-hint">{{ t('groups.channelWeightHint') }}</span>
                </n-form-item>
              </n-form>
            </n-card>

            <n-card :bordered="false" size="small" :title="t('groups.linkedChannels')" style="margin-top: 12px">
              <n-alert type="info" style="margin-bottom: 12px">
                {{ t('groups.channelPriorityTip') }}
              </n-alert>
              <n-button size="small" @click="openAddChannelModal">+ {{ t('groups.addChannel') }}</n-button>
              <n-empty v-if="!cgDetail?.channels?.length" :description="t('groups.noChannelsLinked')" style="margin-top: 12px" />
              <n-table v-else :single-line="false" size="small" style="margin-top: 8px">
                <thead>
                  <tr>
                    <th>ID</th>
                    <th>{{ t('channels.name') }}</th>
                    <th>{{ t('common.type') }}</th>
                    <th>{{ t('common.status') }}</th>
                    <th>{{ t('common.weight') }}</th>
                    <th>{{ t('common.actions') }}</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="ch in sortedChannels" :key="ch.id">
                    <td>{{ ch.id }}</td>
                    <td>{{ ch.name }}</td>
                    <td>{{ ch.type }}</td>
                    <td><n-tag :type="ch.status === 'active' ? 'success' : 'default'" size="small">{{ t('common.' + ch.status) }}</n-tag></td>
                    <td>{{ ch.weight }}</td>
                    <td><n-button size="tiny" type="error" ghost @click="removeChannel(ch.id)">{{ t('groups.removeChannel') }}</n-button></td>
                  </tr>
                </tbody>
              </n-table>
            </n-card>

            <div style="margin-top: 16px; text-align: right">
              <n-button type="error" ghost size="small" @click="handleDeleteCG" style="margin-right: 8px">{{ t('common.delete') }}</n-button>
              <n-button type="primary" size="small" @click="handleSaveCG">{{ t('common.save') }}</n-button>
            </div>
          </template>
        </div>
      </div>
    </n-tab-pane>

    <!-- ====== 密钥分组 Tab ====== -->
    <n-tab-pane name="keys" :tab="t('groups.keysGroups')">
      <div class="group-layout">
        <n-card class="group-list-panel" :bordered="false" size="small" content-style="padding: 8px">
          <div class="list-header">
            <n-button size="small" type="primary" block @click="startCreateKG">+ {{ t('groups.createKeysGroup') }}</n-button>
          </div>
          <n-empty v-if="keysGroups.length === 0 && !kgLoading" :description="t('groups.noGroups')">
            <template #extra>
              <n-button size="small" @click="startCreateKG">{{ t('groups.createFirst') }}</n-button>
            </template>
          </n-empty>
          <n-spin :show="kgLoading">
            <div class="list-items">
              <div
                v-for="kg in keysGroups"
                :key="kg.id"
                class="list-item"
                :class="{ selected: kgSelected?.id === kg.id }"
                @click="selectKG(kg)"
              >
                <div class="item-name">{{ kg.name }}</div>
                <div class="item-meta">
                  <n-tag size="small" :bordered="false" type="info">RPM {{ kg.quota_rpm }}</n-tag>
                  <span class="item-count" v-if="kg.channel_count">{{ kg.channel_count }} groups</span>
                </div>
              </div>
            </div>
          </n-spin>
        </n-card>

        <div class="group-edit-panel">
          <div v-if="!kgSelected" class="empty-hint">
            <n-empty :description="t('groups.selectGroup')" />
            <p class="hint-sub">{{ t('groups.selectGroupHint') }}</p>
          </div>
          <n-spin v-else-if="kgDetailLoading" />
          <template v-else-if="kgDetail">
            <n-card :bordered="false" size="small" :title="t('common.detail')">
              <n-form :model="kgDetailForm" label-placement="left" label-width="80">
                <n-form-item :label="t('groups.name')">
                  <n-input v-model:value="kgDetailForm.name" />
                </n-form-item>
                <n-form-item :label="t('groups.description')">
                  <n-input v-model:value="kgDetailForm.description" type="textarea" />
                </n-form-item>
              </n-form>
            </n-card>

            <n-card :bordered="false" size="small" :title="t('groups.linkedKeys')" style="margin-top: 12px">
              <n-button size="small" @click="openAddKeysModal" style="margin-bottom: 8px">+ {{ t('groups.addKeys') }}</n-button>
              <n-empty v-if="!kgDetail?.bound_keys?.length" :description="t('groups.noKeysLinked')" />
              <n-table v-else :single-line="false" size="small">
                <thead>
                  <tr>
                    <th>ID</th>
                    <th>{{ t('keys.name') }}</th>
                    <th>Prefix</th>
                    <th>{{ t('common.status') }}</th>
                    <th>{{ t('common.actions') }}</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="k in kgDetail.bound_keys" :key="k.id">
                    <td>{{ k.id }}</td>
                    <td>{{ k.name }}</td>
                    <td>{{ k.prefix }}</td>
                    <td><n-tag :type="k.status === 'active' ? 'success' : 'default'" size="small">{{ t('common.' + k.status) }}</n-tag></td>
                    <td><n-button size="tiny" type="error" ghost @click="removeKeys(k.id)">{{ t('groups.removeKeys') }}</n-button></td>
                  </tr>
                </tbody>
              </n-table>
            </n-card>

            <n-card :bordered="false" size="small" :title="t('groups.accessibleChannels')" style="margin-top: 12px">
              <n-checkbox-group v-model:value="kgChannelGroupSelection">
                <n-space vertical>
                  <n-checkbox v-for="opt in kgChannelGroupOptions" :key="opt.value" :value="opt.value" :label="opt.label" />
                </n-space>
              </n-checkbox-group>
            </n-card>

            <n-card :bordered="false" size="small" :title="t('groups.quotaRPM')" style="margin-top: 12px">
              <n-form :model="kgDetailForm" label-placement="left" label-width="80">
                <n-form-item :label="t('groups.quotaRPM')">
                  <n-input-number v-model:value="kgDetailForm.quota_rpm" :min="0" />
                  <span class="form-hint">{{ t('groups.quotaRPMHint') }}</span>
                </n-form-item>
                <n-form-item :label="t('groups.quotaTPM')">
                  <n-input-number v-model:value="kgDetailForm.quota_tpm" :min="0" />
                  <span class="form-hint">{{ t('groups.quotaTPMHint') }}</span>
                </n-form-item>
              </n-form>
            </n-card>

            <div style="margin-top: 16px; text-align: right">
              <n-button type="error" ghost size="small" @click="handleDeleteKG" style="margin-right: 8px">{{ t('common.delete') }}</n-button>
              <n-button type="primary" size="small" @click="handleSaveKG">{{ t('common.save') }}</n-button>
            </div>
          </template>
        </div>
      </div>
    </n-tab-pane>
  </n-tabs>

  <!-- 新建渠道分组 -->
  <n-modal v-model:show="showCreateCG" preset="dialog" :title="t('groups.createChannelGroup')" :positive-text="t('common.save')" :negative-text="t('common.cancel')" @positive-click="handleCreateCG">
    <n-form :model="cgForm">
      <n-form-item :label="t('groups.name')"><n-input v-model:value="cgForm.name" /></n-form-item>
      <n-form-item :label="t('groups.description')"><n-input v-model:value="cgForm.description" type="textarea" /></n-form-item>
    </n-form>
  </n-modal>

  <!-- 新建密钥分组 -->
  <n-modal v-model:show="showCreateKG" preset="dialog" :title="t('groups.createKeysGroup')" :positive-text="t('common.save')" :negative-text="t('common.cancel')" @positive-click="handleCreateKG">
    <n-form :model="kgForm">
      <n-form-item :label="t('groups.name')"><n-input v-model:value="kgForm.name" /></n-form-item>
      <n-form-item :label="t('groups.description')"><n-input v-model:value="kgForm.description" type="textarea" /></n-form-item>
    </n-form>
  </n-modal>

  <!-- 添加渠道弹窗 -->
  <n-modal v-model:show="showAddChannelModal" preset="dialog" :title="t('groups.addChannel')" :positive-text="t('common.save')" :negative-text="t('common.cancel')" @positive-click="handleAddChannels">
    <n-checkbox-group v-model:value="addChannelSelection">
      <n-space vertical>
        <n-checkbox v-for="ch in availableChannels" :key="ch.id" :value="ch.id" :label="`${ch.name} (${ch.type})`" />
      </n-space>
    </n-checkbox-group>
  </n-modal>

  <!-- 添加密钥弹窗 -->
  <n-modal v-model:show="showAddKeysModal" preset="dialog" :title="t('groups.addKeys')" :positive-text="t('common.save')" :negative-text="t('common.cancel')" @positive-click="handleAddKeys">
    <n-checkbox-group v-model:value="addKeysSelection">
      <n-space vertical>
        <n-checkbox v-for="k in kgDetail?.available_keys" :key="k.id" :value="k.id" :label="`${k.name} (${k.prefix})`" />
      </n-space>
    </n-checkbox-group>
  </n-modal>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useMessage } from 'naive-ui'
import { groupApi, type ChannelGroup, type ChannelGroupDetail, type ChannelInfo, type KeysGroup, type KeysGroupDetail } from '../api/group'
import { channelApi, type Channel } from '../api/channel'

const { t } = useI18n()
const message = useMessage()

// ========== 渠道分组 ==========
const channelGroups = ref<ChannelGroup[]>([])
const cgLoading = ref(false)
const cgSelected = ref<ChannelGroup | null>(null)
const cgDetail = ref<ChannelGroupDetail | null>(null)
const cgDetailLoading = ref(false)
const cgDetailForm = reactive({ name: '', description: '', weight: 0 })
const showCreateCG = ref(false)
const cgForm = reactive({ name: '', description: '' })

// 按权重降序排列的渠道
const sortedChannels = computed<ChannelInfo[]>(() => {
  if (!cgDetail.value?.channels) return []
  return [...cgDetail.value.channels].sort((a, b) => b.weight - a.weight)
})

let cgAbort: AbortController | null = null

async function loadChannelGroups() {
  cgLoading.value = true
  try { const res = await groupApi.listChannelGroups(); channelGroups.value = res.data.data || [] }
  finally { cgLoading.value = false }
}

async function selectCG(cg: ChannelGroup) {
  cgAbort?.abort()
  cgAbort = new AbortController()
  cgSelected.value = cg
  cgDetailLoading.value = true
  try {
    const res = await groupApi.getChannelGroup(cg.id)
    cgDetail.value = res.data.data
    cgDetailForm.name = cgDetail.value.name
    cgDetailForm.description = cgDetail.value.description
    cgDetailForm.weight = cgDetail.value.weight
  } catch (e: any) {
    if (e?.name !== 'CanceledError') { /* ignore abort */ }
  } finally { cgDetailLoading.value = false }
}

function startCreateCG() { cgForm.name = ''; cgForm.description = ''; showCreateCG.value = true }
async function handleCreateCG() {
  try { await groupApi.createChannelGroup(cgForm); message.success(t('common.success')); showCreateCG.value = false; loadChannelGroups() }
  catch { message.error(t('common.createFailed')) }
}

async function handleSaveCG() {
  if (!cgSelected.value || !cgDetail.value) return
  try {
    await groupApi.updateChannelGroup(cgSelected.value.id, { name: cgDetailForm.name, description: cgDetailForm.description, weight: cgDetailForm.weight })
    message.success(t('common.success'))
    loadChannelGroups()
    selectCG(cgSelected.value)
  } catch { message.error(t('common.operationFailed')) }
}

async function handleDeleteCG() {
  if (!cgSelected.value) return
  try {
    await groupApi.deleteChannelGroup(cgSelected.value.id)
    message.success(t('common.deleted'))
    cgSelected.value = null; cgDetail.value = null
    loadChannelGroups()
  } catch (e: any) { message.error(e?.response?.data?.error?.message || t('groups.deleteBlocked')) }
}

// 添加渠道
const showAddChannelModal = ref(false)
const addChannelSelection = ref<number[]>([])
const availableChannels = ref<Channel[]>([])

function openAddChannelModal() {
  addChannelSelection.value = cgDetail.value?.channels.map(ch => ch.id) || []
  showAddChannelModal.value = true
}

async function handleAddChannels() {
  if (!cgSelected.value || addChannelSelection.value.length === 0) return
  try {
    await groupApi.setChannelGroupChannels(cgSelected.value.id, addChannelSelection.value)
    message.success(t('common.success'))
    showAddChannelModal.value = false
    selectCG(cgSelected.value)
  } catch { message.error(t('common.operationFailed')) }
}

async function removeChannel(channelId: number) {
  if (!cgSelected.value || !cgDetail.value) return
  try {
    await groupApi.removeChannelFromGroup(cgSelected.value.id, channelId)
    message.success(t('common.deleted'))
    selectCG(cgSelected.value)
  } catch { message.error(t('common.operationFailed')) }
}

// ========== 密钥分组 ==========
const keysGroups = ref<KeysGroup[]>([])
const kgLoading = ref(false)
const kgSelected = ref<KeysGroup | null>(null)
const kgDetail = ref<KeysGroupDetail | null>(null)
const kgDetailLoading = ref(false)
const kgDetailForm = reactive({ name: '', description: '', quota_rpm: 0, quota_tpm: 0 })
const showCreateKG = ref(false)
const kgForm = reactive({ name: '', description: '' })

// checkbox 替代穿梭框
const kgChannelGroupSelection = ref<number[]>([])
const kgChannelGroupOptions = computed(() => {
  if (!kgDetail.value) return []
  const bound = kgDetail.value.bound_channel_groups.map(g => ({ label: `${g.name} (W${g.weight})`, value: g.id }))
  const avail = kgDetail.value.available_channel_groups.map(g => ({ label: `${g.name} (W${g.weight})`, value: g.id }))
  return [...bound, ...avail]
})

let kgAbort: AbortController | null = null

async function loadKeysGroups() {
  kgLoading.value = true
  try { const res = await groupApi.listKeysGroups(); keysGroups.value = res.data.data || [] }
  finally { kgLoading.value = false }
}

async function selectKG(kg: KeysGroup) {
  kgAbort?.abort()
  kgAbort = new AbortController()
  kgSelected.value = kg
  kgDetailLoading.value = true
  try {
    const res = await groupApi.getKeysGroup(kg.id)
    kgDetail.value = res.data.data
    kgDetailForm.name = kgDetail.value.name
    kgDetailForm.description = kgDetail.value.description
    kgDetailForm.quota_rpm = kgDetail.value.quota_rpm
    kgDetailForm.quota_tpm = kgDetail.value.quota_tpm
    kgChannelGroupSelection.value = kgDetail.value.bound_channel_groups.map(g => g.id)
  } catch (e: any) {
    if (e?.name !== 'CanceledError') { /* ignore */ }
  } finally { kgDetailLoading.value = false }
}

function startCreateKG() { kgForm.name = ''; kgForm.description = ''; showCreateKG.value = true }
async function handleCreateKG() {
  try { await groupApi.createKeysGroup(kgForm); message.success(t('common.success')); showCreateKG.value = false; loadKeysGroups() }
  catch { message.error(t('common.createFailed')) }
}

async function handleSaveKG() {
  if (!kgSelected.value) return
  try {
    await groupApi.updateKeysGroup(kgSelected.value.id, kgDetailForm)
    await groupApi.setKeysGroupChannelGroups(kgSelected.value.id, kgChannelGroupSelection.value)
    message.success(t('common.success'))
    loadKeysGroups()
  } catch { message.error(t('common.operationFailed')) }
}

async function handleDeleteKG() {
  if (!kgSelected.value) return
  try {
    await groupApi.deleteKeysGroup(kgSelected.value.id)
    message.success(t('common.deleted'))
    kgSelected.value = null; kgDetail.value = null
    loadKeysGroups()
  } catch (e: any) { message.error(e?.response?.data?.error?.message || t('groups.deleteBlocked')) }
}

// 密钥分组 - 添加/移除密钥
const showAddKeysModal = ref(false)
const addKeysSelection = ref<number[]>([])

function openAddKeysModal() {
  addKeysSelection.value = []
  showAddKeysModal.value = true
}

async function handleAddKeys() {
  if (!kgSelected.value || addKeysSelection.value.length === 0) return
  try {
    for (const kid of addKeysSelection.value) {
      await groupApi.addKeysToGroup(kgSelected.value.id, kid)
    }
    message.success(t('common.success'))
    showAddKeysModal.value = false
    selectKG(kgSelected.value)
  } catch { message.error(t('common.operationFailed')) }
}

async function removeKeys(keysId: number) {
  if (!kgSelected.value) return
  try {
    await groupApi.removeKeysFromGroup(kgSelected.value.id, keysId)
    message.success(t('common.deleted'))
    selectKG(kgSelected.value)
  } catch { message.error(t('common.operationFailed')) }
}

async function loadAvailableChannels() {
  try { const res = await channelApi.list({ page_size: 1000 }); availableChannels.value = res.data.data || [] }
  catch { /* ignore */ }
}

onMounted(() => { loadChannelGroups(); loadKeysGroups(); loadAvailableChannels() })
onUnmounted(() => { cgAbort?.abort(); kgAbort?.abort() })
</script>

<style scoped>
.group-layout {
  display: flex;
  gap: 0;
  height: calc(100vh - 200px);
  min-height: 500px;
}
.group-list-panel {
  width: 260px;
  min-width: 260px;
  border-right: 1px solid var(--n-border-color);
  overflow-y: auto;
}
.list-header { margin-bottom: 12px; }
.list-items { display: flex; flex-direction: column; gap: 4px; }
.list-item {
  padding: 10px 12px;
  border-radius: 8px;
  cursor: pointer;
  transition: all .15s;
  border: 1px solid transparent;
}
.list-item:hover { background: rgba(0, 210, 255, 0.06); }
.list-item.selected {
  background: rgba(0, 210, 255, 0.12);
  border-color: transparent;
}
.list-item.selected .item-name { color: #00d2ff; }
.list-item.selected:hover { background: rgba(0, 210, 255, 0.18); }
.item-name { font-weight: 600; }
.item-meta { display: flex; gap: 6px; align-items: center; margin-top: 4px; }
.item-count { font-size: 12px; color: var(--n-text-color-disabled); }

.group-edit-panel {
  flex: 1;
  overflow-y: auto;
  padding: 16px 24px;
}
.empty-hint {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 100%;
  min-height: 300px;
}
.hint-sub {
  margin-top: 8px;
  font-size: 13px;
  color: var(--n-text-color-disabled);
}
.form-hint {
  font-size: 12px;
  color: var(--n-text-color-disabled);
  margin-left: 8px;
}
</style>