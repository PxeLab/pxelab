import { type FC } from 'react'
import { useTranslation } from 'react-i18next'
import { AlertTriangle } from 'lucide-react'
import { Modal } from './Modal'
import { Button } from './Button'

interface Props {
  open: boolean
  onClose: () => void
  onConfirm: () => void
  title?: string
  message: string
  confirmLabel?: string
  danger?: boolean
  loading?: boolean
}

export const ConfirmDialog: FC<Props> = ({
  open,
  onClose,
  onConfirm,
  title,
  message,
  confirmLabel,
  danger = true,
  loading = false,
}) => {
  const { t } = useTranslation()
  return (
    <Modal
      open={open}
      onClose={onClose}
      title={title || t('common.confirmAction')}
      width="400px"
      footer={
        <>
          <Button variant="secondary" size="sm" onClick={onClose}>{t('common.cancel')}</Button>
          <Button variant={danger ? 'danger' : 'primary'} size="sm" onClick={onConfirm} disabled={loading}>
            {loading ? t('common.processing') : (confirmLabel || t('common.confirm'))}
          </Button>
        </>
      }
    >
      <div className="flex items-start gap-3">
        <div className="shrink-0 mt-0.5">
          <AlertTriangle size={18} className="text-amber-400" />
        </div>
        <p className="text-sm text-[var(--text-secondary)]">{message}</p>
      </div>
    </Modal>
  )
}
