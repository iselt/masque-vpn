import { defineStore } from 'pinia'
import apiClient from '@/api' // 确保 apiClient 已正确配置

interface UserState {
  username: string | null
  isAuthenticated: boolean // 显式维护认证状态
}

export const useUserStore = defineStore('user', {
  state: (): UserState => ({
    username: null,
    isAuthenticated: false, // 初始化为 false
  }),
  getters: {
    isLoggedIn(): boolean {
      return this.isAuthenticated
    },
    getUsername(): string | null {
      return this.username;
    }
  },
  actions: {
    loginSuccess(username: string) {
      this.username = username
      this.isAuthenticated = true
      // HttpOnly cookie 由服务器设置
    },

    async logout() {
      console.log('[UserStore] logout action CALLED. Current isAuthenticated before logout:', this.isAuthenticated);
      console.trace('[UserStore] Call stack for logout:'); // 这会打印调用栈    
      try {
        await apiClient.post('/logout') // 调用后端登出 API
      } catch (error) {
        console.error('Logout API call failed:', error)
        // 即使API调用失败，也清理客户端状态
      }
      this.username = null
      this.isAuthenticated = false
      // HttpOnly cookie 的移除由服务器通过 SetCookie MaxAge=-1 处理
      // router.push('/login'); // 导航应由组件或路由守卫处理
    },

    async checkAuthStatus() {
      try {
        // apiClient.get 直接返回数据对象 { loggedIn: boolean; username?: string }
        const response:any = await apiClient.get('/auth/check');
        console.log('checkAuthStatus response object (expected data object):', response);

        // 直接检查 response.loggedIn
        if (response && response.loggedIn) {
          this.isAuthenticated = true;
          this.username = response.username || null;
          console.log('[UserStore] checkAuthStatus: isAuthenticated successfully SET to true. Current value:', this.isAuthenticated, 'Username:', this.username);
        } else {
          this.isAuthenticated = false;
          this.username = null;
          console.log('[UserStore] checkAuthStatus: Conditions not met (loggedIn is false or response structure issue). Response was:', response, '. isAuthenticated SET to false.');
        }
      } catch (error: any) {
        // API调用失败 (例如401 Unauthorized, 404 Not Found, 网络错误等)
        // 均视为用户未认证
        this.isAuthenticated = false
        this.username = null
        console.error('[UserStore] checkAuthStatus: API call failed. Error:', error)
        // 此处不需要 ElMessage，因为这是一个后台检查。
        // 如果发生错误，路由守卫将处理重定向。
        // 全局的 apiClient 拦截器可能会对非401错误显示消息。
      }
    },
  },
})
