import api from './index'

export interface ChannelGroup {
  id: number
  name: string
  description: string
  weight: number
  channel_count: number
  created_at: string
  updated_at: string
}

export interface ChannelInfo {
  id: number
  name: string
  type: string
  status: string
  weight: number
}

export interface ChannelGroupDetail extends ChannelGroup {
  channels: ChannelInfo[]
}

export interface KeysGroup {
  id: number
  name: string
  description: string
  quota_rpm: number
  quota_tpm: number
  channel_count: number
  created_at: string
  updated_at: string
}

export interface KeysInfo {
  id: number
  name: string
  prefix: string
  status: string
}
export interface KeysGroupDetail extends KeysGroup {
  bound_channel_groups: ChannelGroup[]
  available_channel_groups: ChannelGroup[]
  bound_keys: KeysInfo[]
  available_keys: KeysInfo[]
}

export const groupApi = {
  // 渠道分组
  listChannelGroups() {
    return api.get<{ data: ChannelGroup[]; total: number }>('/channel-groups')
  },
  getChannelGroup(id: number) {
    return api.get<{ data: ChannelGroupDetail }>(`/channel-groups/${id}`)
  },
  createChannelGroup(data: { name: string; description?: string; weight?: number }) {
    return api.post<{ data: ChannelGroup }>('/channel-groups', data)
  },
  updateChannelGroup(id: number, data: { name?: string; description?: string; weight?: number }) {
    return api.put(`/channel-groups/${id}`, data)
  },
  deleteChannelGroup(id: number) {
    return api.delete(`/channel-groups/${id}`)
  },
  addChannelToGroup(groupId: number, channelId: number, weight?: number) {
    return api.post(`/channel-groups/${groupId}/channels`, { channel_id: channelId, weight: weight || 0 })
  },
  setChannelGroupChannels(groupId: number, channelIds: number[]) {
    return api.put(`/channel-groups/${groupId}/channels`, { channel_ids: channelIds })
  },
  removeChannelFromGroup(groupId: number, channelId: number) {
    return api.delete(`/channel-groups/${groupId}/channels/${channelId}`)
  },

  // 密钥分组
  listKeysGroups() {
    return api.get<{ data: KeysGroup[]; total: number }>('/keys-groups')
  },
  getKeysGroup(id: number) {
    return api.get<{ data: KeysGroupDetail }>(`/keys-groups/${id}`)
  },
  createKeysGroup(data: { name: string; description?: string; quota_rpm?: number; quota_tpm?: number }) {
    return api.post<{ data: KeysGroup }>('/keys-groups', data)
  },
  updateKeysGroup(id: number, data: { name?: string; description?: string; quota_rpm?: number; quota_tpm?: number }) {
    return api.put(`/keys-groups/${id}`, data)
  },
  deleteKeysGroup(id: number) {
    return api.delete(`/keys-groups/${id}`)
  },
  addKeysToGroup(groupId: number, keysId: number) {
    return api.post(`/keys-groups/${groupId}/keys`, { keys_id: keysId })
  },
  setKeysGroupChannelGroups(groupId: number, channelGroupIds: number[]) {
    return api.put(`/keys-groups/${groupId}/channel-groups`, { channel_group_ids: channelGroupIds })
  },
  removeKeysFromGroup(groupId: number, keysId: number) {
    return api.delete(`/keys-groups/${groupId}/keys/${keysId}`)
  },

  // 绑定关系
  bindChannelGroup(keysGroupId: number, channelGroupId: number) {
    return api.post('/group-bindings', { keys_group_id: keysGroupId, channel_group_id: channelGroupId })
  },
  unbindChannelGroup(keysGroupId: number, channelGroupId: number) {
    return api.delete(`/group-bindings/${keysGroupId}/${channelGroupId}`)
  },
}