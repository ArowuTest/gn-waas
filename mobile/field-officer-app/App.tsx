import React, { useEffect } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { StatusBar } from 'expo-status-bar'
import AppNavigator from './src/navigation/AppNavigator'
import { useAuthStore } from './src/store/authStore'

const queryClient = new QueryClient({
  defaultOptions: { queries: { staleTime: 30_000, retry: 1 } },
})

export default function App() {
  const { loadStoredAuth } = useAuthStore()

  useEffect(() => {
    loadStoredAuth()
  }, [])

  return (
    <QueryClientProvider client={queryClient}>
      <StatusBar style="light" />
      <AppNavigator />
    </QueryClientProvider>
  )
}
