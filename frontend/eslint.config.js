import js from '@eslint/js';
import tseslint from 'typescript-eslint';

/**
 * Flat ESLint config.
 *
 * The primary purpose today is FE-F06: ban `innerHTML` / `outerHTML` /
 * `insertAdjacentHTML` in the embed SDK so the widget can never regress to a
 * string-HTML render path (an XSS sink, since secret keys/values are
 * attacker-influenced). The widget renders exclusively via createElement /
 * textContent / setAttribute.
 */

/** Rule set that forbids HTML-injection sinks. Reused for the embed SDK. */
const noHtmlInjectionRules = {
  // Bans `el.innerHTML = ...` and `el.outerHTML = ...` (read or write).
  'no-restricted-properties': [
    'error',
    {
      property: 'innerHTML',
      message:
        'innerHTML is banned in the embed SDK (FE-F06): build DOM with document.createElement / textContent / setAttribute instead.',
    },
    {
      property: 'outerHTML',
      message:
        'outerHTML is banned in the embed SDK (FE-F06): build DOM with document.createElement / textContent / setAttribute instead.',
    },
  ],
  // Bans `el.insertAdjacentHTML(...)`, which is an equivalent HTML sink.
  'no-restricted-syntax': [
    'error',
    {
      selector: "CallExpression[callee.property.name='insertAdjacentHTML']",
      message:
        'insertAdjacentHTML is banned in the embed SDK (FE-F06): build DOM with document.createElement / textContent / setAttribute instead.',
    },
    {
      // Catch `Element.prototype.innerHTML = ...` style assignments too.
      selector: "AssignmentExpression[left.property.name='innerHTML']",
      message:
        'Assigning innerHTML is banned in the embed SDK (FE-F06): build DOM with document.createElement / textContent / setAttribute instead.',
    },
  ],
};

export default tseslint.config(
  {
    // Only lint TypeScript sources; build artifacts and configs are ignored.
    ignores: ['dist', 'dist-embed', 'node_modules', '**/*.d.ts'],
  },
  {
    // Pre-existing `// eslint-disable-next-line no-console` comments in the
    // embed SDK document intentional operator-facing logging. We don't enable
    // the no-console rule here, so don't flag those directives as unused.
    linterOptions: {
      reportUnusedDisableDirectives: 'off',
    },
  },
  // Embed SDK: full recommended TS rules PLUS the HTML-injection ban.
  {
    files: ['src/embed/**/*.ts'],
    extends: [js.configs.recommended, ...tseslint.configs.recommended],
    languageOptions: {
      globals: {
        window: 'readonly',
        document: 'readonly',
        console: 'readonly',
        fetch: 'readonly',
        customElements: 'readonly',
        HTMLElement: 'readonly',
        MessageEvent: 'readonly',
        URL: 'readonly',
        setTimeout: 'readonly',
      },
    },
    rules: {
      ...noHtmlInjectionRules,
      // Tests use vi/expect globals; keep the SDK strict but pragmatic.
      '@typescript-eslint/no-explicit-any': 'error',
    },
  },
  {
    // Test files in the embed dir may use the no-undef-prone test globals.
    files: ['src/embed/**/*.test.ts'],
    languageOptions: {
      globals: {
        describe: 'readonly',
        it: 'readonly',
        expect: 'readonly',
        vi: 'readonly',
        beforeEach: 'readonly',
        afterEach: 'readonly',
      },
    },
  }
);
