/* Diamonds — akış / unlock / quiz yardımcı scripti.
 *
 * Senaryolar:
 *  1) File sekmesinde "Okudum, Devam Et" → slot=file tamamlandı işareti + quiz sekmesine git.
 *  2) Quiz formunda cevapları topla → POST /api/quiz-submit → her soruyu yeşil/kırmızı renklendir,
 *     tümü doğruysa quiz slotunu tamamlandı say + 1.5sn sonra dashboard'a dön.
 *  3) Video son %90'a ulaşınca otomatik bir sonraki sekmeye geçişi player halleder (diamonds-player.js).
 *
 * Cookie/session: fetch credentials 'same-origin' — scs session cookie'si otomatik gider.
 */
(function () {
  'use strict';

  function currentDayNo() {
    const el = document.querySelector('[data-day-no]');
    return el ? parseInt(el.dataset.dayNo, 10) : 0;
  }

  function postJSON(url, body) {
    return fetch(url, {
      method: 'POST',
      credentials: 'same-origin',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body || {}),
    });
  }

  function goToTab(tab) {
    const u = new URL(window.location.href);
    u.searchParams.set('tab', tab);
    window.location.href = u.toString();
  }

  const TAB_ORDER = ['l1', 'l2', 'l3', 'file', 'quiz'];
  function nextTab(cur) {
    const i = TAB_ORDER.indexOf(cur);
    if (i < 0 || i === TAB_ORDER.length - 1) return null;
    return TAB_ORDER[i + 1];
  }

  const Flow = {
    async markFileDone(slot) {
      slot = slot || 'file';
      const dayNo = currentDayNo();
      if (!dayNo) return;
      try {
        await postJSON('/api/slot-complete', { day_no: dayNo, slot: slot });
      } catch (e) {
        /* sessiz geç */
      }
      const nxt = nextTab(slot);
      if (nxt) {
        goToTab(nxt);
      } else {
        window.location.href = '/';
      }
    },

    async submitQuiz(form) {
      const dayNo = parseInt(form.dataset.dayNo, 10) || currentDayNo();
      const cards = form.querySelectorAll('[data-question-index]');
      const answers = [];
      let missing = false;
      cards.forEach((card) => {
        const idx = parseInt(card.dataset.questionIndex, 10);
        const sel = form.querySelector('input[name="q' + idx + '"]:checked');
        if (!sel) {
          missing = true;
          card.classList.add('border-red-500/60');
        }
        answers[idx] = sel ? parseInt(sel.value, 10) : -1;
      });
      const fb = document.getElementById('diamonds-quiz-feedback');
      if (missing) {
        if (fb) {
          fb.textContent = 'Tüm soruları cevapla.';
          fb.className = 'text-sm mono text-red-400';
        }
        return;
      }
      if (fb) {
        fb.textContent = 'Gönderiliyor…';
        fb.className = 'text-sm mono text-white/60';
      }
      let res, data;
      try {
        res = await postJSON('/api/quiz-submit', { day_no: dayNo, answers: answers });
        data = await res.json();
      } catch (e) {
        if (fb) {
          fb.textContent = 'Bağlantı hatası.';
          fb.className = 'text-sm mono text-red-400';
        }
        return;
      }

      // Her soruyu renklendir
      cards.forEach((card) => {
        const idx = parseInt(card.dataset.questionIndex, 10);
        const correctIdx = (data.answers && data.answers[idx] !== undefined)
          ? data.answers[idx]
          : parseInt(card.dataset.correct || '-1', 10);
        const userIdx = answers[idx];
        const opts = card.querySelectorAll('.diamonds-q-opt');
        opts.forEach((opt, j) => {
          opt.classList.remove('border-green-500/60', 'border-red-500/60', 'bg-green-500/10', 'bg-red-500/10');
          if (j === correctIdx) {
            opt.classList.add('border-green-500/60', 'bg-green-500/10');
          } else if (j === userIdx) {
            opt.classList.add('border-red-500/60', 'bg-red-500/10');
          }
        });
        const st = card.querySelector('.diamonds-q-status');
        if (st) {
          st.textContent = userIdx === correctIdx ? '✓ Doğru' : '✗ Yanlış';
          st.className = 'diamonds-q-status mono text-xs ' + (userIdx === correctIdx ? 'text-green-400' : 'text-red-400');
        }
      });

      if (fb) {
        if (data.passed) {
          fb.textContent = data.correct + ' / ' + data.total + ' — Tebrikler! Geçtiniz.';
          fb.className = 'text-sm mono text-green-400';
        } else {
          fb.textContent = data.correct + ' / ' + data.total + ' — Başarısız! %70 gerekli. Önceki 3 videoyu tekrar izlemelisin.';
          fb.className = 'text-sm mono text-red-400';
        }
      }

      if (data.passed) {
        setTimeout(function () {
          window.location.href = '/';
        }, 1800);
      }
    },

    // Video tamamlandığında (diamonds-player.js çağırır) slot'u işaretler.
    async markVideoComplete(slot) {
      const dayNo = currentDayNo();
      if (!dayNo || !slot) return;
      try {
        await postJSON('/api/slot-complete', { day_no: dayNo, slot: slot });
      } catch (_) {}
    },

    // Quiz sorusu olmayan günlerde "Eğitimi Tamamla" butonu için.
    async forceCompleteQuiz() {
      const dayNo = currentDayNo();
      if (!dayNo) return;
      try {
        await postJSON('/api/slot-complete', { day_no: dayNo, slot: 'quiz' });
      } catch (_) {}
      window.location.href = '/';
    },
  };

  window.DiamondsFlow = Flow;

  document.addEventListener('DOMContentLoaded', function () {
    const form = document.getElementById('diamonds-quiz');
    if (form) {
      form.addEventListener('submit', function (e) {
        e.preventDefault();
        Flow.submitQuiz(form);
      });
    }
    initCountdowns();
  });

  // ---------------- 24 saatlik kilit sayaçları
  function fmt(sec) {
    if (sec < 0) sec = 0;
    const h = Math.floor(sec / 3600);
    const m = Math.floor((sec % 3600) / 60);
    const s = sec % 60;
    const pad = (n) => String(n).padStart(2, '0');
    return pad(h) + ':' + pad(m) + ':' + pad(s);
  }

  function initCountdowns() {
    const nodes = document.querySelectorAll('[data-unlock-countdown]');
    if (!nodes.length) return;
    let reloadedOnce = false;
    function tick() {
      const now = Math.floor(Date.now() / 1000);
      let anyExpired = false;
      nodes.forEach((n) => {
        const target = parseInt(n.dataset.unlockCountdown, 10) || 0;
        const remaining = target - now;
        const label = n.querySelector('[data-countdown]') || n;
        if (label && label !== n) {
          label.textContent = fmt(remaining);
        } else if (label === n) {
          // root element — sadece title güncelle
          n.title = '24 saat sonra: ' + fmt(remaining);
        }
        if (remaining <= 0) anyExpired = true;
      });
      if (anyExpired && !reloadedOnce) {
        reloadedOnce = true;
        setTimeout(() => window.location.reload(), 800);
      }
    }
    tick();
    setInterval(tick, 1000);
  }
})();
