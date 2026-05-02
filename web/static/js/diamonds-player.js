/* Diamonds Academy — Plyr başlatıcı + öğrenme akışı
 *
 * Özellikler:
 *   1) İzleme ilerlemesi (resume): localStorage + /api/progress (kullanıcı varsa)
 *   2) Otomatik sonraki derse geç: l1 → l2 → l3 → file → quiz
 *   3) İzleme yüzdesi takibi (90%+ → slot tamamlandı sayılır)
 *   4) Klavye kısayol paneli ( ? tuşu ile aç/kapa )
 *   5) Hız hafızası (localStorage 'diamonds.player.speed')
 *   6) Watermark — kullanıcı email'i, periyodik yer değişimi
 *   7) Quality kilidi — minimum 720p (Vimeo/HTML5 default)
 *   8) PiP otomatik — sayfadan ayrılınca (gizlenince) PiP'e geç
 *   9) İzleme istatistikleri — heartbeat backend'e iletir
 *
 * HOST API:
 *   <div data-diamonds-player
 *        data-slot="l1|l2|l3"          (ilerleme + sonraki sekme için zorunlu)
 *        data-src="https://..."        (zorunlu)
 *        data-poster="..."             (opsiyonel)
 *        data-autoplay="false"
 *        data-loop="false"
 *        data-start="0"
 *        data-block-download="true"
 *   ></div>
 *
 * Sayfa root <div> üzerinde bekleniyor:
 *   data-day-no, data-active-tab, data-user-email
 */
(function () {
	'use strict';

	const SPEED_KEY = 'diamonds.player.speed';
	const HEARTBEAT_MS = 5000;
	const TAB_ORDER = ['l1', 'l2', 'l3', 'file', 'quiz'];

	const root = document.querySelector('[data-day-no]');
	const ctx = {
		dayNo: root ? parseInt(root.dataset.dayNo || '0', 10) : 0,
		activeTab: root ? root.dataset.activeTab || '' : '',
		userEmail: root ? root.dataset.userEmail || '' : '',
	};

	// ------------------------------- Plyr defaults
	const PRESET = {
		controls: ['play-large','play','progress','current-time','duration','mute','volume','settings','pip','airplay','fullscreen'],
		settings: ['quality', 'speed', 'loop'],
		speed: { selected: loadSpeed(), options: [0.5, 0.75, 1, 1.25, 1.5, 1.75, 2] },
		quality: { default: 720, options: [2160, 1440, 1080, 720] },
		seekTime: 10,
		keyboard: { focused: true, global: false },
		tooltips: { controls: true, seek: true },
		ratio: '16:9',
		invertTime: false,
		toggleInvert: true,
		hideControls: true,
		clickToPlay: true,
		disableContextMenu: true,
		fullscreen: { enabled: true, fallback: true, iosNative: true },
		i18n: {
			restart: 'Baştan', rewind: '{seektime}sn geri', play: 'Oynat', pause: 'Duraklat',
			fastForward: '{seektime}sn ileri', seek: 'Atla', seekLabel: '{currentTime} / {duration}',
			played: 'İzlenen', buffered: 'Yüklenen', currentTime: 'Şu an', duration: 'Toplam süre',
			volume: 'Ses', mute: 'Sustur', unmute: 'Sesi aç',
			enableCaptions: 'Altyazıyı aç', disableCaptions: 'Altyazıyı kapat', download: 'İndir',
			enterFullscreen: 'Tam ekran', exitFullscreen: 'Tam ekrandan çık',
			frameTitle: '{title} oynatıcısı', captions: 'Altyazılar', settings: 'Ayarlar',
			pip: 'Resim içinde resim', menuBack: 'Geri', speed: 'Hız', normal: 'Normal',
			quality: 'Kalite', loop: 'Döngü', start: 'Başla', end: 'Bitir', all: 'Tümü',
			reset: 'Sıfırla', disabled: 'Devre dışı', enabled: 'Etkin', advertisement: 'Reklam',
			qualityBadge: { 2160: '4K', 1440: 'HD', 1080: 'HD', 720: 'HD', 576: 'SD', 480: 'SD' },
		},
		youtube: { noCookie: true, rel: 0, showinfo: 0, iv_load_policy: 3, modestbranding: 1, playsinline: 1 },
		vimeo: { byline: false, portrait: false, title: false, speed: true, transparent: false, dnt: true, quality: '720p' },
	};

	// ------------------------------- Provider tespiti
	function detectProvider(url) {
		if (!url) return null;
		if (/youtu\.be\/|youtube\.com\//i.test(url)) return 'youtube';
		if (/vimeo\.com\//i.test(url)) return 'vimeo';
		return 'html5';
	}
	function ytId(u) { const m = u.match(/(?:youtu\.be\/|v=|embed\/|shorts\/)([\w-]{6,})/); return m ? m[1] : u; }
	function vmId(u) { const m = u.match(/vimeo\.com\/(?:video\/)?(\d+)/); return m ? m[1] : u; }
	function guessMime(url) {
		const ext = (url.split('.').pop() || '').toLowerCase().split('?')[0];
		return ({ mp4:'video/mp4', m4v:'video/mp4', webm:'video/webm', ogv:'video/ogg', mov:'video/quicktime' })[ext] || 'video/mp4';
	}

	function buildElement(host) {
		const src = host.dataset.src || '';
		const poster = host.dataset.poster || '';
		const provider = detectProvider(src);
		host.innerHTML = '';

		if (provider === 'youtube' || provider === 'vimeo') {
			const wrap = document.createElement('div');
			wrap.setAttribute('data-plyr-provider', provider);
			wrap.setAttribute('data-plyr-embed-id', provider === 'youtube' ? ytId(src) : vmId(src));
			host.appendChild(wrap);
			return wrap;
		}
		const video = document.createElement('video');
		video.setAttribute('playsinline', '');
		video.setAttribute('controls', '');
		if (poster) video.setAttribute('poster', poster);
		const source = document.createElement('source');
		source.src = src;
		source.type = guessMime(src);
		video.appendChild(source);
		host.appendChild(video);
		return video;
	}

	// ------------------------------- Hız hafızası
	function loadSpeed() {
		const v = parseFloat(localStorage.getItem(SPEED_KEY) || '1');
		return isFinite(v) && v > 0 ? v : 1;
	}
	function saveSpeed(v) { try { localStorage.setItem(SPEED_KEY, String(v)); } catch (_) {} }

	// ------------------------------- İlerleme (resume)
	async function fetchProgress(dayNo) {
		try {
			const res = await fetch('/api/progress/' + dayNo, { credentials: 'same-origin' });
			if (!res.ok) return {};
			return await res.json();
		} catch (_) { return {}; }
	}
	function localResumeKey(dayNo, slot) { return 'diamonds.resume.' + dayNo + '.' + slot; }
	function getLocalResume(dayNo, slot) {
		try {
			const raw = localStorage.getItem(localResumeKey(dayNo, slot));
			if (!raw) return null;
			return JSON.parse(raw);
		} catch (_) { return null; }
	}
	function setLocalResume(dayNo, slot, data) {
		try { localStorage.setItem(localResumeKey(dayNo, slot), JSON.stringify(data)); } catch (_) {}
	}

	// ------------------------------- Heartbeat
	function makeBeater(dayNo, slot) {
		let lastSent = 0;
		let lastSecondsAccum = 0;
		return function beat(player) {
			const now = Date.now();
			if (now - lastSent < HEARTBEAT_MS) return;
			lastSent = now;
			const dur = Number(player.duration) || 0;
			const pos = Number(player.currentTime) || 0;
			const pct = dur > 0 ? Math.min(100, (pos / dur) * 100) : 0;
			const delta = Math.min(30, Math.max(0, (HEARTBEAT_MS / 1000)));
			lastSecondsAccum += delta;

			setLocalResume(dayNo, slot, { position: pos, duration: dur, percent: pct, t: now });

			if (!ctx.userEmail) return; // anonim ise backend yok
			fetch('/api/progress', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				credentials: 'same-origin',
				body: JSON.stringify({
					day_no: dayNo,
					slot: slot,
					position: pos,
					duration: dur,
					percent: pct,
					seconds_delta: delta,
				}),
			}).catch(() => {});
		};
	}

	// ------------------------------- Sonraki sekmeye geç
	function gotoNextTab(currentSlot) {
		const idx = TAB_ORDER.indexOf(currentSlot);
		if (idx < 0 || idx >= TAB_ORDER.length - 1) return;
		const next = TAB_ORDER[idx + 1];
		const url = new URL(window.location.href);
		url.searchParams.set('tab', next);
		window.location.href = url.toString();
	}

	// ------------------------------- Watermark
	function attachWatermark(container, email) {
		if (!email) return;
		const wm = document.createElement('div');
		wm.textContent = email;
		wm.setAttribute('aria-hidden', 'true');
		Object.assign(wm.style, {
			position: 'absolute', zIndex: 5, pointerEvents: 'none',
			color: 'rgba(255,255,255,.35)', fontFamily: 'ui-monospace, monospace',
			fontSize: '11px', textShadow: '0 1px 2px rgba(0,0,0,.6)',
			padding: '4px 8px', userSelect: 'none', mixBlendMode: 'difference',
		});
		container.style.position = container.style.position || 'relative';
		container.appendChild(wm);
		const positions = [
			{ top: '10px', left: '12px', right: 'auto', bottom: 'auto' },
			{ top: '10px', right: '12px', left: 'auto', bottom: 'auto' },
			{ bottom: '60px', left: '12px', top: 'auto', right: 'auto' },
			{ bottom: '60px', right: '12px', top: 'auto', left: 'auto' },
		];
		let i = 0;
		const move = () => { Object.assign(wm.style, positions[i % positions.length]); i++; };
		move();
		setInterval(move, 30000);
	}

	// ------------------------------- PiP otomatik (sayfa gizlenince)
	function attachAutoPiP(player) {
		document.addEventListener('visibilitychange', () => {
			if (!document.hidden) return;
			if (player.paused || player.ended) return;
			try {
				const v = player.media;
				if (v && document.pictureInPictureEnabled && !document.pictureInPictureElement && v.requestPictureInPicture) {
					v.requestPictureInPicture().catch(() => {});
				}
			} catch (_) {}
		});
	}

	// ------------------------------- Klavye kısayol paneli
	function ensureShortcutsPanel() {
		if (document.getElementById('diamonds-shortcuts')) return;
		const panel = document.createElement('div');
		panel.id = 'diamonds-shortcuts';
		panel.innerHTML = `
			<div class="dsc-bg"></div>
			<div class="dsc-card">
				<div class="dsc-head">
					<div>Klavye Kısayolları</div>
					<button class="dsc-close" aria-label="Kapat">×</button>
				</div>
				<table>
					<tr><td><kbd>Space</kbd> / <kbd>K</kbd></td><td>Oynat / Duraklat</td></tr>
					<tr><td><kbd>←</kbd></td><td>10 sn geri</td></tr>
					<tr><td><kbd>→</kbd></td><td>10 sn ileri (sadece izlenen kısımda)</td></tr>
					<tr><td><kbd>↑</kbd> / <kbd>↓</kbd></td><td>Ses +/−</td></tr>
					<tr><td><kbd>M</kbd></td><td>Sustur</td></tr>
					<tr><td><kbd>F</kbd></td><td>Tam ekran</td></tr>
					<tr><td><kbd>P</kbd></td><td>Resim içinde resim</td></tr>
					<tr><td><kbd>?</kbd></td><td>Bu paneli aç/kapa</td></tr>
				</table>
			</div>`;
		const style = document.createElement('style');
		style.textContent = `
			#diamonds-shortcuts{position:fixed;inset:0;z-index:9999;display:none;align-items:center;justify-content:center;font-family:Inter,system-ui,sans-serif;}
			#diamonds-shortcuts.open{display:flex;}
			#diamonds-shortcuts .dsc-bg{position:absolute;inset:0;background:rgba(0,0,0,.7);backdrop-filter:blur(4px);}
			#diamonds-shortcuts .dsc-card{position:relative;background:#111;border:1px solid #2a2a2a;border-radius:18px;padding:24px 28px;min-width:340px;max-width:480px;box-shadow:0 0 60px rgba(168,85,247,.25);}
			#diamonds-shortcuts .dsc-head{display:flex;justify-content:space-between;align-items:center;font-weight:700;color:#a855f7;margin-bottom:14px;letter-spacing:.05em;font-size:14px;}
			#diamonds-shortcuts .dsc-close{background:none;border:none;color:#fff;font-size:24px;cursor:pointer;line-height:1;}
			#diamonds-shortcuts table{width:100%;border-collapse:collapse;color:#e5e5e5;font-size:13px;}
			#diamonds-shortcuts td{padding:6px 4px;border-bottom:1px solid rgba(255,255,255,.05);}
			#diamonds-shortcuts kbd{display:inline-block;padding:2px 6px;background:#1d1d1d;border:1px solid #333;border-bottom-width:2px;border-radius:6px;font-family:ui-monospace,monospace;font-size:11px;color:#c084fc;}
		`;
		document.head.appendChild(style);
		document.body.appendChild(panel);
		const close = () => panel.classList.remove('open');
		panel.querySelector('.dsc-bg').addEventListener('click', close);
		panel.querySelector('.dsc-close').addEventListener('click', close);
	}
	function toggleShortcuts() {
		ensureShortcutsPanel();
		document.getElementById('diamonds-shortcuts').classList.toggle('open');
	}
	document.addEventListener('keydown', (e) => {
		if (e.key === '?' || (e.shiftKey && e.key === '/')) {
			const tag = (e.target && e.target.tagName) || '';
			if (/INPUT|TEXTAREA|SELECT/.test(tag)) return;
			e.preventDefault();
			toggleShortcuts();
		}
	});

	// ------------------------------- İleri sarma kilidi UI
	function flashLocked(player) {
		try {
			const c = player.elements && player.elements.container;
			if (!c) return;
			let tip = c.querySelector('.dsc-locked');
			if (!tip) {
				tip = document.createElement('div');
				tip.className = 'dsc-locked';
				Object.assign(tip.style, {
					position: 'absolute', top: '14px', left: '50%', transform: 'translateX(-50%)',
					background: 'rgba(168,85,247,.95)', color: '#0a0a0a', padding: '6px 12px',
					borderRadius: '999px', fontFamily: 'Inter,system-ui,sans-serif', fontSize: '12px',
					fontWeight: '700', letterSpacing: '.02em', zIndex: 6, opacity: '0',
					transition: 'opacity .25s ease', pointerEvents: 'none',
					boxShadow: '0 6px 24px rgba(168,85,247,.45)',
				});
				tip.textContent = 'İleri sarma kapalı';
				c.appendChild(tip);
			}
			tip.style.opacity = '1';
			clearTimeout(tip.__t);
			tip.__t = setTimeout(() => { tip.style.opacity = '0'; }, 900);
		} catch (_) {}
	}

	// ------------------------------- Init
	function init(host) {
		if (host.dataset.diamondsReady === '1') return;
		const src = host.dataset.src || '';
		const slot = host.dataset.slot || '';
		if (!src) {
			host.innerHTML = '<div style="color:rgba(255,255,255,.4);font-family:ui-monospace,monospace;font-size:12px">Video kaynağı yok</div>';
			host.dataset.diamondsReady = '1';
			return;
		}
		if (typeof window.Plyr === 'undefined') { console.warn('[diamonds] Plyr yok'); return; }

		const el = buildElement(host);
		const opts = Object.assign({}, PRESET, {
			autoplay: host.dataset.autoplay === 'true',
			loop: { active: host.dataset.loop === 'true' },
		});
		const player = new window.Plyr(el, opts);
		host.__plyr = player;
		host.dataset.diamondsReady = '1';

		const beat = makeBeater(ctx.dayNo, slot);

		// İleri sarma kilidi: kullanıcı sadece izlediği en uzak noktaya kadar atlayabilir.
		// Geri sarma serbest. Hız değiştirme serbest.
		let maxAllowed = 0;
		const SEEK_EPS = 1.5; // saniye toleransı (buffering / küçük yuvarlama)
		function clampSeek() {
			try {
				const cur = Number(player.currentTime) || 0;
				if (cur > maxAllowed + SEEK_EPS) {
					player.currentTime = maxAllowed;
					flashLocked(player);
				}
			} catch (_) {}
		}

		player.on('ready', async () => {
			// Watermark
			const container = player.elements && player.elements.container;
			if (container) attachWatermark(container, ctx.userEmail);
			// Auto PiP
			attachAutoPiP(player);
			// Sağ tık engelle
			if ((host.dataset.blockDownload || 'true') === 'true' && container) {
				container.addEventListener('contextmenu', (e) => e.preventDefault());
			}
			// Resume
			let startAt = parseFloat(host.dataset.start || '0');
			if (slot && ctx.dayNo) {
				const local = getLocalResume(ctx.dayNo, slot);
				if (local && local.position && local.duration && local.position < local.duration - 5) {
					startAt = Math.max(startAt, local.position);
					maxAllowed = Math.max(maxAllowed, local.position);
				}
				if (ctx.userEmail) {
					const remote = await fetchProgress(ctx.dayNo);
					const r = remote && remote[slot];
					if (r && r.position && r.duration) {
						maxAllowed = Math.max(maxAllowed, r.position);
						if (r.position < r.duration - 5 && r.position > startAt) {
							startAt = r.position;
						}
					}
				}
			}
			if (startAt > 0) { try { player.currentTime = startAt; } catch (_) {} }
			maxAllowed = Math.max(maxAllowed, startAt);
			// Hız hafızası uygula
			try { player.speed = loadSpeed(); } catch (_) {}
		});

		player.on('ratechange', () => {
			const s = Number(player.speed);
			if (isFinite(s) && s > 0) saveSpeed(s);
		});

		player.on('timeupdate', () => {
			const cur = Number(player.currentTime) || 0;
			if (cur > maxAllowed && cur <= maxAllowed + SEEK_EPS + 0.5) {
				maxAllowed = cur;
			}
			beat(player);
		});
		player.on('seeking', clampSeek);
		player.on('seeked', () => { clampSeek(); beat(player); });
		player.on('pause', () => beat(player));
		player.on('ended', () => {
			beat(player);
			if (slot) gotoNextTab(slot);
		});
	}

	function initAll(scope) {
		(scope || document).querySelectorAll('[data-diamonds-player]').forEach(init);
	}

	if (document.readyState === 'loading') {
		document.addEventListener('DOMContentLoaded', () => initAll());
	} else {
		initAll();
	}
	if (document.body) {
		document.body.addEventListener('htmx:afterSwap', (e) => initAll(e.target));
	}

	window.DiamondsPlayer = { init, initAll, toggleShortcuts, ctx };
})();
