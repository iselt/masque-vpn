import { createI18n } from 'vue-i18n';
import enLocale from './locales/en.json'; // 恢复导入
import zhLocale from './locales/zh.json'; // 恢复导入
// 从 element-plus/es/locale/lang/ 导入区域设置文件
import elementEnLocale from 'element-plus/es/locale/lang/en';
import elementZhLocale from 'element-plus/es/locale/lang/zh-cn';

// 定义 Element Plus 区域设置的映射
export const elementPlusLocales = {
  en: elementEnLocale,
  zh: elementZhLocale
};

// 恢复使用从 JSON 文件导入的 messages 对象
const messages = {
  en: {
    ...enLocale,
    // el: elementEnLocale // Element Plus 自己的国际化消息可以这样合并，但通常 ElementPlus 组件会自己处理
  },
  zh: {
    ...zhLocale,
    // el: elementZhLocale
  }
};

const i18n = createI18n({
  locale: localStorage.getItem('locale') || 'zh',
  fallbackLocale: 'en',
  messages, // 使用从 JSON 文件加载的 messages
  legacy: false,
  runtimeOnly: false, 
  globalInjection: true,
  silentTranslationWarn: false, 
  missingWarn: true,
  fallbackWarn: true 
});

export default i18n;
