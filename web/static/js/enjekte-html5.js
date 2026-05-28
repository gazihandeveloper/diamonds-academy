/* diamonds-player.js — Plyr + forward seek lock + captions */
(function(){'use strict';
var H=5000;
var root=document.querySelector('[data-day-no]');if(!root)return;
var C={dayNo:+root.dataset.dayNo||0,userEmail:root.dataset.userEmail||'',locale:root.dataset.locale||'tr'};
var SPEEDS=[0.5,0.75,1,1.25,1.5,1.75,2];

function stK(d,s){return'dp.'+d+'.'+s;}
function stS(d,s,v){try{localStorage.setItem(stK(d,s),JSON.stringify(v));}catch(e){}}
function stL(d,s){try{var v=localStorage.getItem(stK(d,s));return v?JSON.parse(v):null;}catch(e){return null;}}
function ytId(u){var m=u.match(/(?:youtu\.be\/|v=|embed\/|shorts\/)([\w-]{6,})/);return m?m[1]:'';}

function init(){
document.querySelectorAll('[data-diamonds-player]:not([data-ready])').forEach(function(h){
if(h.dataset.ready)return;h.dataset.ready='1';
var src=h.dataset.src||'',slot=h.dataset.slot||'',loc=h.dataset.locale||C.locale;
if(!src){h.innerHTML='<div style="color:rgba(255,255,255,.4);font-size:14px;padding:40px;text-align:center">Kaynak yok</div>';return;}

var isYT=/youtu\.be\/|youtube\.com\//i.test(src);
var maxSeen=0;
var plyr=null;
h.innerHTML='';

// Build Plyr element
var el;
if(isYT){
var id=ytId(src);if(!id){h.textContent='Geçersiz URL';return;}
el=document.createElement('div');
el.setAttribute('data-plyr-provider','youtube');
el.setAttribute('data-plyr-embed-id',id);
// Add caption tracks from DB via /subtitles
['tr','en','bg'].forEach(function(l){
var t=document.createElement('track');t.kind='captions';t.label=l.toUpperCase();t.srclang=l;
t.src='/subtitles?v='+id+'&lang='+l;if(l===loc)t.setAttribute('default','');
el.appendChild(t);
});
}else{
el=document.createElement('video');el.setAttribute('playsinline','');
var s=document.createElement('source');s.src=src;s.type='video/mp4';el.appendChild(s);
// Caption tracks for native video
['tr','en','bg'].forEach(function(l){
var t=document.createElement('track');t.kind='captions';t.label=l.toUpperCase();t.srclang=l;
t.src='/subtitles?v='+encodeURIComponent(src)+'&lang='+l;if(l===loc)t.setAttribute('default','');
el.appendChild(t);
});
}
h.appendChild(el);

var maxSeen=0;
var player=new Plyr(el,{
controls:['play-large','play','progress','current-time','duration','mute','volume','settings','captions','pip','fullscreen'],
settings:['captions','quality','speed'],
captions:{active:true,language:loc},
speed:{selected:+localStorage.getItem('dp.speed')||1,options:SPEEDS},
quality:{default:720,options:[2160,1440,1080,720]},
seekTime:10,
keyboard:{focused:true,global:false},
tooltips:{controls:true,seek:true},
ratio:'16:9',
invertTime:false,hideControls:true,clickToPlay:true,disableContextMenu:true,
fullscreen:{enabled:true,fallback:true,iosNative:true},
youtube:{noCookie:true,rel:0,modestbranding:1,playsinline:1,cc_load_policy:1,hl:loc,cc_lang_pref:loc},
i18n:{restart:'Baştan',rewind:'{seektime}sn geri',play:'Oynat',pause:'Duraklat',fastForward:'{seektime}sn ileri',seek:'Atla',played:'İzlenen',buffered:'Yüklenen',currentTime:'Şu an',duration:'Toplam süre',volume:'Ses',mute:'Sustur',unmute:'Sesi aç',enableCaptions:'Altyazıyı aç',disableCaptions:'Altyazıyı kapat',enterFullscreen:'Tam ekran',exitFullscreen:'Tam ekrandan çık',captions:'Altyazılar',settings:'Ayarlar',speed:'Hız',normal:'Normal',quality:'Kalite',loop:'Döngü',start:'Başla',end:'Bitir',disabled:'Devre dışı',enabled:'Etkin'},
});

// Forward seek lock
player.on('ready',function(){
// Restore position
if(slot&&C.dayNo){
var g=stL(C.dayNo,slot);
if(g&&g.position&&g.duration&&g.position<g.duration-5){
maxSeen=g.position;player.currentTime=g.position;
}
}
// Set saved speed
var sp=+localStorage.getItem('dp.speed')||1;
if(SPEEDS.indexOf(sp)>=0){try{player.speed=sp;}catch(e){}}
});

player.on('timeupdate',function(){
var t=player.currentTime||0,d=player.duration||0;
if(maxSeen>0&&t>maxSeen+1.5){player.currentTime=maxSeen;return;}
if(t>maxSeen)maxSeen=t;
});
player.on('seeking',function(){
var t=player.currentTime||0;
if(maxSeen>0&&t>maxSeen+1.5){player.currentTime=maxSeen;}
});
player.on('ratechange',function(){try{localStorage.setItem('dp.speed',String(player.speed));}catch(e){}});

// Heartbeat
var lastBeat=0;
player.on('timeupdate',function(){
var t=player.currentTime||0,d=player.duration||0,p=d>0?Math.min(100,t/d*100):0;
stS(C.dayNo,slot,{position:t,duration:d,percent:p,t:Date.now()});
if(!C.userEmail||Date.now()-lastBeat<H)return;
lastBeat=Date.now();
fetch('/api/progress',{
method:'POST',headers:{'Content-Type':'application/json'},credentials:'same-origin',
body:JSON.stringify({day_no:C.dayNo,slot:slot,position:t,duration:d,percent:p,seconds_delta:5})
}).catch(function(){});
});

});
}

if(document.readyState==='loading')document.addEventListener('DOMContentLoaded',init);else init();
document.body.addEventListener('htmx:afterSwap',function(){setTimeout(init,100);});
})();
