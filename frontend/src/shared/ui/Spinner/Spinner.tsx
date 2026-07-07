interface SpinnerProps {
  size?: number
}

export function Spinner({ size = 14 }: SpinnerProps) {
  return (
    <span
      className="spinner inline-block rounded-full border-2 border-inputbg border-t-accent animate-spin"
      style={{ width: size, height: size }}
    />
  )
}
