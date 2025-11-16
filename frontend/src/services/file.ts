import { OpenFileDialog, OpenDirectoryDialog } from '../../wailsjs/go/main/App'

export const fileService = {
  async selectMediaFile(): Promise<string> {
    return await OpenFileDialog()
  },

  async selectMediaFolder(): Promise<string> {
    return await OpenDirectoryDialog()
  },
}
