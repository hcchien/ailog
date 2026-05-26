// Signal bootstrap — TOC, code copy, theme toggle, plus the ⌘K command
// palette. Reads the post index from a <script id="search-data"> tag the
// server inlines as JSON.
(function () {
  function ready(fn) {
    if (document.readyState !== 'loading') fn();
    else document.addEventListener('DOMContentLoaded', fn);
  }

  function searchData() {
    try {
      const el = document.getElementById('search-data');
      return el ? JSON.parse(el.textContent) : [];
    } catch { return []; }
  }

  function openPalette(data) {
    const wrap = document.createElement('div');
    wrap.className = 'search-modal';
    wrap.innerHTML = `
      <div class="panel" role="dialog" aria-modal="true">
        <div class="row">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"><circle cx="11" cy="11" r="7"/><path d="M21 21l-4.3-4.3"/></svg>
          <input placeholder="Search posts, tags, ideas…" autofocus>
          <span class="kbd">ESC</span>
        </div>
        <div class="results"></div>
      </div>`;
    const input = wrap.querySelector('input');
    const results = wrap.querySelector('.results');
    document.body.appendChild(wrap);

    // collect tags from data
    const tagCounts = {};
    data.forEach(p => p.tags.forEach(t => tagCounts[t] = (tagCounts[t] || 0) + 1));
    const tags = Object.entries(tagCounts)
      .map(([name, count]) => ({ name, count }))
      .sort((a, b) => b.count - a.count);

    function relTime(iso) {
      const ms = Date.now() - new Date(iso).getTime();
      const d = Math.floor(ms / 86400000);
      if (d < 7) return `${d}d ago`;
      if (d < 30) return `${Math.floor(d / 7)}w ago`;
      if (d < 365) return `${Math.floor(d / 30)}mo ago`;
      return `${Math.floor(d / 365)}y ago`;
    }

    function tagURL(tag) {
      // best effort: themes/signal uses the same path scheme as the public site
      const base = location.pathname.replace(/\/(posts|tags)\/.*$/, '');
      const root = base.endsWith('/') ? base.slice(0, -1) : base;
      return `${root}/tags/${encodeURIComponent(tag.toLowerCase().replace(/ /g, '-'))}/`;
    }

    function render() {
      const q = input.value.toLowerCase().trim();
      const postMatches = q
        ? data.filter(p => p.title.toLowerCase().includes(q) || (p.description || '').toLowerCase().includes(q))
        : data.slice(0, 5);
      const tagMatches = q
        ? tags.filter(t => t.name.toLowerCase().includes(q))
        : tags.slice(0, 4);

      let html = '';
      if (postMatches.length) {
        html += '<h6>Posts</h6>';
        for (const p of postMatches) {
          html += `<a href="${p.url}"><span class="icon">›</span><span class="title">${p.title}</span><span class="extra">${relTime(p.date)}</span></a>`;
        }
      }
      if (tagMatches.length) {
        html += '<h6 style="margin-top: 12px;">Topics</h6>';
        for (const t of tagMatches) {
          html += `<a href="${tagURL(t.name)}"><span class="icon">#</span><span class="title">${t.name}</span><span class="extra">${t.count} ${t.count === 1 ? 'post' : 'posts'}</span></a>`;
        }
      }
      if (!postMatches.length && !tagMatches.length) {
        html = `<div class="empty">No matches for "${q}".</div>`;
      }
      results.innerHTML = html;
    }

    input.addEventListener('input', render);
    render();

    function close() {
      wrap.remove();
      document.removeEventListener('keydown', onKey);
    }
    function onKey(e) {
      if (e.key === 'Escape') { e.preventDefault(); close(); }
    }
    document.addEventListener('keydown', onKey);
    wrap.addEventListener('click', (e) => { if (e.target === wrap) close(); });
    setTimeout(() => input.focus(), 30);
  }

  ready(function () {
    const SH = window.AilogShared;
    const data = searchData();

    // Theme toggle.
    document.querySelectorAll('.theme-toggle').forEach((btn) => {
      const sun = btn.querySelector('[data-icon="sun"]');
      const moon = btn.querySelector('[data-icon="moon"]');
      function paint() {
        const dark = document.documentElement.classList.contains('dark');
        if (sun) sun.style.display = dark ? 'block' : 'none';
        if (moon) moon.style.display = dark ? 'none' : 'block';
      }
      paint();
      btn.addEventListener('click', () => {
        const dark = document.documentElement.classList.contains('dark');
        SH.setTheme(dark ? 'light' : 'dark');
        paint();
      });
    });

    // Copy link.
    document.querySelectorAll('[data-copy-link]').forEach((btn) => {
      btn.addEventListener('click', async () => {
        try { await navigator.clipboard.writeText(location.href); } catch {}
        btn.style.color = 'var(--accent)';
        setTimeout(() => { btn.style.color = ''; }, 1200);
      });
    });

    // Search trigger button + ⌘K shortcut.
    document.querySelectorAll('[data-search-trigger]').forEach((btn) => {
      btn.addEventListener('click', () => openPalette(data));
    });
    document.addEventListener('keydown', (e) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') {
        e.preventDefault();
        openPalette(data);
      }
    });

    const body = document.querySelector('[data-post-body]');
    if (body) {
      SH.enhanceCode(body);
      const headings = SH.buildTOC(body);
      const tocEl = document.querySelector('[data-toc] ol');
      if (tocEl && headings.length) {
        tocEl.innerHTML = headings.map(h =>
          `<li class="${h.level === 3 ? 'l3' : ''}" data-toc-id="${h.id}"><a href="#${h.id}">${h.text}</a></li>`
        ).join('');
        tocEl.querySelectorAll('a').forEach((a) => {
          a.addEventListener('click', (e) => {
            e.preventDefault();
            document.getElementById(a.getAttribute('href').slice(1))?.scrollIntoView({ behavior: 'smooth' });
          });
        });
        SH.trackActiveHeading(headings, (id) => {
          tocEl.querySelectorAll('li').forEach((li) => {
            li.classList.toggle('active', li.dataset.tocId === id);
          });
        });
      } else if (tocEl) {
        const aside = tocEl.closest('[data-toc]');
        if (aside) aside.style.visibility = 'hidden';
      }

      const bar = document.querySelector('.progress > i');
      if (bar) SH.bindProgress(bar);
    }
  });
})();
