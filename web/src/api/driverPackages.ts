import type { ApiResponse } from './types'
import { request } from './http'

// ── Driver Packages (R6) ──

export interface DriverPackage {
  name: string
  files: number
  inf_files: number
}

export async function getDriverPackages(): Promise<ApiResponse<DriverPackage[]>> {
  const res = await request<{ packages: DriverPackage[] }>('GET', '/driver-packages')
  return { ...res, data: res.data?.packages ?? [] }
}
