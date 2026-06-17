import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import '@/styles/globals.css'
import App from './App.tsx'
import { SettingsProvider } from '@/providers/SettingsProvider'
import { AuthProvider } from '@/providers/AuthProvider'
import { ErrorBoundary } from '@/components/ui/ErrorBoundary'

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <ErrorBoundary>
      <AuthProvider>
        <SettingsProvider>
          <App />
        </SettingsProvider>
      </AuthProvider>
    </ErrorBoundary>
  </StrictMode>,
)
