<template>
  <div>
    <el-table :data="rows" height="600" style="width: 100%">
      <el-table-column prop="id" label="ID" width="80" />
      <el-table-column prop="client_id" label="Client" width="120" />
      <el-table-column prop="tool_name" label="Tool" width="160" />
      <el-table-column prop="timestamp" label="Time" width="220" />
      <el-table-column prop="latency_ms" label="Latency" width="100" />
      <el-table-column label="Params / Result">
        <template #default="{ row }">
          <el-popover trigger="hover" width="460" placement="left">
            <pre style="white-space: pre-wrap; max-height: 320px; overflow: auto">{{ row.params }}

--- RESULT ---
{{ row.result }}</pre>
            <template #reference><span>{{ preview(row.params) }}</span></template>
          </el-popover>
        </template>
      </el-table-column>
      <el-table-column label="Replay" width="100">
        <template #default="{ row }">
          <el-button size="small" :loading="busy === row.id" @click="replay(row)">Replay</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="dialogVisible" title="Replay response" width="60%">
      <pre style="white-space: pre-wrap; max-height: 420px; overflow: auto">{{ replayText }}</pre>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { fetchLogs, fetchReplay, type CallRecord } from '../api'

const rows = ref<CallRecord[]>([])
const dialogVisible = ref(false)
const replayText = ref('')
const busy = ref<number | null>(null)
const preview = (s: string) => (s && s.length > 48 ? s.slice(0, 48) + '…' : s || '')

async function replay(row: CallRecord) {
  busy.value = row.id
  try {
    const data = await fetchReplay(row.id)
    replayText.value = JSON.stringify(data.response, null, 2)
  } catch (e: any) {
    replayText.value = String(e?.response?.data?.error || e)
  } finally {
    busy.value = null
    dialogVisible.value = true
  }
}

onMounted(async () => {
  rows.value = await fetchLogs({ limit: 200 })
})
</script>
