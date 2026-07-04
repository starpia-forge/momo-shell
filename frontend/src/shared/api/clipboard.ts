import { SetClipboardText } from '../../../wailsjs/go/wails/ClipboardService'

export async function setClipboardText(text: string): Promise<void> {
  await SetClipboardText(text)
}
