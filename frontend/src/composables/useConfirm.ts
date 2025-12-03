import { ref } from "vue";

interface ConfirmOptions {
  title?: string;
  message: string;
  confirmText?: string;
  cancelText?: string;
  variant?: "danger" | "warning" | "info";
  onConfirm: () => Promise<void> | void;
}

const showConfirmModal = ref(false);
const confirmOptions = ref<ConfirmOptions>({
  message: "",
  onConfirm: () => {},
});
const isConfirmLoading = ref(false);

export function useConfirm() {
  const confirm = async (options: ConfirmOptions) => {
    return new Promise<boolean>((resolve) => {
      confirmOptions.value = options;
      showConfirmModal.value = true;

      const handleConfirm = async () => {
        try {
          isConfirmLoading.value = true;
          await options.onConfirm();
          showConfirmModal.value = false;
          resolve(true);
        } catch (error) {
          console.error("Confirm action failed:", error);
          throw error;
        } finally {
          isConfirmLoading.value = false;
        }
      };

      const handleCancel = () => {
        showConfirmModal.value = false;
        resolve(false);
      };

      // Store handlers for component to call
      (confirmOptions.value as any)._handleConfirm = handleConfirm;
      (confirmOptions.value as any)._handleCancel = handleCancel;
    });
  };

  return {
    confirm,
    showConfirmModal,
    confirmOptions,
    isConfirmLoading,
  };
}
