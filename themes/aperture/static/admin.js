// Admin-page client wiring: editor tabs (Write/Preview/Split), live preview
// via /admin/preview, image upload via /admin/upload, word/reading stats and
// SEO snippet kept in sync with the form fields.

(function () {
  function ready(fn) {
    if (document.readyState !== 'loading') fn();
    else document.addEventListener('DOMContentLoaded', fn);
  }

  ready(function () {
    const textarea = document.querySelector('[data-editor-textarea]');
    if (!textarea) return; // not on the edit page
    const preview = document.querySelector('[data-editor-preview]');
    const tabs = document.querySelector('[data-tabs]');

    // --- tabs ---
    let mode = 'write'; // 'write' | 'preview' | 'split'
    function applyMode(m) {
      mode = m;
      tabs.querySelectorAll('button').forEach((b) => {
        b.classList.toggle('active', b.dataset.tab === m);
      });
      const body = document.querySelector('[data-editor-body]');
      if (m === 'write') {
        textarea.style.display = 'block';
        preview.style.display = 'none';
        body.style.display = 'block';
        body.style.gridTemplateColumns = '';
      } else if (m === 'preview') {
        textarea.style.display = 'none';
        preview.style.display = 'block';
        body.style.display = 'block';
        body.style.gridTemplateColumns = '';
        refreshPreview();
      } else {
        textarea.style.display = 'block';
        preview.style.display = 'block';
        body.style.display = 'grid';
        body.style.gridTemplateColumns = '1fr 1fr';
        body.style.gap = '16px';
        refreshPreview();
      }
    }
    tabs.querySelectorAll('button').forEach((b) => {
      b.addEventListener('click', () => applyMode(b.dataset.tab));
    });

    // --- live preview (debounced fetch to /admin/preview) ---
    let timer;
    async function refreshPreview() {
      if (mode === 'write') return;
      preview.innerHTML = '<em style="color: var(--muted);">Rendering…</em>';
      try {
        const res = await fetch('/admin/preview', {
          method: 'POST',
          headers: { 'Content-Type': 'text/plain' },
          body: textarea.value,
        });
        preview.innerHTML = await res.text();
      } catch (e) {
        preview.innerHTML = `<p style="color: oklch(0.55 0.16 25);">Preview error: ${e.message}</p>`;
      }
    }
    textarea.addEventListener('input', () => {
      updateStats();
      updateSEOFromForm();
      clearTimeout(timer);
      timer = setTimeout(() => { if (mode !== 'write') refreshPreview(); }, 250);
    });

    // --- word / reading stats ---
    const wordsEl = document.querySelector('[data-stat-words]');
    const readEl = document.querySelector('[data-stat-reading]');
    function updateStats() {
      const txt = textarea.value.trim();
      const w = txt ? txt.split(/\s+/).length : 0;
      const min = Math.max(1, Math.round(w / 220));
      if (wordsEl) wordsEl.textContent = w;
      if (readEl) readEl.textContent = `${min} min`;
    }
    updateStats();

    // --- SEO snippet sync ---
    const titleInput = document.querySelector('input[name="title"]');
    const descInput = document.querySelector('input[name="description"]');
    const slugInput = document.querySelector('input[name="slug"]');
    const seoTitle = document.querySelector('[data-seo-title]');
    const seoDesc = document.querySelector('[data-seo-desc]');
    const seoURL = document.querySelector('[data-seo-url]');
    const seoURLTemplate = seoURL ? seoURL.textContent : '';
    function updateSEOFromForm() {
      if (seoTitle && titleInput) seoTitle.textContent = titleInput.value || 'Untitled essay';
      if (seoDesc && descInput) seoDesc.textContent = descInput.value || 'Add a description to control the SEO snippet.';
      if (seoURL && slugInput && seoURLTemplate) {
        seoURL.textContent = seoURLTemplate.replace(/\/posts\/[^/]+\/?$/, `/posts/${slugInput.value || 'slug'}/`);
      }
    }
    [titleInput, descInput, slugInput].forEach((el) => el && el.addEventListener('input', updateSEOFromForm));

    // --- image upload ---
    const uploadBtn = document.querySelector('[data-upload-btn]');
    const fileInput = document.getElementById('image-upload');
    if (uploadBtn && fileInput) {
      uploadBtn.addEventListener('click', () => fileInput.click());
      fileInput.addEventListener('change', async () => {
        const file = fileInput.files?.[0];
        if (!file) return;
        const orig = uploadBtn.textContent;
        uploadBtn.textContent = 'Uploading…';
        try {
          const fd = new FormData();
          fd.append('file', file);
          const res = await fetch('/admin/upload', { method: 'POST', body: fd });
          if (!res.ok) throw new Error(await res.text());
          const data = await res.json();
          const path = data?.data?.filePath || '';
          if (!path) throw new Error('no path in response');
          // Insert markdown image at cursor
          const before = textarea.value.slice(0, textarea.selectionStart);
          const after = textarea.value.slice(textarea.selectionEnd);
          const tag = `![${file.name}](${path})`;
          textarea.value = before + tag + after;
          textarea.focus();
          textarea.selectionStart = textarea.selectionEnd = before.length + tag.length;
          textarea.dispatchEvent(new Event('input'));
          uploadBtn.textContent = 'Uploaded ✓';
        } catch (e) {
          uploadBtn.textContent = 'Upload failed';
          console.error(e);
        }
        setTimeout(() => { uploadBtn.textContent = orig; fileInput.value = ''; }, 1400);
      });
    }

    // --- delete confirm ---
    const deleteBtn = document.querySelector('[data-delete-btn]');
    const deleteForm = document.getElementById('delete-form');
    if (deleteBtn && deleteForm) {
      deleteBtn.addEventListener('click', () => {
        if (confirm('Delete this post? This cannot be undone.')) {
          deleteForm.submit();
        }
      });
    }
  });
})();
