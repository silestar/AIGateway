import { createApp } from 'vue'
import App from './App.vue'
import router from './router'
import i18n from './i18n'
import naive from 'naive-ui'

const app = createApp(App)
app.use(router)
app.use(i18n)
app.use(naive)
app.mount('#app')
