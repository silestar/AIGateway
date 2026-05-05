import api from './index'

export interface ChannelGroup {
  id: number
  name: string
  description: string
  weight: number
  created_at: string
  updated_at: string
}

export interface ConsumerGroup {
  id: number
  name: string
  description: string
  created_at: string
  updated_at: string
}

export const groupApi = {
  // 渠道分组
  listChannelGroups() {
    return api.get<{ data: ChannelGroup[]; total: number }>('/channel-groups')
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
  removeChannelFromGroup(groupId: number, channelId: number) {
    return api.delete(`/channel-groups/${groupId}/channels/${channelId}`)
  },

  // 消费者分组
  listConsumerGroups() {
    return api.get<{ data: ConsumerGroup[]; total: number }>('/consumer-groups')
  },
  createConsumerGroup(data: { name: string; description?: string }) {
    return api.post<{ data: ConsumerGroup }>('/consumer-groups', data)
  },
  updateConsumerGroup(id: number, data: { name?: string; description?: string }) {
    return api.put(`/consumer-groups/${id}`, data)
  },
  deleteConsumerGroup(id: number) {
    return api.delete(`/consumer-groups/${id}`)
  },
  addConsumerToGroup(groupId: number, consumerId: number, quotaRpm?: number, quotaTpm?: number) {
    return api.post(`/consumer-groups/${groupId}/consumers`, {
      consumer_id: consumerId,
      quota_rpm: quotaRpm || 0,
      quota_tpm: quotaTpm || 0,
    })
  },
  removeConsumerFromGroup(groupId: number, consumerId: number) {
    return api.delete(`/consumer-groups/${groupId}/consumers/${consumerId}`)
  },

  // 绑定关系
  bindChannelGroup(consumerGroupId: number, channelGroupId: number) {
    return api.post('/group-bindings', { consumer_group_id: consumerGroupId, channel_group_id: channelGroupId })
  },
  unbindChannelGroup(consumerGroupId: number, channelGroupId: number) {
    return api.delete(`/group-bindings/${consumerGroupId}/${channelGroupId}`)
  },
}
