import type { ApiResponse } from './types'
import { request } from './http'

export interface StoreItem {
  id: string
  type: string
  name: string
  description: string
  version: string
  author: string
  tags?: string[]
  icon?: string
  downloads?: number
  created_at?: string
  updated_at?: string
}

export interface StoreCatalog {
  items: StoreItem[]
  total: number
}

export interface StoreItemDetail {
  id: string
  type: string
  name: string
  description: string
  version: string
  author: string
  tags?: string[]
  source?: string
  releases?: string[]
  created_at?: string
  updated_at?: string
  template_variables?: Record<string, string>
  variable_values?: Record<string, string>
  content: any
}

export interface ImportResult {
  id: string
  name: string
  type: string
  store_item: string
}

/** Fetch the store catalog from hub.pxelab.com (via PxeLab proxy). */
export async function getStoreCatalog(): Promise<ApiResponse<{ catalog: StoreCatalog }>> {
  return request('GET', '/store/catalog')
}

/** Fetch a single store item detail. */
export async function getStoreItem(type: string, id: string): Promise<ApiResponse<StoreItemDetail>> {
  return request('GET', `/store/items/${type}/${id}`)
}

/** Import a store item into the local database. */
export async function importStoreItem(itemId: string, itemType: string): Promise<ApiResponse<ImportResult>> {
  return request('POST', '/store/import', { item_id: itemId, item_type: itemType })
}

/** Import a store item from a local JSON file (content from export). */
export async function importLocalStoreItem(payload: StoreItemDetail): Promise<ApiResponse<ImportResult>> {
  return request('POST', '/store/import-local', payload)
}
