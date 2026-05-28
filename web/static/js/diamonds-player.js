/* Diamonds Academy — Heartbeat tracking for native <video> elements */
(function(){'use strict';
var H=5000;var root=document.querySelector('[data-day-no]');if(!root)return;
var ctx={dayNo:+root.dataset.dayNo||0,userEmail:root.dataset.userEmail||''};
function lrK(d,s){return'dp.'+d+'.'+s;}
function sLR(d,s,v){try{localStorage.setItem(lrK(d,s),JSON.stringify(v));}catch(e){}}
function gLR(d,s){try{var v=localStorage.getItem(lrK(d,s));return v?JSON.parse(v):null;}catch(e){return null;}}
document.querySelectorAll('video[data-slot]').forEach(function(v){
var slot=v.dataset.slot;if(!slot||!ctx.dayNo)return;
var g=gLR(ctx.dayNo,slot);if(g&&g.position&&g.duration&&g.position<g.duration-5)v.currentTime=g.position;
setInterval(function(){if(v.paused)return;var d=v.duration||0,t=v.currentTime||0,p=d>0?Math.min(100,t/d*100):0;sLR(ctx.dayNo,slot,{position:t,duration:d,percent:p,t:Date.now()});if(ctx.userEmail)fetch('/api/progress',{method:'POST',headers:{'Content-Type':'application/json'},credentials:'same-origin',body:JSON.stringify({day_no:ctx.dayNo,slot:slot,position:t,duration:d,percent:p,seconds_delta:5})}).catch(function(){});},H);
});
})();
