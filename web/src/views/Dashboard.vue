<template>
  <div>
    <el-row :gutter="16">
      <el-col :span="8">
        <el-card><el-statistic title="Total Calls" :value="stats.total_calls" /></el-card>
      </el-col>
      <el-col :span="8">
        <el-card><el-statistic title="Errors" :value="stats.error_count" /></el-card>
      </el-col>
      <el-col :span="8">
        <el-card><el-statistic title="Avg Latency (ms)" :value="stats.avg_latency_ms" /></el-card>
      </el-col>
    </el-row>
    <el-card style="margin-top: 16px">
      <template #header>Calls per tool</template>
      <ul>
        <li v-for="(c, name) in stats.tool_counts" :key="name">{{ name }}: {{ c }}</li>
        <li v-if="!stats.tool_counts || !Object.keys(stats.tool_counts).length">No data yet</li>
      </ul>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { fetchStats, type Stats } from '../api'

const stats = ref<Stats>({ total_calls: 0, error_count: 0, avg_latency_ms: 0, tool_counts: {} })

onMounted(async () => {
  stats.value = await fetchStats()
})
</script>
