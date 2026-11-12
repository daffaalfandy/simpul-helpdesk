import '@fontsource/plus-jakarta-sans/400.css'
import '@fontsource/plus-jakarta-sans/500.css'
import '@fontsource/plus-jakarta-sans/600.css'
import '@fontsource/plus-jakarta-sans/700.css'
import '@fontsource/plus-jakarta-sans/800.css'
import './styles/app.css'
import './i18n'

import { QueryClientProvider } from '@tanstack/react-query'
import { RouterProvider } from '@tanstack/react-router'
import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'

import { createAppRouter, createQueryClient } from './router'

const ROOT_ELEMENT_ID = 'root'

const queryClient = createQueryClient()
const router = createAppRouter(queryClient)
const root = document.getElementById(ROOT_ELEMENT_ID)
if (!root) throw new Error(`#${ROOT_ELEMENT_ID} element missing from index.html`)

createRoot(root).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  </StrictMode>,
)
