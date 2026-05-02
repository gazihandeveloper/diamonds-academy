/* Diamonds Admin — Quiz Builder
 *
 * #quiz-questions konteynerine accordion soru kartları render eder.
 * Her kart: soru metni, 2-6 şık, doğru şık seçimi (radio).
 * Submit'te (#quiz-json-output) textarea'sına JSON serialize edilir:
 *   [{ "q":"...", "options":["...","..."], "correct":0 }, ...]
 *
 * Mevcut quiz_json varsa parse edilip yüklenir; yoksa boş başlar.
 */
(function () {
  'use strict';

  const out = document.getElementById('quiz-json-output');
  const list = document.getElementById('quiz-questions');
  const addBtn = document.getElementById('quiz-add-question');
  const preview = document.getElementById('quiz-json-preview');
  if (!out || !list || !addBtn) return;

  const form = out.closest('form');

  // ------------ State
  /** @type {{q:string, options:string[], correct:number, open:boolean}[]} */
  let state = [];
  try {
    const raw = (out.value || '').trim();
    if (raw) {
      const parsed = JSON.parse(raw);
      if (Array.isArray(parsed)) {
        state = parsed.map((it) => ({
          q: String(it.q || ''),
          options: Array.isArray(it.options) ? it.options.map(String) : [],
          correct: Number.isInteger(it.correct) ? it.correct : 0,
          open: false,
        }));
      }
    }
  } catch (e) {
    /* parse hatası → boş başla */
  }

  // ------------ Helpers
  function letter(i) {
    return String.fromCharCode(65 + i);
  }

  function escapeHtml(s) {
    return String(s)
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;')
      .replace(/'/g, '&#39;');
  }

  function serialize() {
    const cleaned = state.map((q) => ({
      q: q.q.trim(),
      options: q.options.map((o) => o.trim()),
      correct: q.correct,
    }));
    out.value = JSON.stringify(cleaned, null, 2);
    if (preview) preview.textContent = out.value || '[]';
  }

  function summary(q, idx) {
    const text = (q.q || '').trim();
    const head = text ? text.slice(0, 60) + (text.length > 60 ? '…' : '') : '(soru yok)';
    return idx + 1 + '. ' + escapeHtml(head);
  }

  function render() {
    list.innerHTML = '';
    state.forEach((q, idx) => {
      const card = document.createElement('div');
      card.className = 'rounded border border-line bg-ink-900';

      // Header
      const header = document.createElement('button');
      header.type = 'button';
      header.className = 'w-full flex items-center justify-between gap-3 px-3 py-2.5 text-left hover:bg-white/5 transition';
      header.innerHTML =
        '<div class="flex items-center gap-2 min-w-0">' +
          '<span class="mono text-[10px] text-accent shrink-0">SORU ' + (idx + 1) + '</span>' +
          '<span class="text-xs text-white/85 truncate">' + summary(q, idx) + '</span>' +
        '</div>' +
        '<div class="flex items-center gap-2 shrink-0">' +
          '<span class="mono text-[10px] text-muted">' + q.options.length + ' şık</span>' +
          '<svg class="w-3.5 h-3.5 text-white/50 transition-transform ' + (q.open ? 'rotate-180' : '') + '" fill="currentColor" viewBox="0 0 24 24"><path d="M7 10l5 5 5-5z"/></svg>' +
        '</div>';
      header.addEventListener('click', () => {
        state[idx].open = !state[idx].open;
        render();
      });
      card.appendChild(header);

      if (!q.open) {
        list.appendChild(card);
        return;
      }

      // Body
      const body = document.createElement('div');
      body.className = 'px-3 pb-3 pt-2 border-t border-line space-y-3';

      // Soru metni
      const qBlock = document.createElement('div');
      qBlock.innerHTML =
        '<label class="block mono text-[10px] tracking-[0.2em] text-muted mb-1">SORU METNİ</label>';
      const qIn = document.createElement('textarea');
      qIn.rows = 2;
      qIn.value = q.q;
      qIn.placeholder = 'Soruyu yaz…';
      qIn.className = 'w-full px-3 py-2 rounded bg-ink-700 border border-line text-white focus:border-accent focus:outline-none text-sm';
      qIn.addEventListener('input', () => {
        state[idx].q = qIn.value;
        serialize();
        // başlığı canlı güncelle (yeniden render etmeden)
        const span = header.querySelector('span.text-xs');
        if (span) span.innerHTML = summary(state[idx], idx);
      });
      qBlock.appendChild(qIn);
      body.appendChild(qBlock);

      // Şıklar
      const optHead = document.createElement('div');
      optHead.className = 'flex items-center justify-between';
      optHead.innerHTML = '<span class="mono text-[10px] tracking-[0.2em] text-muted">ŞIKLAR</span>';
      const addOptBtn = document.createElement('button');
      addOptBtn.type = 'button';
      addOptBtn.className = 'mono text-[10px] px-2 py-1 rounded bg-accent/20 text-accent border border-accent/40 hover:bg-accent hover:text-ink-900 transition';
      addOptBtn.textContent = '+ ŞIK';
      addOptBtn.disabled = q.options.length >= 6;
      if (addOptBtn.disabled) addOptBtn.classList.add('opacity-40', 'cursor-not-allowed');
      addOptBtn.addEventListener('click', () => {
        if (state[idx].options.length >= 6) return;
        state[idx].options.push('');
        render();
      });
      optHead.appendChild(addOptBtn);
      body.appendChild(optHead);

      const optList = document.createElement('div');
      optList.className = 'space-y-2';
      q.options.forEach((opt, j) => {
        const row = document.createElement('div');
        row.className = 'flex items-center gap-2';

        const radioWrap = document.createElement('label');
        radioWrap.className = 'flex items-center gap-2 cursor-pointer';
        radioWrap.title = 'Doğru cevap olarak işaretle';
        const radio = document.createElement('input');
        radio.type = 'radio';
        radio.name = 'correct-' + idx;
        radio.checked = q.correct === j;
        radio.className = 'accent-accent';
        radio.addEventListener('change', () => {
          state[idx].correct = j;
          serialize();
          render();
        });
        const badge = document.createElement('span');
        const isRight = q.correct === j;
        badge.className =
          'w-7 h-7 rounded flex items-center justify-center text-xs font-bold mono border ' +
          (isRight
            ? 'bg-accent text-ink-900 border-accent'
            : 'bg-ink-900 text-white/60 border-line');
        badge.textContent = letter(j);
        radioWrap.appendChild(radio);
        radioWrap.appendChild(badge);
        row.appendChild(radioWrap);

        const optIn = document.createElement('input');
        optIn.type = 'text';
        optIn.value = opt;
        optIn.placeholder = 'Şık ' + letter(j);
        optIn.className = 'flex-1 px-3 py-1.5 rounded bg-ink-700 border border-line text-white focus:border-accent focus:outline-none text-sm';
        optIn.addEventListener('input', () => {
          state[idx].options[j] = optIn.value;
          serialize();
        });
        row.appendChild(optIn);

        const del = document.createElement('button');
        del.type = 'button';
        del.className = 'mono text-[10px] px-2 py-1 rounded border border-line text-white/60 hover:text-red-400 hover:border-red-400/40 transition';
        del.textContent = '✕';
        del.title = 'Şıkkı sil';
        del.disabled = q.options.length <= 2;
        if (del.disabled) del.classList.add('opacity-30', 'cursor-not-allowed');
        del.addEventListener('click', () => {
          if (state[idx].options.length <= 2) return;
          state[idx].options.splice(j, 1);
          if (state[idx].correct >= state[idx].options.length) {
            state[idx].correct = state[idx].options.length - 1;
          } else if (state[idx].correct > j) {
            state[idx].correct -= 1;
          }
          render();
        });
        row.appendChild(del);

        optList.appendChild(row);
      });
      body.appendChild(optList);

      // Kart altı: soru sil
      const foot = document.createElement('div');
      foot.className = 'flex justify-end pt-1';
      const delQ = document.createElement('button');
      delQ.type = 'button';
      delQ.className = 'mono text-[10px] px-3 py-1.5 rounded border border-red-500/30 text-red-300 hover:bg-red-500/10 transition';
      delQ.textContent = 'SORUYU SİL';
      delQ.addEventListener('click', () => {
        if (!confirm('Bu soruyu silmek istediğine emin misin?')) return;
        state.splice(idx, 1);
        render();
      });
      foot.appendChild(delQ);
      body.appendChild(foot);

      card.appendChild(body);
      list.appendChild(card);
    });

    if (state.length === 0) {
      const empty = document.createElement('div');
      empty.className = 'px-4 py-6 rounded border border-dashed border-line text-center text-xs text-muted mono';
      empty.textContent = 'Henüz soru yok. Yukarıdaki + SORU EKLE butonuna bas.';
      list.appendChild(empty);
    }

    serialize();
  }

  addBtn.addEventListener('click', () => {
    state.push({
      q: '',
      options: ['', ''],
      correct: 0,
      open: true,
    });
    render();
  });

  // Submit'te boş soruları temizle (q ve >=2 dolu opsiyon zorunlu)
  if (form) {
    form.addEventListener('submit', () => {
      state = state
        .map((q) => ({
          ...q,
          q: q.q.trim(),
          options: q.options.map((o) => o.trim()).filter((o) => o.length > 0),
        }))
        .filter((q) => q.q && q.options.length >= 2)
        .map((q) => ({
          ...q,
          correct: Math.min(Math.max(q.correct, 0), q.options.length - 1),
        }));
      serialize();
    });
  }

  render();
})();
