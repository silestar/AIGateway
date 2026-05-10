import api from './index'

export interface UpstreamModelItem {
  actual_model_name: string
  visible: boolean
  ref_count: number
}

export interface DisplayModelItem {
  display_model_name: string
  visible: boolean
  ref_count: number
}

export interface ModelListResponse {
  upstream: UpstreamModelItem[]
  display: DisplayModelItem[]
}

export const modelApi = {
  listModels() {
    return api.get<{ data: ModelListResponse }>('/models/list')
  },

  setUpstreamVisible(modelName: string, visible: boolean) {
    return api.put('/models/upstream/visible', { model_name: modelName, visible })
  },

  setDisplayVisible(modelName: string, visible: boolean) {
    return api.put('/models/display/visible', { model_name: modelName, visible })
  },
}