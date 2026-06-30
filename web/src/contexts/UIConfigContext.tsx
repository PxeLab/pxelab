import { createContext, useContext, useState, useEffect, type FC, type ReactNode } from 'react'
import { api } from '../api/client'

interface UIConfig {
  pageSize: number
  setPageSize: (size: number) => void
}

const UIConfigContext = createContext<UIConfig>({ pageSize: 50, setPageSize: () => {} })

export const UIConfigProvider: FC<{ children: ReactNode }> = ({ children }) => {
  const [pageSize, setPageSize] = useState(50)

  useEffect(() => {
    api.getGeneralSettings().then(res => {
      if (res.data?.page_size && res.data.page_size > 0) {
        setPageSize(res.data.page_size)
      }
    }).catch(() => {})
  }, [])

  return (
    <UIConfigContext.Provider value={{ pageSize, setPageSize }}>
      {children}
    </UIConfigContext.Provider>
  )
}

export function useUIConfig() {
  return useContext(UIConfigContext)
}
