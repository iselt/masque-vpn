import { createApp, watch } from 'vue'
import { createPinia } from 'pinia'
import ElementPlus from 'element-plus'
import 'element-plus/dist/index.css'
import App from './App.vue'
import router from './router'
import i18n from './i18n' // 引入 i18n 实例
import { elementPlusLocales } from './i18n' // 引入 Element Plus 区域设置
import * as ElementPlusIconsVue from '@element-plus/icons-vue'
import { useUserStore } from '@/store/user'; // 新增：导入 user store
import { setupInterceptors } from '@/api' // 导入 setupInterceptors

const app = createApp(App)

// 注册所有 Element Plus 图标
for (const [key, component] of Object.entries(ElementPlusIconsVue)) {
  app.component(key, component)
}

app.use(createPinia()) // Pinia 必须先于 store 的使用

// 新增：在路由和应用挂载前检查认证状态
const userStore = useUserStore()

// 调用 setupInterceptors 并传入 router 实例
setupInterceptors(router)

userStore.checkAuthStatus().then(() => {
  app.use(router)
  app.use(i18n)

  const currentLocale = i18n.global.locale.value // 正确访问 locale
  app.use(ElementPlus, { locale: elementPlusLocales[currentLocale as 'en' | 'zh'] || elementPlusLocales.en })

  app.mount('#app')

  // 监听语言切换事件，动态更新 Element Plus 的语言环境
  watch(
    () => i18n.global.locale.value,
    (newLocale) => {
      // Element Plus 的 locale 通常在初始化时设置。
      // 动态更改 Element Plus 的区域设置比较复杂，通常需要重新加载组件或页面，
      // 或者 Element Plus 自身提供特定的 API 来动态更新。
      // 对于大多数应用，初始化时设置一次，并在用户切换语言后提示刷新页面可能是更简单的做法。
      console.log('Language changed to:', newLocale)
      // console.log('Current Element Plus locale config:', app.config.globalProperties.$ELEMENT?.locale)
      // 如果 Element Plus 支持动态切换，类似这样:
      // app.config.globalProperties.$ELEMENT.locale = elementPlusLocales[newLocale as 'en' | 'zh'] || elementPlusLocales.en
      // 但更推荐的方式是，如果i18n的locale是响应式的，并且ElementPlus的locale配置也接受一个ref，那它会自动更新。
      // 查阅Element Plus文档确认最佳实践。目前，我们仅在app初始化时设置。
    }
  )
});
