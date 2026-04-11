import { useState, useRef, useEffect } from 'react';
import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { MessageCircle, X, Send } from 'lucide-react';
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
        className="fixed bottom-6 left-6 z-[200] w-12 h-12 rounded-full bg-primary text-primary-foreground border-none cursor-pointer text-xl flex items-center justify-center shadow-lg hover:bg-primary/90 transition-colors"
        title="App Assistant"
      >
        {isOpen ? <X className="h-5 w-5" /> : <MessageCircle className="h-5 w-5" />}
      </button>

      {/* Chat Panel */}
      {isOpen && (
        <div className="fixed bottom-20 left-6 w-[360px] max-h-[480px] z-[200] bg-card border border-border rounded-2xl flex flex-col shadow-lg overflow-hidden">
          <div className="px-4 py-3 border-b border-border flex justify-between items-center">
            <span className="font-bold text-sm text-foreground">App Assistant</span>
            <span className="text-xs text-muted-foreground">
              Ask about your {applications.length} apps
            </span>
          </div>

          <div className="flex-1 overflow-y-auto p-3 flex flex-col gap-2 min-h-[200px] max-h-[320px]">
            {messages.length === 0 && (
              <div className="text-center py-10 px-4 text-muted-foreground text-sm">
                <p>Ask me about your applications!</p>
                <p className="mt-2 text-xs">Try: &quot;What apps do I have?&quot; or &quot;Find dev tools&quot;</p>
              </div>
            )}
            {messages.map((msg, i) => (
              <div
                key={i}
                className={cn(
                  'px-3 py-2 rounded-xl max-w-[85%] text-sm leading-relaxed whitespace-pre-wrap',
                  msg.role === 'user'
                    ? 'self-end bg-primary text-primary-foreground'
                    : 'self-start bg-muted text-foreground'
                )}
              >
                {msg.content}
              </div>
            ))}
            {loading && (
              <div className="self-start px-3 py-2 rounded-xl bg-muted text-muted-foreground text-sm">
                Thinking...
              </div>
            )}
            <div ref={messagesEndRef} />
          </div>

          <form onSubmit={handleSubmit} className="p-3 border-t border-border flex gap-2">
            <Input
              value={input}
              onChange={(e) => setInput(e.target.value)}
              placeholder="Ask about your apps..."
              className="flex-1 text-sm"
            />
            <Button type="submit" size="sm" disabled={loading || !input.trim()}>
              <Send className="h-4 w-4" />
            </Button>
          </form>
        </div>
      )}
    </>
  );
}

function generateResponse(query: string, apps: DashboardApplication[]): string {
  const q = query.toLowerCase();

  if (q.includes('what app') || q.includes('list') || q.includes('all app') || q.includes('show me')) {
    if (apps.length === 0) return 'You don\'t have any applications registered yet. Click "+ Register Service" to add one!';
    const list = apps.map((a) => `\u2022 ${a.icon} ${a.name} - ${a.category} (${a.url})`).join('\n');
    return `You have ${apps.length} application(s):\n\n${list}`;
  }

  const categories = [...new Set(apps.map((a) => a.category))];
  for (const cat of categories) {
    if (q.includes(cat.toLowerCase())) {
      const matched = apps.filter((a) => a.category === cat);
      if (matched.length === 0) return `No applications found in the "${cat}" category.`;
      const list = matched.map((a) => `\u2022 ${a.icon} ${a.name} - ${a.description || 'No description'}`).join('\n');
      return `Found ${matched.length} app(s) in "${cat}":\n\n${list}`;
    }
  }

  const matched = apps.filter(
    (a) => a.name.toLowerCase().includes(q) || (a.description || '').toLowerCase().includes(q)
  );
  if (matched.length > 0) {
    const list = matched.map((a) => `\u2022 ${a.icon} ${a.name} (${a.category})\n  ${a.description || 'No description'}\n  URL: ${a.url}`).join('\n\n');
    return `Found ${matched.length} matching app(s):\n\n${list}`;
  }

  if (q.includes('favorite') || q.includes('fav')) {
    const favs = apps.filter((a) => a.is_favorite);
    if (favs.length === 0) return 'You haven\'t favorited any applications yet. Click the star icon on any app card to favorite it.';
    const list = favs.map((a) => `\u2022 ${a.icon} ${a.name} - ${a.category}`).join('\n');
    return `Your favorite apps:\n\n${list}`;
  }

  if (q.includes('stat') || q.includes('how many') || q.includes('count')) {
    const catCounts = categories.map((c) => `${c}: ${apps.filter((a) => a.category === c).length}`).join(', ');
    return `You have ${apps.length} total applications.\nBy category: ${catCounts || 'none'}`;
  }

  if (q.includes('help') || q.includes('what can')) {
    return 'I can help you with:\n\n\u2022 "List all apps" - see all your applications\n\u2022 "Find dev tools" - search by category\n\u2022 "Search MedQCNN" - find apps by name\n\u2022 "Show favorites" - see your favorites\n\u2022 "Stats" - get application counts';
  }

  return `I couldn't find apps matching "${query}". Try asking "list all apps" or "help" to see what I can do.`;
}
