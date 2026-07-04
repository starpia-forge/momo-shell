/** POSIX single-quote escaping for injecting a local path into a shell
 * command line -- used when a file is dropped on a *local* pane, where the
 * conventional behavior is to type the quoted path rather than upload it. */
export function shellQuotePath(path: string): string {
  return `'${path.replace(/'/g, `'\\''`)}'`
}
