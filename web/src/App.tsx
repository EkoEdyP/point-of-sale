import { Navigate, Route, Routes } from 'react-router'
import { homeFor, useAuth } from './auth/AuthContext'
import { RequireAuth, RequireRole } from './auth/RequireAuth'
import { Layout } from './components/Layout'
import { CashierPage } from './pages/Cashier'
import { DeliveriesPage } from './pages/Deliveries'
import { HistoryPage } from './pages/History'
import { LoginPage } from './pages/Login'
import { PreordersPage } from './pages/Preorders'
import { ProductsPage } from './pages/Products'
import { ReportsPage } from './pages/Reports'
import { UsersPage } from './pages/Users'

function Home() {
  const { state } = useAuth()
  if (state.status === 'authenticated') return <Navigate to={homeFor(state.user.role)} replace />
  return <Navigate to="/login" replace />
}

export default function App() {
  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />

      <Route element={<RequireAuth />}>
        <Route element={<Layout />}>
          <Route path="/kasir" element={<CashierPage />} />
          <Route path="/preorder" element={<PreordersPage />} />
          <Route path="/pengiriman" element={<DeliveriesPage />} />
          <Route path="/riwayat" element={<HistoryPage />} />

          <Route element={<RequireRole roles={['admin']} />}>
            <Route path="/produk" element={<ProductsPage />} />
            <Route path="/laporan" element={<ReportsPage />} />
            <Route path="/pengguna" element={<UsersPage />} />
          </Route>
        </Route>
      </Route>

      <Route path="/" element={<Home />} />
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  )
}
