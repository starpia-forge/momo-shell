import i18n from '../../../shared/i18n'

/** executedAt is Unix seconds (matches the backend's storage unit). Small
 * enough to duplicate from history-panel's copy rather than import it --
 * widget-to-widget imports aren't allowed under FSD's same-layer rule. */
export function formatRelativeTime(executedAt: number): string {
  const diffMin = Math.floor((Date.now() - executedAt * 1000) / 60000)
  if (diffMin < 1) return i18n.t('relativeTime.justNow')
  if (diffMin < 60) return i18n.t('relativeTime.minutesAgo', { count: diffMin })
  const diffHour = Math.floor(diffMin / 60)
  if (diffHour < 24) return i18n.t('relativeTime.hoursAgo', { count: diffHour })
  const diffDay = Math.floor(diffHour / 24)
  if (diffDay === 1) return i18n.t('relativeTime.yesterday')
  return i18n.t('relativeTime.daysAgo', { count: diffDay })
}
