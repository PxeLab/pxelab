// ── Shared types ──

export interface SubnetConfig {
  cidr: string; dhcpMode: string; pools: string[]; gateway: string; dnsServers: string; leaseTime: string; nextServer: string; chainToIPXE: boolean; whitelistEnabled: boolean
}

export interface InterfaceConfig {
  name: string; ip: string
  subnets: SubnetConfig[]
}

export const defaultIface: InterfaceConfig = {
  name: '', ip: '',
  subnets: [{ cidr: '', dhcpMode: 'server', pools: [''], gateway: '', dnsServers: '', leaseTime: '3600', nextServer: '', chainToIPXE: false, whitelistEnabled: false }],
}

export function validateIP(ip: string): boolean {
  if (!ip) return true
  const parts = ip.split('.')
  return parts.length === 4 && parts.every(p => {
    const n = parseInt(p)
    return n >= 0 && n <= 255 && String(n) === p
  })
}

export function validateCIDR(cidr: string): boolean {
  if (!cidr) return true
  const parts = cidr.split('/')
  if (parts.length !== 2) return false
  const ips = parts[0].split('.')
  const mask = parseInt(parts[1])
  return ips.length === 4 && ips.every(p => {
    const n = parseInt(p)
    return n >= 0 && n <= 255 && String(n) === p
  }) && !isNaN(mask) && mask >= 0 && mask <= 32
}

export function ipToInt(ip: string): number {
  const parts = ip.trim().split('.')
  if (parts.length !== 4) return NaN
  return parts.reduce((acc, oct) => {
    const n = parseInt(oct)
    if (isNaN(n) || n < 0 || n > 255) return NaN
    return (acc << 8) + n
  }, 0) >>> 0
}

export function ipInCIDR(ip: string, cidr: string): boolean {
  const [netIP, maskStr] = cidr.split('/')
  const mask = parseInt(maskStr)
  if (isNaN(mask) || mask < 0 || mask > 32) return false
  const ipInt = ipToInt(ip)
  const netInt = ipToInt(netIP)
  if (isNaN(ipInt) || isNaN(netInt)) return false
  const maskInt = mask === 0 ? 0 : (~0 << (32 - mask)) >>> 0
  return (ipInt & maskInt) === (netInt & maskInt)
}
