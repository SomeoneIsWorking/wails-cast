import { Ref, onMounted, onUnmounted } from 'vue'
import { isAcceptedFile } from '../utils/file'

type DropEventDetail = { x: number; y: number; paths: string[] }

interface Options {
  dropZoneRef: Ref<HTMLElement | null>
  acceptedExtensions: string[]
  onDrop: (paths: string[]) => void
}

export function useOnFileDrop({ dropZoneRef, acceptedExtensions, onDrop }: Options) {

  const listener = (evt: Event) => {
    const e = evt as CustomEvent<DropEventDetail>
    const paths = e?.detail?.paths
    if (!paths || paths.length === 0) return

    if (dropZoneRef && dropZoneRef.value) {
      const rect = dropZoneRef.value.getBoundingClientRect()
      const x = e.detail.x
      const y = e.detail.y
      if (x < rect.left || x > rect.right || y < rect.top || y > rect.bottom) {
        return
      }
    }

    const accepted = paths.filter(p => isAcceptedFile(p, acceptedExtensions))
    if (accepted.length > 0) {
      onDrop(accepted)
    }
  }

  onMounted(() => {
    window.addEventListener('wails-file-drop', listener as EventListener)
  })

  onUnmounted(() => {
    window.removeEventListener('wails-file-drop', listener as EventListener)
  })
}

export default useOnFileDrop
