import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { useAuth } from './hooks/useAuth';
import { useTheme } from './hooks/useTheme';
import { Layout } from './components/Layout';
import { LoginPage } from './pages/LoginPage';
import { RegisterPage } from './pages/RegisterPage';
import { ProjectsPage } from './pages/ProjectsPage';
import { ProjectDetailPage } from './pages/ProjectDetailPage';
import { OrganizationsPage } from './pages/OrganizationsPage';
import { TemplatesPage } from './pages/TemplatesPage';
import { AdminDashboardPage } from './pages/AdminDashboardPage';
import { HelpPage } from './pages/HelpPage';
import { MCPHubPage } from './pages/MCPHubPage';
import { OAuthClientsPage } from './pages/OAuthClientsPage';
import { ApplicationDashboardPage } from './pages/ApplicationDashboardPage';
import { ApplicationSettingsPage } from './pages/ApplicationSettingsPage';

export default function App() {
  useTheme();
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
          <Route path="/organizations" element={<OrganizationsPage />} />
          <Route path="/templates" element={<TemplatesPage />} />
          <Route path="/mcp-hub" element={<MCPHubPage />} />
          <Route path="/oauth-clients" element={<OAuthClientsPage />} />
          <Route path="/applications" element={<ApplicationDashboardPage />} />
          <Route path="/applications/settings" element={<ApplicationSettingsPage />} />
          <Route path="/admin" element={<AdminDashboardPage />} />
          <Route path="/help" element={<HelpPage />} />
          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </Layout>
    </BrowserRouter>
  );
}
