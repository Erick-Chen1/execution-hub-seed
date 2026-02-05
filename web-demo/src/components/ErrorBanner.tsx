interface ErrorBannerProps {
  message?: string;
}

export default function ErrorBanner({ message }: ErrorBannerProps) {
  if (!message) {
    return null;
  }
  return (
    <div className="panel compact" style={{ borderColor: 'var(--danger)', background: 'rgba(185, 28, 28, 0.08)' }}>
      <div style={{ fontWeight: 600, color: 'var(--danger)' }}>Error</div>
      <div className="mono" style={{ marginTop: 6 }}>{message}</div>
    </div>
  );
}
