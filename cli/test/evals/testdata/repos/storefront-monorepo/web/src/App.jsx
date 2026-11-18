import { BrowserRouter, Route, Routes } from 'react-router';
import { OrdersPage } from './pages/OrdersPage.jsx';

export function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/account/orders" element={<OrdersPage />} />
      </Routes>
    </BrowserRouter>
  );
}
