(function () {
  'use strict';

  var TOOLBAR = [
    ['heading', 'bold', 'italic'],
    ['ul', 'ol'],
    ['quote', 'hr'],
  ];

  // Mount a Toast UI WYSIWYG editor onto a textarea[data-md-editor].
  // The textarea is hidden; the editor syncs back to it on every change
  // so form submission and HTMX auto-save both work without modification.
  function mountEditor(ta) {
    if (ta._mdEditorMounted) return;
    ta._mdEditorMounted = true;

    // Wrapper div inserted before the textarea.
    var wrap = document.createElement('div');
    wrap.className = 'md-editor-wrap';
    ta.parentNode.insertBefore(wrap, ta);
    ta.style.display = 'none';

    var editor = new toastui.Editor({
      el: wrap,
      initialEditType: 'wysiwyg',
      initialValue: ta.value || '',
      height: 'auto',
      minHeight: '120px',
      toolbarItems: TOOLBAR,
      hideModeSwitch: true,
      events: {
        change: function () {
          var md = editor.getMarkdown();
          // Avoid spurious saves when the editor normalises the initial value.
          if (ta.value === md) return;
          ta.value = md;
          ta.dispatchEvent(new Event('change', { bubbles: true }));
        },
      },
    });

    // Keep a reference so we can destroy on re-init.
    wrap._tuiEditor = editor;
    ta._mdEditorWrap = wrap;
  }

  // Scan root for any textarea[data-md-editor] that isn't mounted yet.
  // Skips textareas inside closed <details> — they'll be lazily mounted
  // when the details opens.
  function initInRoot(root) {
    var tas = (root || document).querySelectorAll('textarea[data-md-editor]');
    tas.forEach(function (ta) {
      // Don't init inside a closed <details> — editor needs to be visible.
      var details = ta.closest('details');
      if (details && !details.open) return;
      mountEditor(ta);
    });
  }

  // Lazy-mount when a <details> containing a data-md-editor textarea opens.
  function bindDetailsToggle(details) {
    if (details._mdToggleBound) return;
    details._mdToggleBound = true;
    details.addEventListener('toggle', function () {
      if (details.open) {
        var tas = details.querySelectorAll('textarea[data-md-editor]');
        tas.forEach(mountEditor);
      }
    });
  }

  function bindAllDetailsToggles(root) {
    var all = (root || document).querySelectorAll('details');
    all.forEach(bindDetailsToggle);
  }

  // Public entry point called after HTMX swaps and on DOMContentLoaded.
  window.initMarkdownEditors = function (root) {
    initInRoot(root);
    bindAllDetailsToggles(root);
  };

  document.addEventListener('DOMContentLoaded', function () {
    window.initMarkdownEditors(document);
  });

  // Re-run after any HTMX swap settles.
  document.addEventListener('htmx:afterSettle', function (e) {
    window.initMarkdownEditors(e.target);
  });
})();
