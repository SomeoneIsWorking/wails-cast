<script setup lang="ts">
import { X, AlertTriangle, Loader2 } from "lucide-vue-next";

interface Props {
  title?: string;
  message: string;
  confirmText?: string;
  cancelText?: string;
  variant?: "danger" | "warning" | "info";
  loading?: boolean;
}

const props = withDefaults(defineProps<Props>(), {
  title: "Confirm Action",
  confirmText: "Confirm",
  cancelText: "Cancel",
  variant: "warning",
  loading: false,
});

const showModal = defineModel<boolean>();

const emit = defineEmits<{
  confirm: [];
  cancel: [];
}>();

const handleConfirm = () => {
  emit("confirm");
};

const handleCancel = () => {
  if (!props.loading) {
    emit("cancel");
  }
};

const getVariantClasses = () => {
  switch (props.variant) {
    case "danger":
      return {
        icon: "text-red-400",
        button: "bg-red-600 hover:bg-red-700",
      };
    case "warning":
      return {
        icon: "text-yellow-400",
        button: "bg-yellow-600 hover:bg-yellow-700",
      };
    case "info":
      return {
        icon: "text-blue-400",
        button: "bg-blue-600 hover:bg-blue-700",
      };
  }
};

const variantClasses = getVariantClasses();
</script>

<template>
  <div
    v-if="showModal"
    class="confirm-overlay"
    @click.self="handleCancel"
  >
    <div class="confirm-modal">
      <!-- Header -->
      <div class="confirm-header">
        <div class="flex items-center gap-3">
          <AlertTriangle :class="['w-6 h-6', variantClasses.icon]" />
          <h2 class="confirm-title">{{ title }}</h2>
        </div>
        <button
          @click="handleCancel"
          :disabled="loading"
          class="btn-close"
        >
          <X class="w-5 h-5" />
        </button>
      </div>

      <!-- Content -->
      <div class="confirm-content">
        <p class="confirm-message">{{ message }}</p>
      </div>

      <!-- Footer -->
      <div class="confirm-footer">
        <button
          @click="handleCancel"
          :disabled="loading"
          class="btn-cancel"
        >
          {{ cancelText }}
        </button>
        <button
          @click="handleConfirm"
          :disabled="loading"
          :class="['btn-confirm', variantClasses.button]"
        >
          <Loader2 v-if="loading" class="w-4 h-4 animate-spin" />
          <span>{{ confirmText }}</span>
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped src="./confirm-modal.css"></style>
