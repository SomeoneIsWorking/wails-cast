<script setup lang="ts">
import { computed } from "vue";

const props = defineProps<{
  proress: number;
  total: number;
}>();
const value = computed(() => {
  if (props.total === 0) return 0;
  return (props.proress / props.total) * 100;
});
</script>

<template>
  <div class="relative justify-stretch [&>div]:col-0 [&>div]:row-0">
    <div class="w-full bg-gray-600 rounded-full">
      <div
        class="bg-blue-500 h-5 rounded-full transition-all duration-300"
        :style="{
          width: value + '%',
        }"
      ></div>
    </div>
    <div class="absolute inset-0 text-center text-sm">{{ proress }} / {{ total }}</div>
  </div>
</template>
