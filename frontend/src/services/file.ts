import { OpenFileDialog, OpenDirectoryDialog } from '../../wailsjs/go/main/App'

export const fileService = {
  async selectMediaFile(): Promise<string> {
    return await OpenFileDialog('Select Video File', [
      '*.mp4', '*.mkv', '*.avi', '*.mov', '*.flv', '*.webm', '*.m4v'
    ])
  },

  async selectMediaFolder(): Promise<string> {
    return await OpenDirectoryDialog()
  },
}
