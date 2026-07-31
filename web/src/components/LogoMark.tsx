/**
 * PxeLab "Stream" logo mark — a chevron ">" built from 5 pixel packets
 * streaming toward the cursor block (OS image flowing over the network
 * into a bare-metal machine at the boot prompt).
 * Geometry mirrors docs/design/logo/mark.svg (64x64 grid); color via currentColor.
 */
export default function LogoMark({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 64 64" fill="currentColor" className={className} aria-hidden="true">
      <rect x="8" y="8" width="9" height="9" />
      <rect x="16" y="16" width="9" height="9" />
      <rect x="24" y="24" width="9" height="9" />
      <rect x="16" y="32" width="9" height="9" />
      <rect x="8" y="40" width="9" height="9" />
      <rect x="38" y="42" width="18" height="10" />
    </svg>
  )
}
