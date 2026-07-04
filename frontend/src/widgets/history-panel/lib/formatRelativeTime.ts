/** executedAt is Unix seconds (matches the backend's storage unit). */
export function formatRelativeTime(executedAt: number): string {
  const diffMin = Math.floor((Date.now() - executedAt * 1000) / 60000)
  if (diffMin < 1) return '방금'
  if (diffMin < 60) return `${diffMin}분 전`
  const diffHour = Math.floor(diffMin / 60)
  if (diffHour < 24) return `${diffHour}시간 전`
  const diffDay = Math.floor(diffHour / 24)
  if (diffDay === 1) return '어제'
  return `${diffDay}일 전`
}
