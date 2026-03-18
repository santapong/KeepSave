# KeepSave Widget - Integration Guide (v2.0)

The `<keepsave-widget>` is a Web Component that can be embedded in any website to manage secrets stored in KeepSave. It uses Shadow DOM for style isolation and supports multiple authentication methods.

> **v2.0 Changes:** Updated to align with KeepSave SDK v2.0.0. The programmatic API now supports batch secret fetch. For full SDK features (retry, circuit breaker, caching), use the dedicated language SDKs (`@keepsave/sdk`, `keepsave` Python package, or Go module).

## Quick Start

### CDN (UMD)

```html
<script src="https://your-cdn.com/keepsave-widget.umd.js"></script>

<keepsave-widget
  project-id="your-project-id"
  api-url="https://your-keepsave-server.com"
  api-key="ks_your_api_key"
  theme="light"
  mode="read"
></keepsave-widget>
```

### ES Module

```html
<script type="module">
  import 'https://your-cdn.com/keepsave-widget.es.js';
</script>

<keepsave-widget
  project-id="your-project-id"
  api-url="https://your-keepsave-server.com"
  token="your-jwt-token"
></keepsave-widget>
```

### NPM Package

```bash
npm install keepsave-widget
```

```typescript
import { register } from 'keepsave-widget';

register(); // registers <keepsave-widget> custom element
```

## HTML Attributes

| Attribute    | Required | Default               | Description                                      |
|-------------|----------|-----------------------|--------------------------------------------------|
| `project-id` | Yes      | —                     | The KeepSave project ID to display secrets for  |
| `api-url`    | No       | Current origin        | Base URL of the KeepSave API server             |
| `theme`      | No       | `light`               | Color theme: `light` or `dark`                  |
| `mode`       | No       | `read`                | Access mode: `read` (view only) or `readwrite`  |
| `token`      | No       | —                     | JWT token for direct authentication             |
| `api-key`    | No       | —                     | API key for direct authentication               |

## Authentication

The widget supports three authentication methods:

### 1. Direct API Key (simplest)

Pass an API key directly via the `api-key` attribute:

```html
<keepsave-widget
  project-id="my-project"
  api-key="ks_abc123..."
></keepsave-widget>
```

### 2. Direct JWT Token

Pass a JWT token via the `token` attribute:

```html
<keepsave-widget
  project-id="my-project"
  token="eyJhbG..."
></keepsave-widget>
```

### 3. postMessage Handshake (recommended for cross-origin)

If no `token` or `api-key` attribute is provided, the widget initiates a postMessage handshake:

1. The widget sends a `keepsave-auth-request` message to `window.parent`
2. The host page responds with a `keepsave-auth` message containing credentials

```javascript
// Host page listens for auth requests
window.addEventListener('message', (event) => {
  if (event.data.type === 'keepsave-auth-request') {
    // Send credentials back to the widget
    event.source.postMessage({
      type: 'keepsave-auth',
      token: 'your-jwt-token',
      // OR: apiKey: 'ks_your_api_key'
    }, '*');
  }
});
```

**Message Types:**

```typescript
// Widget -> Host (request)
interface AuthRequestMessage {
  type: 'keepsave-auth-request';
  widgetId: string;
}

// Host -> Widget (response)
interface AuthMessage {
  type: 'keepsave-auth';
  token?: string;
  apiKey?: string;
}
```

## Theming

### Built-in Themes

Set the `theme` attribute to `light` or `dark`:

```html
<keepsave-widget theme="dark" ...></keepsave-widget>
```

### Custom CSS Variables

Override CSS custom properties on the host element to customize colors:

```css
keepsave-widget {
  --ks-color-bg: #fafafa;
  --ks-color-surface: #ffffff;
  --ks-color-primary: #6366f1;
  --ks-color-primary-hover: #4f46e5;
  --ks-color-danger: #ef4444;
  --ks-color-text: #111827;
  --ks-color-text-secondary: #6b7280;
  --ks-color-border: #e5e7eb;
}
```

### Available CSS Variables

| Variable                     | Description           |
|-----------------------------|-----------------------|
| `--ks-color-bg`             | Widget background     |
| `--ks-color-surface`        | Card/item background  |
| `--ks-color-surface-hover`  | Hover state           |
| `--ks-color-primary`        | Primary action color  |
| `--ks-color-primary-hover`  | Primary hover color   |
| `--ks-color-danger`         | Delete/error color    |
| `--ks-color-danger-hover`   | Danger hover color    |
| `--ks-color-success`        | Success/connected     |
| `--ks-color-text`           | Primary text          |
| `--ks-color-text-secondary` | Secondary text        |
| `--ks-color-border`         | Border color          |
| `--ks-color-input-bg`       | Input background      |

## Modes

### Read-only (`mode="read"`)

- View secrets across environments (Alpha, UAT, PROD)
- Reveal/hide secret values
- No create, edit, or delete actions

### Read-write (`mode="readwrite"`)

- All read-only capabilities
- Add new secrets
- Edit existing secret values
- Delete secrets

## Programmatic API

The widget also exports a standalone API client:

```typescript
import { KeepSaveAPI } from 'keepsave-widget';

const api = new KeepSaveAPI('https://your-keepsave-server.com');
api.setApiKey('ks_your_api_key');

// List secrets
const secrets = await api.listSecrets('project-id', 'alpha');

// Create a secret
await api.createSecret('project-id', 'DB_HOST', 'localhost', 'alpha');

// Update a secret
await api.updateSecret('project-id', 'secret-id', 'new-value');

// Delete a secret
await api.deleteSecret('project-id', 'secret-id');
```

## Style Isolation

The widget uses Shadow DOM to ensure:

- Widget styles do not affect the host page
- Host page styles do not affect the widget
- CSS variable overrides are the only intentional bridge

## Browser Support

The widget uses standard Web Components APIs supported in all modern browsers:

- Chrome 67+
- Firefox 63+
- Safari 10.1+
- Edge 79+

## Building from Source

```bash
cd frontend

# Build the widget bundle
npm run build:widget

# Output files in dist-embed/:
#   keepsave-widget.es.js   (ES module)
#   keepsave-widget.umd.js  (UMD for script tags)
```
