import api from './index'

export interface ModelCatalogItem {
  id: number
  model_name: string
  is_mapped: boolean
  visible: boolean
  ref_count: number
  created_at: string
  updated_at: string
}

export const modelApi = {
  listCatalog() {
    return api.get<{ data: ModelCatalogItem[] }>('/models/catalog')
  },
  updateVisibility(id: number, visible: boolean) {
    return api.put(`/models/catalog/${id}/visibility`, { visible })
  },
  batchUpdateVisibility(ids: number[], visible: boolean) {
    return api.put('/models/catalog/visibility/batch', { ids, visible })
  },
}
