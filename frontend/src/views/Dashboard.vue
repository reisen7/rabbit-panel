<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, shallowRef, watch } from 'vue'
import { Container, Cpu, HardDrive, MemoryStick } from 'lucide-vue-next'
import * as echarts from 'echarts'
import { useSystemStore } from '@/stores/system'
import { useContainerStore } from '@/stores/containers'
import { useTheme } from '@/composables/useTheme'
import { useI18n } from '@/composables/useI18n'
import UpdateBanner from '@/components/common/UpdateBanner.vue'
import UpdateDialog from '@/components/common/UpdateDialog.vue'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Progress } from '@/components/ui/progress'

const { t } = useI18n()
const { theme } = useTheme()
const systemStore = useSystemStore()
const containerStore = useContainerStore()

const lineChartRef = ref<HTMLElement | null>(null)
const pieChartRef = ref<HTMLElement | null>(null)
const gaugeChartRef = ref<HTMLElement | null>(null)
const lineChart = shallowRef<echarts.ECharts | null>(null)
const pieChart = shallowRef<echarts.ECharts | null>(null)
const gaugeChart = shallowRef<echarts.ECharts | null>(null)
const cpuHistory = ref<number[]>([])
const memoryHistory = ref<number[]>([])
const historyLabels = ref<string[]>([])
const showUpdateDialog = ref(false)
const maxHistoryLength = 20

const chartPalette = computed(() => {
  const isDark = theme.value === 'dark'
  return {
    text: isDark ? '#a1a1aa' : '#71717a',
    strongText: isDark ? '#fafafa' : '#18181b',
    splitLine: isDark ? 'rgba(255,255,255,0.08)' : 'rgba(0,0,0,0.06)',
    cardBorder: isDark ? '#27272a' : '#ffffff',
    track: isDark ? 'rgba(255,255,255,0.1)' : '#e4e4e7',
    empty: isDark ? '#3f3f46' : '#e4e4e7',
  }
})

function formatSize(kb: number): string {
  if (kb === 0) return '0 B'
  const gb = kb / 1024 / 1024
  if (gb >= 1) return `${gb.toFixed(1)} GB`
  const mb = kb / 1024
  if (mb >= 1) return `${mb.toFixed(1)} MB`
  return `${kb.toFixed(0)} KB`
}

function updateHistory() {
  const now = new Date()
  const timeLabel = `${now.getHours().toString().padStart(2, '0')}:${now.getMinutes().toString().padStart(2, '0')}:${now.getSeconds().toString().padStart(2, '0')}`
  cpuHistory.value.push(systemStore.stats.cpu)
  memoryHistory.value.push(systemStore.stats.memory)
  historyLabels.value.push(timeLabel)

  if (cpuHistory.value.length > maxHistoryLength) {
    cpuHistory.value.shift()
    memoryHistory.value.shift()
    historyLabels.value.shift()
  }

  updateLineChart()
  updateGaugeChart()
}

function initLineChart() {
  if (!lineChartRef.value) return
  lineChart.value = echarts.init(lineChartRef.value)
  updateLineChart()
}

function updateLineChart() {
  if (!lineChart.value) return
  const colors = chartPalette.value
  const option: echarts.EChartsOption = {
    tooltip: {
      trigger: 'axis',
      formatter: (params: unknown) => {
        if (!Array.isArray(params)) return ''
        let result = (params[0] as { axisValue?: string })?.axisValue || ''
        params.forEach((item: { marker?: string; seriesName?: string; value?: number }) => {
          result += `<br/>${item.marker}${item.seriesName}: ${(item.value ?? 0).toFixed(1)}%`
        })
        return result
      },
    },
    legend: {
      data: ['CPU', t('system.memory')],
      bottom: 0,
      textStyle: { color: colors.text, fontSize: 12 },
    },
    grid: { left: '3%', right: '4%', bottom: '15%', top: '10%', containLabel: true },
    xAxis: {
      type: 'category',
      boundaryGap: false,
      data: historyLabels.value,
      axisLabel: { fontSize: 10, rotate: 30, color: colors.text },
      axisLine: { lineStyle: { color: colors.splitLine } },
    },
    yAxis: {
      type: 'value',
      min: 0,
      max: 100,
      axisLabel: { formatter: '{value}%', color: colors.text, fontSize: 11 },
      splitLine: { lineStyle: { color: colors.splitLine } },
    },
    series: [
      {
        name: 'CPU',
        type: 'line',
        smooth: true,
        showSymbol: false,
        data: cpuHistory.value,
        itemStyle: { color: '#3b82f6' },
        areaStyle: {
          color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
            { offset: 0, color: 'rgba(59,130,246,0.28)' },
            { offset: 1, color: 'rgba(59,130,246,0.02)' },
          ]),
        },
      },
      {
        name: t('system.memory'),
        type: 'line',
        smooth: true,
        showSymbol: false,
        data: memoryHistory.value,
        itemStyle: { color: '#10b981' },
        areaStyle: {
          color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
            { offset: 0, color: 'rgba(16,185,129,0.28)' },
            { offset: 1, color: 'rgba(16,185,129,0.02)' },
          ]),
        },
      },
    ],
  }
  lineChart.value.setOption(option)
}

function initPieChart() {
  if (!pieChartRef.value) return
  pieChart.value = echarts.init(pieChartRef.value)
  updatePieChart()
}

function updatePieChart() {
  if (!pieChart.value) return
  const colors = chartPalette.value
  const stats = containerStats.value
  const pieData = [
    { value: stats.running, name: t('container.running'), itemStyle: { color: '#10b981' } },
    { value: stats.stopped, name: t('container.stopped'), itemStyle: { color: '#ef4444' } },
    { value: stats.paused, name: t('container.paused'), itemStyle: { color: '#f59e0b' } },
  ]
  const displayData = pieData.filter((item) => item.value > 0)
  if (displayData.length === 0) {
    displayData.push({ value: 1, name: 'No Containers', itemStyle: { color: colors.empty } })
  }

  const option: echarts.EChartsOption = {
    tooltip: { trigger: 'item', formatter: '{b}: {c} ({d}%)' },
    legend: {
      bottom: 0,
      itemWidth: 10,
      itemHeight: 10,
      textStyle: { color: colors.text, fontSize: 12 },
      data: pieData.map((item) => item.name),
    },
    series: [
      {
        type: 'pie',
        radius: ['45%', '72%'],
        center: ['50%', '44%'],
        itemStyle: { borderRadius: 6, borderColor: colors.cardBorder, borderWidth: 2 },
        label: {
          show: true,
          position: 'center',
          formatter: () => `${stats.total}\nContainers`,
          fontSize: 15,
          fontWeight: 'bold',
          color: colors.strongText,
        },
        data: displayData,
      },
    ],
  }
  pieChart.value.setOption(option)
}

function initGaugeChart() {
  if (!gaugeChartRef.value) return
  gaugeChart.value = echarts.init(gaugeChartRef.value)
  updateGaugeChart()
}

function updateGaugeChart() {
  if (!gaugeChart.value) return
  const colors = chartPalette.value
  const diskValue = systemStore.stats.disk
  const option: echarts.EChartsOption = {
    series: [
      {
        type: 'gauge',
        startAngle: 200,
        endAngle: -20,
        min: 0,
        max: 100,
        itemStyle: { color: diskValue >= 90 ? '#ef4444' : diskValue >= 70 ? '#f59e0b' : '#10b981' },
        progress: { show: true, width: 18, roundCap: true },
        pointer: { show: false },
        axisLine: { roundCap: true, lineStyle: { width: 18, color: [[1, colors.track]] } },
        axisTick: { show: false },
        splitLine: { show: false },
        axisLabel: { show: false },
        title: { show: true, offsetCenter: [0, '30%'], fontSize: 13, color: colors.text },
        detail: {
          valueAnimation: true,
          offsetCenter: [0, '-10%'],
          fontSize: 28,
          fontWeight: 'bold',
          formatter: '{value}%',
          color: colors.strongText,
        },
        data: [{ value: diskValue, name: `${t('system.disk')} ${t('system.usage')}` }],
      },
    ],
  }
  gaugeChart.value.setOption(option)
}

function handleResize() {
  lineChart.value?.resize()
  pieChart.value?.resize()
  gaugeChart.value?.resize()
}

function getProgressIndicatorClass(percent: number): string {
  if (percent >= 90) return 'bg-destructive'
  if (percent >= 70) return 'bg-amber-500'
  return 'bg-emerald-500'
}

const containerStats = computed(() => {
  const containers = containerStore.containers
  const running = containers.filter((c) => c.state === 'running').length
  const paused = containers.filter((c) => c.state === 'paused').length
  const stopped = containers.filter((c) => c.state !== 'running' && c.state !== 'paused').length
  return { total: containers.length, running, stopped, paused }
})

let historyTimer: ReturnType<typeof setInterval> | null = null

onMounted(() => {
  systemStore.startPolling()
  containerStore.fetchContainers()
  for (let i = 0; i < 5; i++) {
    cpuHistory.value.push(0)
    memoryHistory.value.push(0)
    historyLabels.value.push('--:--:--')
  }
  setTimeout(() => {
    initLineChart()
    initPieChart()
    initGaugeChart()
  }, 100)
  historyTimer = setInterval(updateHistory, 5000)
  setTimeout(updateHistory, 1000)
  window.addEventListener('resize', handleResize)
})

onUnmounted(() => {
  systemStore.stopPolling()
  if (historyTimer) clearInterval(historyTimer)
  window.removeEventListener('resize', handleResize)
  lineChart.value?.dispose()
  pieChart.value?.dispose()
  gaugeChart.value?.dispose()
})

watch(() => containerStore.containers, () => updatePieChart(), { deep: true })
watch(theme, () => {
  updateLineChart()
  updatePieChart()
  updateGaugeChart()
})
</script>

<template>
  <div class="space-y-5">
    <UpdateBanner @open-dialog="showUpdateDialog = true" />

    <!-- KPI Cards -->
    <div class="grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-4 gap-4">
      <Card class="gap-3 py-5">
        <CardHeader class="flex flex-row items-center justify-between space-y-0 pb-0">
          <CardTitle class="text-sm font-medium text-muted-foreground">
            CPU {{ t('system.usage') }}
          </CardTitle>
          <div class="size-8 rounded-lg bg-blue-500/10 text-blue-600 dark:text-blue-400 flex items-center justify-center">
            <Cpu class="size-4" />
          </div>
        </CardHeader>
        <CardContent class="space-y-3">
          <div class="text-2xl font-bold tracking-tight tabular-nums">
            {{ systemStore.stats.cpu.toFixed(1) }}%
          </div>
          <Progress
            :model-value="systemStore.stats.cpu"
            class="h-1.5"
            :indicator-class="getProgressIndicatorClass(systemStore.stats.cpu)"
          />
        </CardContent>
      </Card>

      <Card class="gap-3 py-5">
        <CardHeader class="flex flex-row items-center justify-between space-y-0 pb-0">
          <CardTitle class="text-sm font-medium text-muted-foreground">
            {{ t('system.memory') }} {{ t('system.usage') }}
          </CardTitle>
          <div class="size-8 rounded-lg bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 flex items-center justify-center">
            <MemoryStick class="size-4" />
          </div>
        </CardHeader>
        <CardContent class="space-y-3">
          <div class="flex items-baseline justify-between gap-2">
            <span class="text-2xl font-bold tracking-tight tabular-nums">
              {{ systemStore.stats.memory.toFixed(1) }}%
            </span>
            <span class="text-xs text-muted-foreground tabular-nums">
              {{ formatSize(systemStore.stats.memoryUsed) }} / {{ formatSize(systemStore.stats.memoryTotal) }}
            </span>
          </div>
          <Progress
            :model-value="systemStore.stats.memory"
            class="h-1.5"
            :indicator-class="getProgressIndicatorClass(systemStore.stats.memory)"
          />
        </CardContent>
      </Card>

      <Card class="gap-3 py-5">
        <CardHeader class="flex flex-row items-center justify-between space-y-0 pb-0">
          <CardTitle class="text-sm font-medium text-muted-foreground">
            {{ t('system.disk') }} {{ t('system.usage') }}
          </CardTitle>
          <div class="size-8 rounded-lg bg-amber-500/10 text-amber-600 dark:text-amber-400 flex items-center justify-center">
            <HardDrive class="size-4" />
          </div>
        </CardHeader>
        <CardContent class="space-y-3">
          <div class="flex items-baseline justify-between gap-2">
            <span class="text-2xl font-bold tracking-tight tabular-nums">
              {{ systemStore.stats.disk.toFixed(1) }}%
            </span>
            <span class="text-xs text-muted-foreground tabular-nums">
              {{ formatSize(systemStore.stats.diskUsed) }} / {{ formatSize(systemStore.stats.diskTotal) }}
            </span>
          </div>
          <Progress
            :model-value="systemStore.stats.disk"
            class="h-1.5"
            :indicator-class="getProgressIndicatorClass(systemStore.stats.disk)"
          />
        </CardContent>
      </Card>

      <Card class="gap-3 py-5">
        <CardHeader class="flex flex-row items-center justify-between space-y-0 pb-0">
          <CardTitle class="text-sm font-medium text-muted-foreground">
            {{ t('nav.containers') }}
          </CardTitle>
          <div class="size-8 rounded-lg bg-muted text-foreground flex items-center justify-center">
            <Container class="size-4" />
          </div>
        </CardHeader>
        <CardContent class="space-y-2.5">
          <div class="text-2xl font-bold tracking-tight tabular-nums">
            {{ containerStats.running }} / {{ containerStats.total }}
          </div>
          <div class="flex flex-wrap gap-1.5">
            <Badge variant="success">{{ t('container.running') }}: {{ containerStats.running }}</Badge>
            <Badge variant="destructive">{{ t('container.stopped') }}: {{ containerStats.stopped }}</Badge>
          </div>
        </CardContent>
      </Card>
    </div>

    <!-- Charts Row 1 -->
    <div class="grid grid-cols-1 lg:grid-cols-3 gap-4">
      <Card class="lg:col-span-2">
        <CardHeader>
          <CardTitle class="text-base">
            CPU / {{ t('system.memory') }} {{ t('system.usage') }}
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div ref="lineChartRef" class="w-full h-[250px]" />
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle class="text-base">{{ t('nav.containers') }}</CardTitle>
        </CardHeader>
        <CardContent>
          <div ref="pieChartRef" class="w-full h-[250px]" />
        </CardContent>
      </Card>
    </div>

    <!-- Charts Row 2 -->
    <div class="grid grid-cols-1 lg:grid-cols-2 gap-4">
      <Card>
        <CardHeader>
          <CardTitle class="text-base">
            {{ t('system.disk') }} {{ t('system.usage') }}
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div ref="gaugeChartRef" class="w-full h-[250px]" />
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle class="text-base">Server Info</CardTitle>
        </CardHeader>
        <CardContent>
          <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <div class="rounded-lg border bg-muted/30 p-4 space-y-1">
              <div class="text-xs text-muted-foreground">{{ t('system.serverTime') }}</div>
              <div class="text-lg font-semibold font-mono tabular-nums">
                {{ systemStore.stats.time || '--' }}
              </div>
            </div>
            <div class="rounded-lg border bg-muted/30 p-4 space-y-1">
              <div class="text-xs text-muted-foreground">Containers</div>
              <div class="text-lg font-semibold tabular-nums">{{ containerStats.total }}</div>
            </div>
            <div class="rounded-lg border bg-muted/30 p-4 space-y-1">
              <div class="text-xs text-muted-foreground">Running</div>
              <div class="text-lg font-semibold tabular-nums text-emerald-600 dark:text-emerald-400">
                {{ containerStats.running }}
              </div>
            </div>
            <div class="rounded-lg border bg-muted/30 p-4 space-y-1">
              <div class="text-xs text-muted-foreground">Stopped</div>
              <div class="text-lg font-semibold tabular-nums text-destructive">
                {{ containerStats.stopped }}
              </div>
            </div>
          </div>
        </CardContent>
      </Card>
    </div>
  </div>

  <UpdateDialog v-model="showUpdateDialog" />
</template>
