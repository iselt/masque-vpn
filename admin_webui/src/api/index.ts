import axios from 'axios'
import { ElMessage } from 'element-plus'
import { useUserStore } from '@/store/user'
import type { Router } from 'vue-router' // Import Router type

const apiClient = axios.create({
  baseURL: '/api',
  timeout: 5000,
  withCredentials: true,
})

// Keep the original response data structure for non-error cases
apiClient.interceptors.response.use(
  (response) => response, // Pass through the full response object
  async (error) => { // Make error handler async if store actions are async
    // This interceptor will be further configured by setupInterceptors
    // For now, basic error message handling:
    const userStore = useUserStore() // Access store here if needed before setupInterceptors runs
    let message = 'An unknown error occurred'

    if (error.response) {
      message = error.response.data?.error || error.response.statusText || 'Server error'
      if (error.response.status === 401) {
        // This part will be enhanced by setupInterceptors to include router redirection
        if (userStore.isLoggedIn) {
          await userStore.logout() // Ensure logout completes
        }
        ElMessage({
          message: 'Session expired or unauthorized. Please login again.',
          type: 'warning',
          duration: 5 * 1000,
        })
      } else {
        ElMessage({
          message: message,
          type: 'error',
          duration: 5 * 1000,
        })
      }
    } else if (error.request) {
      message = 'No response from server. Please check your network connection.'
      ElMessage({
        message: message,
        type: 'error',
        duration: 5 * 1000,
      })
    } else {
      message = error.message
      ElMessage({
        message: message,
        type: 'error',
        duration: 5 * 1000,
      })
    }
    return Promise.reject(error)
  }
)

export function setupInterceptors(router: Router) {
  apiClient.interceptors.response.use(
    (response) => response.data, // For successful responses, return response.data
    async (error) => {
      const userStore = useUserStore()
      let message = 'An unknown error occurred'

      if (error.response) {
        message = error.response.data?.error || error.response.statusText || 'Server error'
        if (error.response.status === 401) {
          if (userStore.isLoggedIn) {
            // Logout action clears local state and attempts to call backend /logout
            await userStore.logout()
          }
          // Redirect to login page, unless already there or the error came from /auth/check on login page
          if (router.currentRoute.value.name !== 'Login') {
            router.push({
              name: 'Login',
              query: { redirect: router.currentRoute.value.fullPath },
            })
          }
          ElMessage({
            message: 'Session expired or unauthorized. Please login again.',
            type: 'warning',
            duration: 3 * 1000,
          })
        } else {
          // For other errors, display the message
          ElMessage({
            message: message,
            type: 'error',
            duration: 5 * 1000,
          })
        }
      } else if (error.request) {
        message = 'No response from server. Please check your network connection.'
        ElMessage({
          message: message,
          type: 'error',
          duration: 5 * 1000,
        })
      } else {
        message = error.message
        ElMessage({
          message: message,
          type: 'error',
          duration: 5 * 1000,
        })
      }
      return Promise.reject(error)
    }
  )
}

export default apiClient
