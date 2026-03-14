import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { useAuth } from './hooks/useAuth';
import { Layout } from './components/Layout';
import { LoginPage } from './pages/LoginPage';
import { RegisterPage } from './pages/RegisterPage';
import { ProjectsPage } from './pages/ProjectsPage';
import { ProjectDetailPage } from './pages/ProjectDetailPage';
import { APIKeysPage } from './pages/APIKeysPage';
import { OrganizationsPage } from './pages/OrganizationsPage';
import { TemplatesPage } from './pages/TemplatesPage';
import { AdminDashboardPage } from './pages/AdminDashboardPage';

export default function App() {
  const auth = useAuth();

  if (!auth.authenticated) {
    return (
      <BrowserRouter>
        <Routes>
          <Route path="/register" element={<RegisterPage onLogin={auth.login} />} />
          <Route path="*" element={<LoginPage onLogin={auth.login} />} />
        </Routes>
      </BrowserRouter>
    );
  }

  return (
    <BrowserRouter>
      <Layout user={auth.user} onLogout={auth.logout}>
        <Routes>
          <Route path="/" element={<ProjectsPage />} />
          <Route path="/projects/:id/*" element={<ProjectDetailPage />} />
          <Route path="/api-keys" element={<APIKeysPage />} />
          <Route path="/organizations" element={<OrganizationsPage />} />
          <Route path="/templates" element={<TemplatesPage />} />
          <Route path="/admin" element={<AdminDashboardPage />} />
          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </Layout>
    </BrowserRouter>
  );
}
