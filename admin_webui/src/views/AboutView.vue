<template>
  <div class="about-container">
    <el-card class="box-card">
      <template #header>
        <div class="card-header">
          <span>{{ t('about.title') }}</span>
        </div>
      </template>
      <div class="content">
        <p>{{ t('appName') }} {{ t('about.version', { version: '1.0.0' }) }}</p>
        <p>{{ t('about.description') }}</p>
        <p>
          {{ t('about.author') }}
        </p>
        <p>
          <a href="https://github.com/iselt/masque-vpn" target="_blank" rel="noopener noreferrer">
            {{ t('about.projectLink') }}
          </a>
        </p>
        <p>
          {{ t('about.techStack') }}
          <ul>
            <li>Vue 3</li>
            <li>Vite</li>
            <li>Element Plus</li>
            <li>Pinia</li>
            <li>TypeScript</li>
            <li>vue-i18n</li>
          </ul>
        </p>
      </div>
    </el-card>
  </div>
</template>

<script lang="ts" setup>
import { useI18n } from 'vue-i18n'

const { t } = useI18n()
</script>

<style scoped>
.about-container {
  padding: 20px;
}
.card-header span {
  font-size: 18px;
  font-weight: bold;
}
.content {
  line-height: 1.8;
}
.content p {
  margin-bottom: 10px;
}
.content ul {
  list-style-type: disc;
  padding-left: 20px;
}
.content a {
  color: #409EFF;
  text-decoration: none;
}
.content a:hover {
  text-decoration: underline;
}
</style>
