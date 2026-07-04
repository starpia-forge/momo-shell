/** Parses an OSC 7 payload ("file://host/path") into the decoded absolute
 * path, or null if it doesn't match the expected shape. */
export function parseOsc7Path(data: string): string | null {
  const match = /^file:\/\/[^/]*(\/.*)$/.exec(data)
  if (!match) return null
  try {
    return decodeURIComponent(match[1])
  } catch {
    return match[1]
  }
}
