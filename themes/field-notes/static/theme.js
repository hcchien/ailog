// Field Notes bootstrap. Same wiring as Aperture; the theme uses a small
// glyph (☾/☀) instead of icon SVGs for the toggle.
(function () {
  function ready(fn) {
    if (document.readyState !== 'loading') fn();
    else document.addEventListener('DOMContentLoaded', fn);
  }

  ready(function () {
    const SH = window.AilogShared;

    document.querySelectorAll('.theme-toggle').forEach((btn) => {
      const moon = btn.querySelector('[data-icon="moon"]');
      const sun = btn.querySelector('[data-icon="sun"]');
      const paint = () => {
        const dark = document.documentElement.classList.contains('dark');
        if (moon) moon.style.display = dark ? 'none' : 'inline';
        if (sun) sun.style.display = dark ? 'inline' : 'none';
      };
      paint();
      btn.addEventListener('click', (e) => {
        e.preventDefault();
        const dark = document.documentElement.classList.contains('dark');
        SH.setTheme(dark ? 'light' : 'dark');
        paint();
      });
    });

    document.querySelectorAll('[data-copy-link]').forEach((a) => {
      a.addEventListener('click', async (e) => {
        e.preventDefault();
        try { await navigator.clipboard.writeText(location.href); } catch {}
        const orig = a.textContent;
        a.textContent = 'Copied →';
        setTimeout(() => { a.textContent = orig; }, 1200);
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
