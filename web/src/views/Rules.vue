<template>
  <div>
    <el-alert
      v-if="error"
      type="error"
      :closable="false"
      :title="error"
      style="margin-bottom: 16px"
    />
    <el-alert
      type="info"
      :closable="false"
      title="Rules are stored in the database and hot-reloaded — edits take effect on the next tool call, no restart needed. Rules seeded from config.yaml are marked source=config."
      style="margin-bottom: 16px"
    />

    <div style="margin-bottom: 12px">
      <el-button type="primary" @click="openCreate">New rule</el-button>
      <el-button @click="load">Refresh</el-button>
    </div>

    <el-table :data="rows" style="width: 100%" v-loading="loading">
      <el-table-column prop="id" label="ID" width="70" />
      <el-table-column prop="name" label="Name" width="160" />
      <el-table-column label="On" width="80">
        <template #default="{ row }">
          <el-switch
            v-model="row.enabled"
            @change="(v: boolean) => toggle(row, v)"
          />
        </template>
      </el-table-column>
      <el-table-column label="Patterns">
        <template #default="{ row }">
          <el-tag v-for="p in row.patterns" :key="p" size="small" style="margin: 2px">
            {{ p.length > 40 ? p.slice(0, 40) + '…' : p }}
          </el-tag>
          <span v-if="!row.patterns?.length">—</span>
        </template>
      </el-table-column>
      <el-table-column label="Fields" width="200">
        <template #default="{ row }">
          {{ row.fields?.join(', ') || '—' }}
        </template>
      </el-table-column>
      <el-table-column prop="mask_char" label="Mask" width="140" />
      <el-table-column prop="source" label="Source" width="100" />
      <el-table-column label="Actions" width="140">
        <template #default="{ row }">
          <el-button size="small" @click="openEdit(row)">Edit</el-button>
          <el-button size="small" type="danger" @click="remove(row)">Delete</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="dialog" :title="editing ? 'Edit rule' : 'New rule'" width="560">
      <el-form :model="form" label-width="110px">
        <el-form-item label="Name">
          <el-input v-model="form.name" placeholder="e.g. phone_cn" />
        </el-form-item>
        <el-form-item label="Patterns">
          <el-input
            v-model="form.patternsText"
            type="textarea"
            :rows="3"
            placeholder="One regex per line"
          />
        </el-form-item>
        <el-form-item label="Fields">
          <el-input v-model="form.fieldsText" placeholder="Comma separated field names" />
        </el-form-item>
        <el-form-item label="Mask char">
          <el-input v-model="form.mask_char" placeholder="****" />
        </el-form-item>
        <el-form-item label="Enabled">
          <el-switch v-model="form.enabled" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialog = false">Cancel</el-button>
        <el-button type="primary" :loading="saving" @click="save">Save</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  createRule,
  deleteRule,
  fetchRules,
  updateRule,
  type MaskRule,
} from '../api'

const rows = ref<MaskRule[]>([])
const loading = ref(false)
const saving = ref(false)
const dialog = ref(false)
const editing = ref<MaskRule | null>(null)
const error = ref('')

const form = reactive({
  name: '',
  patternsText: '',
  fieldsText: '',
  mask_char: '****',
  enabled: true,
})

function splitLines(s: string) {
  return s
    .split('\n')
    .map((l) => l.trim())
    .filter(Boolean)
}

function splitFields(s: string) {
  return s
    .split(',')
    .map((l) => l.trim())
    .filter(Boolean)
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    rows.value = await fetchRules()
  } catch (e: any) {
    error.value = String(e?.response?.data?.error || e)
  } finally {
    loading.value = false
  }
}

function openCreate() {
  editing.value = null
  form.name = ''
  form.patternsText = ''
  form.fieldsText = ''
  form.mask_char = '****'
  form.enabled = true
  dialog.value = true
}

function openEdit(row: MaskRule) {
  editing.value = row
  form.name = row.name
  form.patternsText = (row.patterns || []).join('\n')
  form.fieldsText = (row.fields || []).join(', ')
  form.mask_char = row.mask_char
  form.enabled = row.enabled
  dialog.value = true
}

async function save() {
  saving.value = true
  const payload = {
    name: form.name,
    patterns: splitLines(form.patternsText),
    fields: splitFields(form.fieldsText),
    mask_char: form.mask_char,
    enabled: form.enabled,
  }
  try {
    if (editing.value) {
      await updateRule(editing.value.id, payload)
      ElMessage.success('Rule updated')
    } else {
      await createRule(payload)
      ElMessage.success('Rule created')
    }
    dialog.value = false
    await load()
  } catch (e: any) {
    ElMessage.error(String(e?.response?.data?.error || e))
  } finally {
    saving.value = false
  }
}

async function toggle(row: MaskRule, value: boolean) {
  try {
    await updateRule(row.id, {
      name: row.name,
      patterns: row.patterns,
      fields: row.fields,
      mask_char: row.mask_char,
      enabled: value,
    })
    ElMessage.success(value ? 'Rule enabled' : 'Rule disabled')
    await load()
  } catch (e: any) {
    row.enabled = !value
    ElMessage.error(String(e?.response?.data?.error || e))
  }
}

async function remove(row: MaskRule) {
  try {
    await ElMessageBox.confirm(`Delete rule "${row.name}"?`, 'Confirm', {
      type: 'warning',
    })
  } catch {
    return
  }
  try {
    await deleteRule(row.id)
    ElMessage.success('Rule deleted')
    await load()
  } catch (e: any) {
    ElMessage.error(String(e?.response?.data?.error || e))
  }
}

onMounted(load)
</script>
