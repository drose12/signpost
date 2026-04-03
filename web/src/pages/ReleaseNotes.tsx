// web/src/pages/ReleaseNotes.tsx
import { useEffect, useState } from 'react';
import { api } from '@/api';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';

interface ChangelogResponse {
  content: string;
}

interface Section {
  title: string;
  body: string;
}

function parseChangelog(raw: string): Section[] {
  // Split by ## headings (version sections)
  const parts = raw.split(/^## /m).filter((s) => s.trim());
  return parts.map((part) => {
    const lines = part.split('\n');
    const title = lines[0].replace(/^\[|\]$/g, '').trim();
    const body = lines.slice(1).join('\n').trim();
    return { title, body };
  });
}

function renderBody(body: string) {
  const lines = body.split('\n');
  const elements: React.ReactNode[] = [];
  let currentList: string[] = [];
  let key = 0;

  function flushList() {
    if (currentList.length > 0) {
      elements.push(
        <ul key={key++} className="list-disc list-inside space-y-1 text-sm text-slate-700 dark:text-slate-300">
          {currentList.map((item, i) => (
            <li key={i}>{item}</li>
          ))}
        </ul>,
      );
      currentList = [];
    }
  }

  for (const line of lines) {
    const trimmed = line.trim();
    if (trimmed.startsWith('### ')) {
      flushList();
      elements.push(
        <h4 key={key++} className="text-sm font-semibold text-slate-800 dark:text-slate-200 mt-3 mb-1">
          {trimmed.replace(/^### /, '')}
        </h4>,
      );
    } else if (trimmed.startsWith('- ')) {
      currentList.push(trimmed.replace(/^- /, ''));
    } else if (trimmed === '') {
      flushList();
    } else {
      flushList();
      elements.push(
        <p key={key++} className="text-sm text-slate-600 dark:text-slate-400">
          {trimmed}
        </p>,
      );
    }
  }
  flushList();
  return elements;
}

export function ReleaseNotes() {
  const [sections, setSections] = useState<Section[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    api
      .get<ChangelogResponse>('/changelog')
      .then((data) => {
        setSections(parseChangelog(data.content));
      })
      .catch((err) => {
        setError(err instanceof Error ? err.message : 'Failed to load changelog');
      })
      .finally(() => setLoading(false));
  }, []);

  if (loading) {
    return (
      <div className="flex items-center justify-center h-32 text-slate-500 dark:text-slate-400">
        Loading...
      </div>
    );
  }

  if (error) {
    return (
      <div className="text-red-500 dark:text-red-400 p-4">Error: {error}</div>
    );
  }

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-semibold text-slate-800 dark:text-slate-100">
        Release Notes
      </h1>

      {sections.length === 0 && (
        <p className="text-slate-500 dark:text-slate-400">No release notes available.</p>
      )}

      {sections.map((section, i) => (
        <Card key={i} className="dark:bg-slate-800">
          <CardHeader className="pb-2">
            <CardTitle className="text-base font-semibold text-slate-800 dark:text-slate-100">
              {section.title}
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-1">
            {renderBody(section.body)}
          </CardContent>
        </Card>
      ))}
    </div>
  );
}
