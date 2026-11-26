import { defineStore } from 'pinia'
import { ref } from 'vue'

export type FloatWindowKey = 'agent' | 'imageSearch'

// 右下角同时只允许一个悬浮窗展开，打开一个自动收起另一个
export const useFloatWindowsStore = defineStore('floatWindows', () => {
  const active = ref<FloatWindowKey | null>(null)
  return { active }
})
