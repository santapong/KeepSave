import { useState, useRef, useEffect } from 'react';
import type { DashboardApplication } from '../types';

interface Message {
  role: 'user' | 'assistant';
  content: string;
}

interface AppChatbotProps {
  applications: DashboardApplication[];
}

export function AppChatbot({ applications }: AppChatbotProps) {
  const [isOpen, setIsOpen] = useState(false);
  const [messages, setMessages] = useState<Message[]>([]);
  const [input, setInput] = useState('');
  const [loading, setLoading] = useState(false);
  const messagesEndRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!input.trim() || loading) return;

    const userMessage = input.trim();
    setInput('');
    setMessages((prev) => [...prev, { role: 'user', content: userMessage }]);
    setLoading(true);

    // Simple local AI-like response based on application data
    const response = generateResponse(userMessage, applications);
    setTimeout(() => {
      setMessages((prev) => [...prev, { role: 'assistant', content: response }]);
      setLoading(false);
    }, 300);
  };

  return (
    <>
      {/* Toggle Button */}
      <button
        onClick={() => setIsOpen(!isOpen)}
        style={{
          position: 'fixed',
          bottom: 24,
          left: 24,
          zIndex: 200,
          width: 48,
          height: 48,
          borderRadius: '50%',
          background: 'var(--color-primary)',
          color: '#fff',
          border: 'none',
          cursor: 'pointer',
          fontSize: 20,
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          boxShadow: '0 4px 12px rgba(0,0,0,0.15)',
        }}
        title="App Assistant"
      >
        {isOpen ? '\u2715' : '\u{1F4AC}'}
      </button>

      {/* Chat Panel */}
      {isOpen && (
        <div style={panelStyle}>
          <div style={headerStyle}>
            <span style={{ fontWeight: 700, fontSize: 14, color: 'var(--color-text)' }}>App Assistant</span>
            <span style={{ fontSize: 11, color: 'var(--color-text-secondary)' }}>
              Ask about your {applications.length} apps
            </span>
          </div>

          <div style={messagesContainer}>
            {messages.length === 0 && (
              <div style={{ textAlign: 'center', padding: '40px 16px', color: 'var(--color-text-secondary)', fontSize: 13 }}>
                <p style={{ margin: 0 }}>Ask me about your applications!</p>
                <p style={{ margin: '8px 0 0', fontSize: 12 }}>Try: "What apps do I have?" or "Find dev tools"</p>
              </div>
            )}
            {messages.map((msg, i) => (
              <div
                key={i}
                style={{
                  padding: '8px 12px',
                  borderRadius: 10,
                  maxWidth: '85%',
                  alignSelf: msg.role === 'user' ? 'flex-end' : 'flex-start',
                  background: msg.role === 'user' ? 'var(--color-primary)' : 'var(--color-bg)',
                  color: msg.role === 'user' ? '#fff' : 'var(--color-text)',
                  fontSize: 13,
                  lineHeight: 1.5,
                  whiteSpace: 'pre-wrap',
                }}
              >
                {msg.content}
              </div>
            ))}
            {loading && (
              <div style={{ padding: '8px 12px', borderRadius: 10, background: 'var(--color-bg)', color: 'var(--color-text-secondary)', fontSize: 13, alignSelf: 'flex-start' }}>
                Thinking...
              </div>
            )}
            <div ref={messagesEndRef} />
          </div>

          <form onSubmit={handleSubmit} style={inputContainer}>
            <input
              value={input}
              onChange={(e) => setInput(e.target.value)}
              placeholder="Ask about your apps..."
              style={chatInput}
            />
            <button type="submit" disabled={loading || !input.trim()} style={sendBtn}>
              Send
            </button>
          </form>
        </div>
      )}
    </>
  );
}

function generateResponse(query: string, apps: DashboardApplication[]): string {
  const q = query.toLowerCase();

  // List all apps
  if (q.includes('what app') || q.includes('list') || q.includes('all app') || q.includes('show me')) {
    if (apps.length === 0) return 'You don\'t have any applications registered yet. Click "+ Register Service" to add one!';
    const list = apps.map((a) => `\u2022 ${a.icon} ${a.name} - ${a.category} (${a.url})`).join('\n');
    return `You have ${apps.length} application(s):\n\n${list}`;
  }

  // Search by category
  const categories = [...new Set(apps.map((a) => a.category))];
  for (const cat of categories) {
    if (q.includes(cat.toLowerCase())) {
      const matched = apps.filter((a) => a.category === cat);
      if (matched.length === 0) return `No applications found in the "${cat}" category.`;
      const list = matched.map((a) => `\u2022 ${a.icon} ${a.name} - ${a.description || 'No description'}`).join('\n');
      return `Found ${matched.length} app(s) in "${cat}":\n\n${list}`;
    }
  }

  // Search by name/description
  const matched = apps.filter(
    (a) => a.name.toLowerCase().includes(q) || (a.description || '').toLowerCase().includes(q)
  );
  if (matched.length > 0) {
    const list = matched.map((a) => `\u2022 ${a.icon} ${a.name} (${a.category})\n  ${a.description || 'No description'}\n  URL: ${a.url}`).join('\n\n');
    return `Found ${matched.length} matching app(s):\n\n${list}`;
  }

  // Favorites
  if (q.includes('favorite') || q.includes('fav')) {
    const favs = apps.filter((a) => a.is_favorite);
    if (favs.length === 0) return 'You haven\'t favorited any applications yet. Click the star icon on any app card to favorite it.';
    const list = favs.map((a) => `\u2022 ${a.icon} ${a.name} - ${a.category}`).join('\n');
    return `Your favorite apps:\n\n${list}`;
  }

  // Stats
  if (q.includes('stat') || q.includes('how many') || q.includes('count')) {
    const catCounts = categories.map((c) => `${c}: ${apps.filter((a) => a.category === c).length}`).join(', ');
    return `You have ${apps.length} total applications.\nBy category: ${catCounts || 'none'}`;
  }

  // Help
  if (q.includes('help') || q.includes('what can')) {
    return 'I can help you with:\n\n\u2022 "List all apps" - see all your applications\n\u2022 "Find dev tools" - search by category\n\u2022 "Search MedQCNN" - find apps by name\n\u2022 "Show favorites" - see your favorites\n\u2022 "Stats" - get application counts';
  }

  return `I couldn't find apps matching "${query}". Try asking "list all apps" or "help" to see what I can do.`;
}

// --- Styles ---

const panelStyle: React.CSSProperties = {
  position: 'fixed',
  bottom: 80,
  left: 24,
  width: 360,
  maxHeight: 480,
  zIndex: 200,
  background: 'var(--color-surface)',
  border: '1px solid var(--color-border)',
  borderRadius: 16,
  display: 'flex',
  flexDirection: 'column',
  boxShadow: '0 8px 32px rgba(0,0,0,0.15)',
  overflow: 'hidden',
};

const headerStyle: React.CSSProperties = {
  padding: '12px 16px',
  borderBottom: '1px solid var(--color-border)',
  display: 'flex',
  justifyContent: 'space-between',
  alignItems: 'center',
};

const messagesContainer: React.CSSProperties = {
  flex: 1,
  overflowY: 'auto',
  padding: 12,
  display: 'flex',
  flexDirection: 'column',
  gap: 8,
  minHeight: 200,
  maxHeight: 320,
};

const inputContainer: React.CSSProperties = {
  padding: 12,
  borderTop: '1px solid var(--color-border)',
  display: 'flex',
  gap: 8,
};

const chatInput: React.CSSProperties = {
  flex: 1,
  padding: '8px 12px',
  borderRadius: 8,
  border: '1px solid var(--color-border)',
  background: 'var(--color-bg)',
  color: 'var(--color-text)',
  fontSize: 13,
  outline: 'none',
};

const sendBtn: React.CSSProperties = {
  padding: '8px 14px',
  borderRadius: 8,
  background: 'var(--color-primary)',
  color: '#fff',
  border: 'none',
  fontSize: 12,
  fontWeight: 600,
  cursor: 'pointer',
};
