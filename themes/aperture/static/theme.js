// Aperture bootstrap: wire up TOC, reading progress, theme toggle.
(function () {
  function ready(fn) {
    if (document.readyState !== 'loading') fn();
    else document.addEventListener('DOMContentLoaded', fn);
  }

  ready(function () {
    const SH = window.AilogShared;

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

    // Copy link button on post.
    document.querySelectorAll('[data-copy-link]').forEach((btn) => {
      btn.addEventListener('click', async () => {
        try { await navigator.clipboard.writeText(location.href); } catch {}
        const t = btn.textContent;
        btn.style.color = 'var(--accent-ink)';
        setTimeout(() => { btn.style.color = ''; }, 1200);
      });
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
            const id = a.getAttribute('href').slice(1);
            document.getElementById(id)?.scrollIntoView({ behavior: 'smooth', block: 'start' });
          });
        });
        SH.trackActiveHeading(headings, (id) => {
          tocEl.querySelectorAll('li').forEach((li) => {
            li.classList.toggle('active', li.dataset.tocId === id);
          });
        });
      } else if (tocEl) {
        // No headings — hide the TOC heading too.
        const aside = tocEl.closest('[data-toc]');
        if (aside) aside.style.visibility = 'hidden';
      }

      const bar = document.querySelector('.progress > i');
      if (bar) SH.bindProgress(bar);
    }
  });
})();
