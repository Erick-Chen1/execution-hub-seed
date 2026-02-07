import { Navigate, Route, Routes } from 'react-router-dom';
import { AuthProvider, useAuth } from './components/AuthProvider';
import Layout from './components/Layout';
import LoginPage from './pages/Login';
import CollabPage from './pages/Collab';
import WorkflowsPage from './pages/Workflows';
import UsersPage from './pages/Users';
import ExecutorsPage from './pages/Executors';
import NotificationsPage from './pages/Notifications';
import AuditPage from './pages/Audit';
import TrustPage from './pages/Trust';

function ProtectedApp() {
  const { user, loading } = useAuth();
  if (loading) {
    return <div className="main">Loading...</div>;
  }
  if (!user) {
    return <Navigate to="/login" replace />;
  }
  return (
    <Layout>
      <Routes>
        <Route path="/" element={<Navigate to="/collab" replace />} />
        <Route path="/collab" element={<CollabPage />} />
        <Route path="/workflows" element={<WorkflowsPage />} />
        <Route path="/executors" element={<ExecutorsPage />} />
        <Route path="/notifications" element={<NotificationsPage />} />
        <Route path="/trust" element={<TrustPage />} />
        <Route path="/audit" element={<AuditPage />} />
        <Route path="/users" element={<UsersPage />} />
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </Layout>
  );
}

export default function App() {
  return (
    <AuthProvider>
      <Routes>
        <Route path="/login" element={<LoginPage />} />
        <Route path="/*" element={<ProtectedApp />} />
      </Routes>
    </AuthProvider>
  );
}
