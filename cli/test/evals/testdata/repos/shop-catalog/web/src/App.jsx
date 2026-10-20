import { BrowserRouter, Outlet, Route, Routes } from 'react-router';
import { TracewayProvider } from '@tracewayapp/react';
import { useCatalog } from './hooks/useCatalog';
import { CategoryPage } from './pages/CategoryPage';
import { ProductPage } from './pages/ProductPage';

function Layout() {
  const catalog = useCatalog();
  return (
    <main>
      <Outlet context={{ catalog }} />
    </main>
  );
}

export function App() {
  return (
    <TracewayProvider
      connectionString={import.meta.env.VITE_TRACEWAY_CONNECTION}
      options={{ version: import.meta.env.VITE_APP_VERSION, recordAllSessions: true }}
    >
      <BrowserRouter>
        <Routes>
          <Route element={<Layout />}>
            <Route index element={<CategoryPage />} />
            <Route path="c/:slug" element={<CategoryPage />} />
            <Route path="p/:sku" element={<ProductPage />} />
          </Route>
        </Routes>
      </BrowserRouter>
    </TracewayProvider>
  );
}
