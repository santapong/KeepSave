export interface PrometheusMetric {
  name: string;
  labels: Record<string, string>;
  value: number;
  type?: string;
}

/**
 * Parse Prometheus text exposition format into structured metric data.
 *
 * Handles lines like:
 *   # TYPE http_requests_total counter
 *   # HELP http_requests_total Total HTTP requests
 *   http_requests_total{method="GET",status="200"} 1234
 *   go_goroutines 42
 */
export function parsePrometheusText(text: string): PrometheusMetric[] {
  const metrics: PrometheusMetric[] = [];
  const typeMap = new Map<string, string>();

  const lines = text.split('\n');

  for (const rawLine of lines) {
    const line = rawLine.trim();

    // Skip empty lines
    if (line === '') continue;

    // Extract type info from TYPE comments
    if (line.startsWith('# TYPE ')) {
      const rest = line.slice(7);
      const spaceIdx = rest.indexOf(' ');
      if (spaceIdx !== -1) {
        const metricName = rest.slice(0, spaceIdx);
        const metricType = rest.slice(spaceIdx + 1).trim();
        typeMap.set(metricName, metricType);
      }
      continue;
    }

    // Skip all other comment lines
    if (line.startsWith('#')) continue;

    // Parse metric line
    const metric = parseMetricLine(line, typeMap);
    if (metric) {
      metrics.push(metric);
    }
  }

  return metrics;
}

function parseMetricLine(
  line: string,
  typeMap: Map<string, string>,
): PrometheusMetric | null {
  // Two formats:
  //   metric_name{label="value",...} 123.45
  //   metric_name 123.45

  const braceStart = line.indexOf('{');

  let name: string;
  let labels: Record<string, string> = {};
  let valueStr: string;

  if (braceStart !== -1) {
    // Has labels
    name = line.slice(0, braceStart);
    const braceEnd = line.indexOf('}', braceStart);
    if (braceEnd === -1) return null;

    const labelsStr = line.slice(braceStart + 1, braceEnd);
    labels = parseLabels(labelsStr);

    valueStr = line.slice(braceEnd + 1).trim();
  } else {
    // No labels - split on first space
    const spaceIdx = line.indexOf(' ');
    if (spaceIdx === -1) return null;

    name = line.slice(0, spaceIdx);
    valueStr = line.slice(spaceIdx + 1).trim();
  }

  // Handle optional timestamp at end (space-separated after value)
  const valueParts = valueStr.split(/\s+/);
  const value = Number(valueParts[0]);

  if (Number.isNaN(value)) return null;

  // Look up type based on the base metric name (strip suffixes like _total, _bucket, etc.)
  const type = typeMap.get(name) ?? findTypeForMetric(name, typeMap);

  return { name, labels, value, ...(type ? { type } : {}) };
}

function parseLabels(labelsStr: string): Record<string, string> {
  const labels: Record<string, string> = {};
  if (labelsStr === '') return labels;

  // Match key="value" pairs, handling escaped quotes inside values
  const regex = /(\w+)="((?:[^"\\]|\\.)*)"/g;
  let match: RegExpExecArray | null;

  while ((match = regex.exec(labelsStr)) !== null) {
    const key = match[1];
    // Unescape standard Prometheus escape sequences
    const val = match[2]
      .replace(/\\n/g, '\n')
      .replace(/\\\\/g, '\\')
      .replace(/\\"/g, '"');
    labels[key] = val;
  }

  return labels;
}

function findTypeForMetric(
  name: string,
  typeMap: Map<string, string>,
): string | undefined {
  // Prometheus conventions: _total, _bucket, _sum, _count suffixes
  const suffixes = ['_total', '_bucket', '_sum', '_count', '_info', '_created'];
  for (const suffix of suffixes) {
    if (name.endsWith(suffix)) {
      const baseName = name.slice(0, -suffix.length);
      const type = typeMap.get(baseName);
      if (type) return type;
    }
  }
  return undefined;
}
