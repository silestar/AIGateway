import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import fs from 'fs'
import path from 'path'

// 从 docs/VERSION 读取版本号
const version = (() => {
  try {
    const p = path.resolve(__dirname, '../docs/VERSION')
    return fs.readFileSync(p, 'utf-8').trim()
  } catch {
    return '0.1.0'
  }
})()

// https://vite.dev/config/
export default defineConfig({
  plugins: [vue()],
  define: {
    __APP_VERSION__: JSON.stringify(version),
  },
})