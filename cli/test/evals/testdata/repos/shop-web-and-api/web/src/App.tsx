import { Route, Routes } from 'react-router-dom'
import { SearchPage } from './pages/SearchPage'

export function App() {
  return (
    <Routes>
      <Route path="/search" element={<SearchPage />} />
    </Routes>
  )
}
