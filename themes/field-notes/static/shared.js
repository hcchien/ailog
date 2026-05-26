// Cross-theme utilities. Mirrors the design package: a tiny syntax
// highlighter, code-copy buttons, TOC builder, reading-progress bar,
// dark-mode persistence. Exposed as window.AilogShared.

(function () {
  const PATTERNS = {
    js: [
      [/(\/\/[^\n]*|\/\*[\s\S]*?\*\/)/g, 'cm'],
      [/("(?:[^"\\]|\\.)*"|'(?:[^'\\]|\\.)*'|`(?:[^`\\]|\\.)*`)/g, 'st'],
      [/\b(const|let|var|function|return|if|else|for|while|await|async|new|class|extends|import|export|from|default|of|in|null|undefined|true|false|this|=>)\b/g, 'kw'],
      [/\b(\d+(?:\.\d+)?)\b/g, 'nu'],
      [/\b([A-Za-z_$][\w$]*)\s*(?=\()/g, 'fn'],
    ],
    go: [
      [/(\/\/[^\n]*|\/\*[\s\S]*?\*\/)/g, 'cm'],
      [/("(?:[^"\\]|\\.)*"|`[\s\S]*?`)/g, 'st'],
      [/\b(func|return|if|else|for|range|var|const|package|import|type|struct|interface|chan|go|defer|map|nil|true|false)\b/g, 'kw'],
      [/\b(\d+(?:\.\d+)?)\b/g, 'nu'],
    ],
    bash: [
      [/(#[^\n]*)/g, 'cm'],
      [/("(?:[^"\\]|\\.)*"|'(?:[^'\\]|\\.)*')/g, 'st'],
      [/(^|\s)(-{1,2}[\w-]+)/g, 'kw'],
    ],
    json: [
      [/("(?:[^"\\]|\\.)*")(\s*:)/g, 'fn'],
      [/("(?:[^"\\]|\\.)*")/g, 'st'],
      [/\b(true|false|null)\b/g, 'kw'],
      [/\b(-?\d+(?:\.\d+)?)\b/g, 'nu'],
    ],
  };
  PATTERNS.ts = PATTERNS.js;
  PATTERNS.jsx = PATTERNS.js;
  PATTERNS.tsx = PATTERNS.js;
  PATTERNS.yaml = [
    [/(#[^\n]*)/g, 'cm'],
    [/^(\s*)([\w.-]+)(:)/gm, (m, sp, k, c) => `${sp}<span class="hl-fn">${k}</span>${c}`],
    [/("(?:[^"\\]|\\.)*"|'(?:[^'\\]|\\.)*')/g, 'st'],
    [/\b(true|false|null|~)\b/g, 'kw'],
  ];

  function escapeHtml(s) {
    return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
  }

  function highlight(code, lang) {
    let src = escapeHtml(code);
    const patterns = PATTERNS[lang];
    if (!patterns) return src;
    const slots = [];
    for (const [re, cls] of patterns) {
      src = src.replace(re, (...args) => {
        const out = typeof cls === 'function'
          ? cls(...args)
          : `<span class="hl-${cls}">${args[0]}</span>`;
        const tok = `\x00${slots.length}\x00`;
        slots.push(out);
        return tok;
      });
    }
    return src.replace(/\x00(\d+)\x00/g, (_, i) => slots[+i]);
  }

  function enhanceCode(root) {
    root.querySelectorAll('pre > code').forEach((code) => {
      if (code.dataset.hl) return;
      code.dataset.hl = '1';
      const lang = (code.className.match(/language-(\w+)/) || [])[1] || '';
      const raw = code.textContent;
      // The server-side renderer may already have inserted highlight markup
      // (goldmark's chroma). We only re-highlight when there's no existing
      // span structure inside the code element.
      const alreadyHl = code.querySelector('span');
      if (!alreadyHl) {
        code.innerHTML = highlight(raw, lang);
      }
      const pre = code.parentElement;
      pre.classList.add('has-copy');
      if (lang) pre.dataset.lang = lang;
      const btn = document.createElement('button');
      btn.type = 'button';
      btn.className = 'code-copy';
      btn.textContent = 'Copy';
      btn.addEventListener('click', async () => {
        try {
          await navigator.clipboard.writeText(raw);
        } catch {
          const t = document.createElement('textarea');
          t.value = raw; document.body.appendChild(t); t.select();
          try { document.execCommand('copy'); } catch {}
          t.remove();
        }
        btn.textContent = 'Copied';
        clearTimeout(btn._t);
        btn._t = setTimeout(() => (btn.textContent = 'Copy'), 1400);
      });
      pre.appendChild(btn);
    });
  }

  function bindProgress(bar, scrollEl) {
    const target = scrollEl || document.scrollingElement || document.documentElement;
    const handler = () => {
      const max = target.scrollHeight - target.clientHeight;
      const pct = max > 0 ? Math.min(100, Math.max(0, (target.scrollTop / max) * 100)) : 0;
      bar.style.width = pct + '%';
    };
    handler();
    (scrollEl || window).addEventListener('scroll', handler, { passive: true });
    window.addEventListener('resize', handler);
    return handler;
  }

  function buildTOC(articleEl) {
    const headings = articleEl.querySelectorAll('h2[id], h3[id]');
    return Array.from(headings).map((h) => ({
      id: h.id,
      text: h.textContent,
      level: h.tagName === 'H2' ? 2 : 3,
      el: h,
    }));
  }

  function trackActiveHeading(headings, onChange, scrollEl) {
    const sc = scrollEl || window;
    const target = scrollEl || (document.scrollingElement || document.documentElement);
    const handler = () => {
      const top = (scrollEl ? scrollEl.scrollTop : target.scrollTop) + 120;
      let active = headings[0]?.id;
      for (const h of headings) {
        if (h.el.offsetTop <= top) active = h.id;
      }
      onChange(active);
    };
    handler();
    sc.addEventListener('scroll', handler, { passive: true });
    return handler;
  }

  const THEME_KEY = 'ailog-theme';
  function getTheme() {
    try { return localStorage.getItem(THEME_KEY) || 'auto'; } catch { return 'auto'; }
  }
  function setTheme(t) {
    try { localStorage.setItem(THEME_KEY, t); } catch {}
    applyTheme();
  }
  function applyTheme() {
    const t = getTheme();
    const root = document.documentElement;
    root.dataset.theme = t;
    if (t === 'auto') {
      root.classList.toggle('dark', matchMedia('(prefers-color-scheme: dark)').matches);
    } else {
      root.classList.toggle('dark', t === 'dark');
    }
    root.classList.toggle('light', !root.classList.contains('dark'));
  }
  if (typeof window !== 'undefined') {
    applyTheme();
    matchMedia('(prefers-color-scheme: dark)').addEventListener('change', applyTheme);
  }

  window.AilogShared = {
    enhanceCode, bindProgress, buildTOC, trackActiveHeading,
    getTheme, setTheme, applyTheme,
    highlight, escapeHtml,
  };
})();
