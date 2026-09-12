<template>
  <div>
    <el-alert
      type="info"
      :closable="false"
      title="Masking & rate-limit rules are configured in config.yaml. This screen is a read-only placeholder for v0.1 (CRUD coming in v0.2)."
    />
    <pre style="margin-top: 16px; background: #f6f8fa; padding: 16px; border-radius: 6px">{{ rulesYaml }}</pre>
  </div>
</template>

<script setup lang="ts">
const rulesYaml = `# masking.rules in config.yaml
- name: email
  patterns: ["\\\\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\\\\.[A-Za-z]{2,}\\\\b"]
  mask_char: "***@***.com"
- name: sensitive_fields
  fields: [password, secret, token]
  mask_char: "****"`
</script>
