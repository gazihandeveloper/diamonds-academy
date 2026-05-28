/* plyr-init.js — Plyr + forward seek lock + DB transcript overlay */
(function() {

function ready(fn) { if (document.readyState !== 'loading') fn(); else document.addEventListener('DOMContentLoaded', fn); }

function parseVTT(vtt) {
	var cues = [];
	vtt = vtt.replace(/^\uFEFF/, '').replace(/\r\n/g, '\n');
	var parts = vtt.split('\n\n');
	for (var i = 1; i < parts.length; i++) {
		var p = parts[i].trim();
		if (!p) continue;
		var lines = p.split('\n');
		if (lines.length < 2) continue;
		var timeLine = lines[0];
		var tMatch = timeLine.match(/([\d:.]+)\s*-->\s*([\d:.]+)/);
		if (!tMatch) { lines.unshift(''); timeLine = lines[0]; tMatch = timeLine.match(/([\d:.]+)\s*-->\s*([\d:.]+)/); if (!tMatch) continue; }
		function toSec(s) { var p = s.split(':'); if (p.length === 3) return +p[0]*3600 + +p[1]*60 + parseFloat(p[2]); return +p[0]*60 + parseFloat(p[1]); }
		var start = toSec(tMatch[1]), end = toSec(tMatch[2]);
		var text = lines.slice(1).join(' ').replace(/<[^>]+>/g, '').trim();
		if (text) cues.push({ start: start, end: end, text: text });
	}
	return cues;
}

ready(function() {
	var root = document.querySelector('[data-day-no]');
	if (!root) return;
	var dayNo = parseInt(root.dataset.dayNo || '0', 10);
	var email = root.dataset.userEmail || '';
	var speed = parseFloat(localStorage.getItem('dp.speed') || '1');
	var maxSeen = {};

	function waitPlyr(cb) {
		if (window.Plyr) { cb(); return; }
		var t = setInterval(function() { if (window.Plyr) { clearInterval(t); cb(); } }, 100);
		setTimeout(function() { clearInterval(t); }, 10000);
	}

	document.querySelectorAll('[data-diamonds-player]').forEach(function(el) {
		if (el.dataset.plyrReady) return;
		var src = el.dataset.src || '', slot = el.dataset.slot || '', locale = el.dataset.locale || 'tr';
		if (!src) return;

		waitPlyr(function() {
			var isYT = /youtu\.be\/|youtube\.com\//i.test(src), id = '';
			if (isYT) {
				var m = src.match(/(?:youtu\.be\/|v=|embed\/|shorts\/)([\w-]{6,})/);
				id = m ? m[1] : '';
				if (!id) { el.textContent = 'Geçersiz URL'; return; }
			}
			el.innerHTML = '';
			var playerEl;
			if (isYT) {
				playerEl = document.createElement('div');
				playerEl.setAttribute('data-plyr-provider', 'youtube');
				playerEl.setAttribute('data-plyr-embed-id', id);
			} else {
				playerEl = document.createElement('video');
				playerEl.setAttribute('playsinline', '');
				var se = document.createElement('source');
				se.src = src; se.type = 'video/mp4';
				playerEl.appendChild(se);
			}
			el.appendChild(playerEl);
			el.style.position = 'relative';
			el.dataset.plyrReady = '1';

			var player = new Plyr(playerEl, {
				controls: ['play-large','play','progress','current-time','duration','mute','volume','settings','pip','fullscreen'],
				settings: ['speed'],
				speed: { selected: speed, options: [0.5, 0.75, 1, 1.25, 1.5, 1.75, 2] },
				seekTime: 10, hideControls: true, clickToPlay: true,
				fullscreen: { enabled: true, fallback: true },
				youtube: { noCookie: true, rel: 0, modestbranding: 1, cc_load_policy: 0 },
			});

			player.on('ready', function() {
				try { player.speed = speed; } catch(e) {}
				var k = slot ? dayNo + '.' + slot : '';
				if (k) {
					try {
						var r = JSON.parse(localStorage.getItem('dp.' + k));
						if (r && r.position && r.duration && r.position < r.duration - 5) {
							maxSeen[k] = r.position;
							player.currentTime = r.position;
						}
					} catch(e) {}
				}
				if (k && email) {
					setInterval(function() {
						var t = player.currentTime || 0, d = player.duration || 0, p = d > 0 ? Math.min(100, t/d*100) : 0;
						try { localStorage.setItem('dp.' + k, JSON.stringify({position:t, duration:d, percent:p, t:Date.now()})); } catch(e) {}
						fetch('/api/progress', { method:'POST', headers:{'Content-Type':'application/json'}, credentials:'same-origin', body:JSON.stringify({day_no:dayNo, slot:slot, position:t, duration:d, percent:p, seconds_delta:5}) }).catch(function(){});
					}, 5000);
				}
				// Transcript overlay from DB
				if (id) {
					var ov = document.createElement('div');
					ov.style.cssText = 'position:absolute;bottom:56px;left:50%;transform:translateX(-50%);z-index:12;max-width:85%;padding:6px 14px;border-radius:8px;background:rgba(0,0,0,.75);color:#fff;font-size:13px;text-align:center;line-height:1.35;pointer-events:none;font-family:Inter,sans-serif;text-shadow:0 1px 2px rgba(0,0,0,.4);overflow:hidden;max-height:2.7em;display:none';
					el.appendChild(ov);
					fetch('/subtitles?v=' + id + '&lang=' + locale).then(function(r) { return r.text(); }).then(function(vtt) {
						var cues = parseVTT(vtt);
						if (!cues.length) { ov.textContent = 'Altyazı yok'; ov.style.display = 'block'; return; }
						player.on('timeupdate', function() {
							var t = player.currentTime || 0, cue = null;
							for (var i = 0; i < cues.length; i++) { if (t >= cues[i].start && t <= cues[i].end) { cue = cues[i]; break; } }
							if (cue) { ov.textContent = cue.text; ov.style.display = 'block'; }
							else { ov.style.display = 'none'; }
						});
					}.catch(function() { ov.textContent = 'Altyazı yüklenemedi'; ov.style.display = '-webkit-box'; });
				}
			});


			player.on('timeupdate', function() {
				var t = player.currentTime || 0, k = slot ? dayNo + '.' + slot : '';
				if (k && maxSeen[k] > 0 && t > maxSeen[k] + 1.5) { player.currentTime = maxSeen[k]; return; }
				if (k && t > (maxSeen[k] || 0)) maxSeen[k] = t;
			});

			player.on('ratechange', function() { try { localStorage.setItem('dp.speed', String(player.speed)); } catch(e) {} });
		});
	});
});
})();
