'use strict';
/* Vantix Utilities — interfaz */
const T = new URLSearchParams(location.search).get('t') || '';
const $ = s => document.querySelector(s);
const $$ = s => [...document.querySelectorAll(s)];

// ---------------------------------------------------------------- iconos
const P = {
  grid:'<rect x="3" y="3" width="7" height="7" rx="1.5"/><rect x="14" y="3" width="7" height="7" rx="1.5"/><rect x="3" y="14" width="7" height="7" rx="1.5"/><rect x="14" y="14" width="7" height="7" rx="1.5"/>',
  activity:'<path d="M3 12h4l3-8 4 16 3-8h4"/>',
  unlock:'<rect x="4" y="11" width="16" height="10" rx="2"/><path d="M8 11V7a4 4 0 0 1 7.5-2"/>',
  search:'<circle cx="11" cy="11" r="7"/><path d="m20 20-3.5-3.5"/>',
  book:'<path d="M4 5a2 2 0 0 1 2-2h13v16H6a2 2 0 0 0-2 2z"/><path d="M4 21V5M19 19v2H6"/>',
  sliders:'<path d="M4 6h10M18 6h2M4 12h4M12 12h8M4 18h12M20 18h0"/><circle cx="16" cy="6" r="2"/><circle cx="10" cy="12" r="2"/><circle cx="18" cy="18" r="2"/>',
  shield:'<path d="M12 3 4.5 6v6c0 4.5 3.2 7.8 7.5 9 4.3-1.2 7.5-4.5 7.5-9V6z"/><path d="m9 12 2 2 4-4"/>',
  cpu:'<rect x="6" y="6" width="12" height="12" rx="2"/><rect x="9.5" y="9.5" width="5" height="5" rx=".5"/><path d="M9 2v3M15 2v3M9 19v3M15 19v3M2 9h3M2 15h3M19 9h3M19 15h3"/>',
  memory:'<rect x="3" y="7" width="18" height="10" rx="1.5"/><path d="M7 7v10M11 7v10M15 7v10M3 20h18M6 17v3M18 17v3"/>',
  layers:'<path d="m12 3 9 5-9 5-9-5z"/><path d="m3 13 9 5 9-5"/>',
  clock:'<circle cx="12" cy="12" r="9"/><path d="M12 7v5l3 2"/>',
  chev:'<path d="m9 6 6 6-6 6"/>',
  zap:'<path d="M13 2 4 14h7l-1 8 9-12h-7z"/>',
  refresh:'<path d="M20 11a8 8 0 0 0-14.5-4.5L3 9M4 13a8 8 0 0 0 14.5 4.5L21 15"/><path d="M3 4v5h5M21 20v-5h-5"/>',
  pause:'<rect x="6" y="5" width="4" height="14" rx="1"/><rect x="14" y="5" width="4" height="14" rx="1"/>',
  play:'<path d="M7 5v14l11-7z"/>',
  x:'<path d="M18 6 6 18M6 6l12 12"/>',
  folder:'<path d="M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"/>',
  file:'<path d="M14 3H7a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V8z"/><path d="M14 3v5h5"/>',
  scan:'<path d="M4 8V6a2 2 0 0 1 2-2h2M16 4h2a2 2 0 0 1 2 2v2M20 16v2a2 2 0 0 1-2 2h-2M8 20H6a2 2 0 0 1-2-2v-2M4 12h16"/>',
  trash:'<path d="M4 7h16M10 11v6M14 11v6M6 7l1 12a2 2 0 0 0 2 2h6a2 2 0 0 0 2-2l1-12M9 7V4h6v3"/>',
  filter:'<path d="M4 5h16l-6 8v5l-4 2v-7z"/>',
  download:'<path d="M12 4v11M7 10l5 5 5-5M5 20h14"/>',
  info:'<circle cx="12" cy="12" r="9"/><path d="M12 11v5M12 8h.01"/>',
  alert:'<path d="M10.3 4.2 2.6 18a2 2 0 0 0 1.7 3h15.4a2 2 0 0 0 1.7-3L13.7 4.2a2 2 0 0 0-3.4 0z"/><path d="M12 9v4M12 17h.01"/>',
  check:'<circle cx="12" cy="12" r="9"/><path d="m8 12 3 3 5-6"/>',
  lock:'<rect x="4" y="11" width="16" height="10" rx="2"/><path d="M8 11V7a4 4 0 0 1 8 0v4"/>',
  keyboard:'<rect x="2" y="6" width="20" height="12" rx="2"/><path d="M6 10h.01M10 10h.01M14 10h.01M18 10h.01M7 14h10"/>',
  help:'<circle cx="12" cy="12" r="9"/><path d="M9.5 9a2.5 2.5 0 0 1 4.9.8c0 1.7-2.4 2.2-2.4 3.7M12 17h.01"/>',
  plus:'<path d="M12 5v14M5 12h14"/>',
  external:'<path d="M14 4h6v6M20 4l-9 9M18 14v5a1 1 0 0 1-1 1H5a1 1 0 0 1-1-1V7a1 1 0 0 1 1-1h5"/>',
  copy:'<rect x="8" y="8" width="12" height="12" rx="2"/><path d="M16 8V6a2 2 0 0 0-2-2H6a2 2 0 0 0-2 2v8a2 2 0 0 0 2 2h2"/>',
  ban:'<circle cx="12" cy="12" r="9"/><path d="m5.6 5.6 12.8 12.8"/>',
  disk:'<rect x="3" y="5" width="18" height="14" rx="2"/><path d="M7 15h.01M11 15h6"/>',
  usb:'<path d="M9 3h6v6H9zM7 9h10v8a5 5 0 0 1-10 0z"/>',
};
const ic = (n, cls='') => `<svg class="${cls}" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.9" stroke-linecap="round" stroke-linejoin="round">${P[n]||''}</svg>`;
function paintIcons(root=document){ root.querySelectorAll('i[data-ic]').forEach(el => { el.outerHTML = ic(el.dataset.ic); }); }
paintIcons();

// ---------------------------------------------------------------- utilidades
const esc = s => String(s ?? '').replace(/[&<>"']/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
function fmtBytes(b, d){ b = +b||0; if (b < 1024) return b + ' B'; const u=['KB','MB','GB','TB']; let i=-1; do{ b/=1024; i++; }while(b>=1024 && i<u.length-1); const dec = d ?? (b < 10 ? 1 : 0); return b.toFixed(dec).replace('.',',') + ' ' + u[i]; }
const fmtN = n => Number(n||0).toLocaleString('es-ES');
const fmtPct = (v, d=0) => (Number(v)||0).toFixed(d).replace('.',',') + ' %';
const fmtMs = ms => ms < 1000 ? ms + ' ms' : (ms/1000).toFixed(1).replace('.',',') + ' s';
function fmtUptime(s){ const d=Math.floor(s/86400), h=Math.floor(s%86400/3600), m=Math.floor(s%3600/60); return d ? `${d} d ${h} h` : h ? `${h} h ${m} min` : `${m} min`; }
const dtf = new Intl.DateTimeFormat('es-ES',{day:'2-digit',month:'2-digit',year:'numeric',hour:'2-digit',minute:'2-digit'});
const fmtDate = s => dtf.format(new Date(s*1000));

async function api(path, body){
  const opt = {headers:{'X-Token':T}};
  if (body !== undefined){ opt.method='POST'; opt.headers['Content-Type']='application/json'; opt.body=JSON.stringify(body); }
  const r = await fetch('/api/' + path, opt);
  if (!r.ok) throw new Error((await r.text()) || ('Error ' + r.status));
  const ct = r.headers.get('content-type') || '';
  return ct.includes('json') ? r.json() : r.text();
}

function toast(title, kind='info', sub=''){
  const el = document.createElement('div');
  el.className = 'toast ' + kind;
  el.innerHTML = ic({ok:'check',warn:'alert',bad:'alert',info:'info'}[kind]||'info') + `<div><div>${esc(title)}</div>${sub?`<small>${esc(sub)}</small>`:''}</div>`;
  $('#toasts').appendChild(el);
  setTimeout(() => { el.style.transition='opacity .25s'; el.style.opacity='0'; setTimeout(() => el.remove(), 260); }, sub ? 5200 : 3200);
}

// Diálogo modal genérico. Devuelve una promesa con true/false (o el valor de onOk).
let modalOpen = false;
function modal({title, text='', body='', icon='info', tone='', ok='Aceptar', okClass='primary', cancel='Cancelar', wide=false, footLeft='', onMount, onOk}){
  return new Promise(resolve => {
    modalOpen = true;
    const bg = $('#modalBg');
    bg.innerHTML = `<div class="modal${wide?' wide':''}" role="dialog">
      <div class="mh"><div class="mi ${tone}">${ic(icon)}</div><div><h3>${esc(title)}</h3>${text?`<p>${text}</p>`:''}</div></div>
      ${body?`<div class="mb">${body}</div>`:''}
      <div class="mf"><span class="left">${footLeft}</span>${cancel?`<button class="btn" data-r="0">${esc(cancel)}</button>`:''}<button class="btn ${okClass}" data-r="1">${esc(ok)}</button></div></div>`;
    bg.classList.add('on');
    const m = bg.firstElementChild;
    const close = v => { bg.classList.remove('on'); bg.innerHTML=''; modalOpen=false; document.removeEventListener('keydown', key, true); resolve(v); };
    const key = e => { if (e.key==='Escape'){ e.stopPropagation(); close(false); } };
    document.addEventListener('keydown', key, true);
    m.querySelector('[data-r="1"]').onclick = () => close(onOk ? onOk(m) : true);
    const c = m.querySelector('[data-r="0"]'); if (c) c.onclick = () => close(false);
    bg.onmousedown = e => { if (e.target === bg) close(false); };
    if (onMount) onMount(m);
    setTimeout(() => m.querySelector('[data-r="1"]').focus(), 30);
  });
}

async function copyText(t, msg){
  try{ await navigator.clipboard.writeText(t); }
  catch(e){ const ta=document.createElement('textarea'); ta.value=t; document.body.appendChild(ta); ta.select(); document.execCommand('copy'); ta.remove(); }
  toast(msg, 'ok');
}

// ---------------------------------------------------------------- estado global
let CFG = {confirmKill:true, includeHeavy:false, heavyMB:400, refresh:3, exclusions:[], alwaysClose:[]};
let STATUS = {admin:false};
let page = 'overview';

async function loadConfig(){ try{ CFG = await api('config'); }catch(e){} applySettingsUI(); }
async function saveConfig(){ try{ const {search, ...c} = CFG; CFG = await api('config', c); }catch(e){ toast('No se pudieron guardar los ajustes', 'bad'); } }

async function loadStatus(){
  try{ STATUS = await api('status'); }catch(e){ return; }
  $('#privDot').className = 'dot ' + (STATUS.admin ? 'ok' : 'warn');
  $('#privTitle').textContent = STATUS.admin ? 'Administrador' : 'Usuario estándar';
  $('#privText').textContent = STATUS.admin ? 'Todas las funciones están disponibles.' : 'Algunas acciones sobre procesos del sistema necesitan permisos de administrador.';
  $('#btnElevate').style.display = STATUS.admin ? 'none' : '';
  $('#ovHost').textContent = [STATUS.host, STATUS.user].filter(Boolean).join(' · ');
  $('#aboutVer').textContent = 'v' + (STATUS.version||'');
}

// ---------------------------------------------------------------- navegación
function go(p){
  if (!$('#p-' + p)) return;
  page = p;
  $$('.nav').forEach(n => n.classList.toggle('on', n.dataset.page === p));
  $$('.page').forEach(s => s.classList.toggle('on', s.id === 'p-' + p));
  schedule(true);
  if (p === 'search') setTimeout(() => $('#q').focus(), 20);
  if (p === 'settings') applySettingsUI();
}
$$('.nav').forEach(n => n.onclick = () => go(n.dataset.page));
document.addEventListener('click', e => { const g = e.target.closest('[data-go]'); if (g) go(g.dataset.go); });

// Sondeo: solo la sección visible y solo con la ventana visible.
let pollTimer = null;
function schedule(now){
  clearTimeout(pollTimer);
  if (document.hidden) return;
  const run = async () => {
    try{
      if (page === 'overview') await refreshOverview();
      else if (page === 'processes' && !pPaused && !modalOpen) await refreshProcesses();
    }catch(e){}
    if (!document.hidden && (page === 'overview' || page === 'processes')) pollTimer = setTimeout(run, (CFG.refresh||3) * 1000);
  };
  if (now) run(); else pollTimer = setTimeout(run, (CFG.refresh||3) * 1000);
}
document.addEventListener('visibilitychange', () => schedule(true));

// ---------------------------------------------------------------- conexión
let elevatingNow = false;
(function alive(){
  let shown = null;
  const a = new EventSource('/api/alive?t=' + T);
  a.addEventListener('hi', () => {
    clearTimeout(shown);
    if ($('#overlay').classList.contains('on')){
      $('#overlay').classList.remove('on');
      loadStatus().then(() => { if (elevatingNow && STATUS.admin) toast('Ahora tienes permisos de administrador', 'ok'); elevatingNow = false; });
      schedule(true);
    }
  });
  a.onerror = () => {
    a.close();
    shown = setTimeout(() => {
      $('#overlayText').textContent = elevatingNow ? 'Reiniciando con permisos de administrador…' : 'Reconectando…';
      $('#overlaySub').textContent = elevatingNow ? 'Acepta el aviso de Windows si aparece.' : 'Si la aplicación se cerró, vuelve a abrirla.';
      $('#overlay').classList.add('on');
    }, elevatingNow ? 0 : 1500);
    setTimeout(alive, 1000);
  };
})();

$('#btnElevate').onclick = elevate;
async function elevate(){
  try{
    const r = await api('elevate', {});
    if (r.error) return toast(r.error, 'warn');
    if (r.already) return loadStatus();
    elevatingNow = true;
  }catch(e){ toast('No se pudo reiniciar como administrador', 'bad'); }
}

// ---------------------------------------------------------------- RESUMEN
const hist = {cpu:[], mem:[]};
function spark(canvas, data, color){
  const dpr = devicePixelRatio || 1, w = canvas.clientWidth, h = canvas.clientHeight;
  if (!w) return;
  if (canvas.width !== w*dpr){ canvas.width = w*dpr; canvas.height = h*dpr; }
  const c = canvas.getContext('2d'); c.setTransform(dpr,0,0,dpr,0,0); c.clearRect(0,0,w,h);
  const N = 60, step = w/(N-1), pts = data.slice(-N), off = N - pts.length;
  if (pts.length < 2) return;
  c.beginPath();
  pts.forEach((v,i) => { const x=(i+off)*step, y=h-2-(v/100)*(h-4); i?c.lineTo(x,y):c.moveTo(x,y); });
  c.strokeStyle = color; c.lineWidth = 1.6; c.lineJoin='round'; c.stroke();
  c.lineTo((N-1)*step, h); c.lineTo(off*step, h); c.closePath();
  const g = c.createLinearGradient(0,0,0,h); g.addColorStop(0, color+'40'); g.addColorStop(1, color+'00'); c.fillStyle = g; c.fill();
}
function initials(name){ const w = String(name||'?').replace(/[^\p{L}\p{N} ]/gu,' ').trim().split(/\s+/); return ((w[0]||'?')[0] + (w[1]?w[1][0]:'')).toUpperCase(); }
async function refreshOverview(){
  const s = await api('stats');
  hist.cpu.push(s.cpu); hist.mem.push(s.memLoad); if (hist.cpu.length>60){ hist.cpu.shift(); hist.mem.shift(); }
  $('#kCpu').innerHTML = `${Math.round(s.cpu)}<small>%</small>`;
  $('#kCpuSub').textContent = `${s.cores||'–'} procesadores lógicos`;
  const used = s.memTotal - s.memAvail;
  $('#kMem').innerHTML = `${fmtBytes(used,1).replace(/ (\w+)$/,'<small>$1</small>')}`;
  $('#kMemSub').textContent = `${fmtPct(s.memLoad)} de ${fmtBytes(s.memTotal,0)} · ${fmtBytes(s.memAvail,1)} libres`;
  $('#kProc').textContent = fmtN(s.processes);
  $('#kProcSub').textContent = `${fmtN(s.threads)} hilos`;
  const cu = s.commitTotal ? (1 - s.commitAvail/s.commitTotal)*100 : 0;
  $('#kCommit').style.width = cu.toFixed(1)+'%';
  $('#kCommit').parentElement.className = 'bar' + (cu>90?' bad':cu>80?' warn':'');
  $('#kCommitSub').textContent = `Memoria comprometida: ${fmtPct(cu)}`;
  $('#kUp').textContent = fmtUptime(s.uptime);
  $('#kUpHint').textContent = s.uptime > 7*86400 ? 'Recomendado reiniciar pronto' : ' ';
  spark($('#spCpu'), hist.cpu, '#3d8bfd');
  spark($('#spMem'), hist.mem, '#9384f7');
  const row = (t, val) => `<tr><td style="width:60%"><div class="pname"><div class="pi">${esc(initials(t.name))}</div><div class="tx"><b>${esc(t.name)}</b><small>${esc(t.exe)}${t.n>1?` · ${t.n} procesos`:''}</small></div></div></td><td class="r-right num" style="width:40%">${val}</td></tr>`;
  $('#topMem').innerHTML = s.topMem.map(t => row(t, fmtBytes(t.WS ?? t.ws, 1))).join('');
  $('#topCpu').innerHTML = s.topCPU.map(t => row(t, fmtPct(t.cpu, 1))).join('');
  $('#disks').innerHTML = (s.disks||[]).map(d => {
    const u = d.total ? (1 - d.free/d.total)*100 : 0;
    return `<div class="disk"><div class="top">${ic(d.kind==='removable'?'usb':'disk')}<b>${esc(d.path.replace(/\\$/,''))}</b><span class="faint">${esc(d.label)}</span><span>${fmtBytes(d.free,1)} libres de ${fmtBytes(d.total,0)}</span></div><div class="bar${u>95?' bad':u>85?' warn':''}"><i style="width:${u.toFixed(1)}%"></i></div></div>`;
  }).join('') || '<div class="faint">Sin discos.</div>';
  setPriv(s.admin);
}
function setPriv(admin){ if (admin !== STATUS.admin){ STATUS.admin = admin; loadStatus(); } }
$('#qaClean').onclick = () => cleanup();
$('#qaExplorer').onclick = () => explorerRestart();

// ---------------------------------------------------------------- PROCESOS
let groups = [], totals = {}, pFilter = 'all', pSort = {k:'ws', d:-1}, pOpen = null, pPaused = false;
const pSel = new Set();
const CAT = {
  bloat:['Innecesario','b-bloat'], app:['Aplicación','b-app'], background:['Segundo plano','b-background'],
  hardware:['Controlador','b-hardware'], security:['Seguridad','b-security'], system:['Sistema','b-system'],
  service:['Servicio','b-service'], self:['Esta aplicación','b-self'],
};
const catOrder = {bloat:0, app:1, background:2, hardware:3, security:4, service:5, system:6, self:7};

let pMouseDown = false;
$('#pBody').addEventListener('mousedown', () => { pMouseDown = true; });
document.addEventListener('mouseup', () => { setTimeout(() => { pMouseDown = false; }, 0); });
async function refreshProcesses(){
  const r = await api('processes');
  if (pMouseDown) return; // no redibujar a mitad de un clic
  groups = r.groups || []; totals = r.totals || {};
  setPriv(totals.admin);
  const keys = new Set(groups.map(g => g.key));
  for (const k of [...pSel]) if (!keys.has(k)) pSel.delete(k);
  renderProcesses();
}
function inFilter(g){
  if (pFilter === 'all') return true;
  if (pFilter === 'system') return ['system','service','security','self'].includes(g.cat);
  if (pFilter === 'bloat') return g.auto;
  return g.cat === pFilter;
}
function renderProcesses(){
  const q = $('#pFilter').value.trim().toLowerCase();
  const counts = {all:0,bloat:0,app:0,background:0,hardware:0,system:0};
  for (const g of groups){
    counts.all++;
    if (g.auto) counts.bloat++;
    if (counts[g.cat] !== undefined && g.cat !== 'bloat') counts[g.cat]++;
    if (['service','security','self'].includes(g.cat)) counts.system++;
  }
  $$('#pSeg button').forEach(b => b.querySelector('.n').textContent = counts[b.dataset.f] ?? '');
  let list = groups.filter(g => inFilter(g) && (!q || (g.name+' '+g.exe+' '+(g.company||'')+' '+(g.window||'')).toLowerCase().includes(q)));
  const k = pSort.k, d = pSort.d;
  list.sort((a,b) => {
    if (k==='ws'||k==='cpu') return (a[k]-b[k])*d;
    if (k==='cat') return ((catOrder[a.cat]-catOrder[b.cat]) || (b.ws-a.ws)) * d;
    return String(a[k]||'').localeCompare(String(b[k]||''),'es',{sensitivity:'base'})*d;
  });
  const autos = groups.filter(g => g.auto);
  const freeable = autos.reduce((s,g) => s+g.ws, 0);
  $('#cleanCount').textContent = autos.length;
  $('#psCpu').textContent = fmtPct(totals.cpu||0);
  $('#psMem').textContent = `${fmtBytes(totals.memUsed||0,1)} / ${fmtBytes(totals.memTotal||0,0)}`;
  $('#psN').textContent = fmtN(totals.processes||0);
  $('#psFree').textContent = autos.length ? `${fmtBytes(freeable,0)} (${autos.length})` : 'Nada que limpiar';

  const rows = [];
  for (const g of list){
    const [lbl, cls] = CAT[g.cat] || ['–',''];
    const open = pOpen === g.key;
    const sub = g.window ? g.window : g.exe + (g.count>1 ? ` · ${g.count} procesos` : '');
    rows.push(`<tr class="r${open?' open':''}" data-k="${esc(g.key)}">
      <td>${g.locked ? '' : `<input type="checkbox" class="check" data-sel="${esc(g.key)}" ${pSel.has(g.key)?'checked':''}>`}</td>
      <td><div class="pname"><div class="pi">${esc(initials(g.name))}</div><div class="tx"><b>${esc(g.name)}${g.count>1?` <span class="cnt">(${g.count})</span>`:''}</b><small>${esc(sub)}</small></div></div></td>
      <td><span class="badge ${cls}">${g.locked?ic('lock'):''}${lbl}</span></td>
      <td class="r-right num">${fmtBytes(g.ws,1)}</td>
      <td class="r-right num">${g.cpu>=0.05?fmtPct(g.cpu,1):'<span class="faint">0 %</span>'}</td>
      <td class="muted">${esc(g.company||'')}</td>
      <td class="r-right">${g.locked ? `<span class="lockic">${ic('lock')}Protegido</span>` : `<button class="btn sm" data-kill="${esc(g.key)}">Finalizar</button>`}</td></tr>`);
    if (open) rows.push(detailRow(g));
  }
  if (!list.length) rows.push(`<tr><td colspan="7" style="max-width:none"><div class="empty"><div class="ic">${ic('search')}</div><b>No hay procesos que mostrar</b>Prueba con otro filtro o búsqueda.</div></td></tr>`);
  $('#pBody').innerHTML = rows.join('');
  $$('#p-processes th.sort').forEach(th => th.classList.toggle('sorted', th.dataset.k === k));
  updateBulk(list);
}
function detailRow(g){
  const ex = (CFG.exclusions||[]).map(s=>s.toLowerCase()).includes(g.exe.toLowerCase());
  const al = (CFG.alwaysClose||[]).map(s=>s.toLowerCase()).includes(g.exe.toLowerCase());
  const pids = g.members.slice(0, 40).map(m => m.pid).join(', ') + (g.members.length>40 ? ` y ${g.members.length-40} más` : '');
  return `<tr class="det"><td colspan="7">
    <div class="detgrid">
      <div>Motivo</div><div>${esc(g.reason)}</div>
      <div>Ejecutable</div><div class="mono">${esc(g.path || g.exe)}</div>
      ${g.window?`<div>Ventana</div><div>${esc(g.window)}</div>`:''}
      <div>Procesos (PID)</div><div class="mono">${esc(pids)}</div>
      <div>Editor</div><div>${esc(g.company||'Desconocido')}</div>
    </div>
    <div class="detacts">
      ${g.locked?'':`<button class="btn sm danger" data-kill="${esc(g.key)}">${ic('x')}Finalizar${g.count>1?` los ${g.count} procesos`:''}</button>`}
      ${g.path?`<button class="btn sm" data-reveal="${esc(g.path)}">${ic('folder')}Abrir ubicación</button>`:''}
      ${g.locked?'':`<button class="btn sm" data-excl="${esc(g.exe)}">${ic('ban')}${ex?'Quitar de exclusiones':'Excluir de la limpieza'}</button>
      <button class="btn sm" data-always="${esc(g.exe)}">${ic('zap')}${al?'Quitar de «cerrar siempre»':'Cerrar siempre en la limpieza'}</button>`}
    </div></td></tr>`;
}
function updateBulk(list){
  const visibleSel = [...pSel];
  $('#pBulk').classList.toggle('on', visibleSel.length > 0);
  const mem = groups.filter(g => pSel.has(g.key)).reduce((s,g)=>s+g.ws,0);
  $('#pBulkText').innerHTML = `<b>${visibleSel.length}</b> seleccionados · ${fmtBytes(mem,0)}`;
  const selectable = list.filter(g => !g.locked);
  const all = $('#pAll');
  const n = selectable.filter(g => pSel.has(g.key)).length;
  all.checked = selectable.length > 0 && n === selectable.length;
  all.indeterminate = n > 0 && n < selectable.length;
}
$('#pBody').addEventListener('click', async e => {
  const t = e.target;
  if (t.matches('[data-sel]')){ t.checked ? pSel.add(t.dataset.sel) : pSel.delete(t.dataset.sel); renderProcesses(); return; }
  const kb = t.closest('[data-kill]'); if (kb){ const g = groups.find(x => x.key === kb.dataset.kill); if (g) killGroups([g]); return; }
  const rv = t.closest('[data-reveal]'); if (rv){ api('reveal', {path: rv.dataset.reveal}).catch(()=>toast('No se pudo abrir la ubicación','bad')); return; }
  const ex = t.closest('[data-excl]'); if (ex){ toggleList('exclusions', ex.dataset.excl); return; }
  const al = t.closest('[data-always]'); if (al){ toggleList('alwaysClose', al.dataset.always); return; }
  const tr = t.closest('tr.r'); if (tr){ pOpen = pOpen === tr.dataset.k ? null : tr.dataset.k; renderProcesses(); }
});
$('#pAll').onchange = e => {
  const q = $('#pFilter').value.trim().toLowerCase();
  const vis = groups.filter(g => !g.locked && inFilter(g) && (!q || (g.name+' '+g.exe+' '+(g.company||'')).toLowerCase().includes(q)));
  vis.forEach(g => e.target.checked ? pSel.add(g.key) : pSel.delete(g.key));
  renderProcesses();
};
$('#pBulkClear').onclick = () => { pSel.clear(); renderProcesses(); };
$('#pBulkKill').onclick = () => killGroups(groups.filter(g => pSel.has(g.key)));
$('#pFilter').addEventListener('input', renderProcesses);
$$('#pSeg button').forEach(b => b.onclick = () => { pFilter = b.dataset.f; $$('#pSeg button').forEach(x => x.classList.toggle('on', x===b)); renderProcesses(); });
$$('#p-processes th.sort').forEach(th => th.onclick = () => { const k = th.dataset.k; pSort = pSort.k===k ? {k, d:-pSort.d} : {k, d:(k==='ws'||k==='cpu')?-1:1}; renderProcesses(); });
$('#pPause').onclick = () => {
  pPaused = !pPaused;
  $('#pPause').innerHTML = pPaused ? ic('play') + '<span>Reanudar</span>' : ic('pause') + '<span>Pausar</span>';
  if (!pPaused) schedule(true);
};
$('#btnClean').onclick = () => cleanup();
$('#btnExplorer').onclick = () => explorerRestart();

async function toggleList(list, exe){
  const arr = CFG[list] || [];
  const i = arr.findIndex(x => x.toLowerCase() === exe.toLowerCase());
  if (i >= 0) arr.splice(i,1); else arr.push(exe);
  const other = list === 'exclusions' ? 'alwaysClose' : 'exclusions';
  if (i < 0) CFG[other] = (CFG[other]||[]).filter(x => x.toLowerCase() !== exe.toLowerCase());
  CFG[list] = arr;
  await saveConfig();
  toast(i >= 0 ? `${exe} quitado de la lista` : (list==='exclusions' ? `${exe} no se cerrará en la limpieza` : `${exe} se cerrará siempre en la limpieza`), 'ok');
  if (page === 'processes') refreshProcesses();
}

function summarizeKill(results, groupsKilled){
  const ok = results.filter(r => r.ok).length, fail = results.filter(r => !r.ok);
  const freed = groupsKilled.reduce((s,g) => s+g.ws, 0);
  if (!fail.length) toast(`Se han finalizado ${ok} ${ok===1?'proceso':'procesos'}`, 'ok', `Memoria liberada aprox.: ${fmtBytes(freed,0)}`);
  else {
    const admin = fail.some(f => /administrador/i.test(f.error||''));
    toast(`${ok} finalizados · ${fail.length} no se pudieron finalizar`, 'warn', admin && !STATUS.admin ? 'Algunos requieren permisos de administrador. Usa «Ejecutar como administrador».' : (fail[0].error||''));
  }
}

async function killGroups(list){
  list = list.filter(g => !g.locked);
  if (!list.length) return;
  if (CFG.confirmKill){
    const hasApp = list.some(g => g.cat === 'app');
    const body = `<div class="mlist">${list.map(g => `<label style="cursor:default"><div class="nm"><b>${esc(g.name)}</b><small>${esc(g.exe)}${g.count>1?` · ${g.count} procesos`:''}</small></div><span class="badge ${(CAT[g.cat]||[])[1]}">${(CAT[g.cat]||[])[0]}</span><span class="num muted" style="width:76px;text-align:right">${fmtBytes(g.ws,1)}</span></label>`).join('')}</div>
      ${hasApp?`<div class="alert warn" style="margin-top:12px">${ic('alert')}<div>Hay aplicaciones con ventana abierta. Se perderá cualquier cambio sin guardar.</div></div>`:''}`;
    const ok = await modal({title: list.length===1 ? `¿Finalizar ${list[0].name}?` : `¿Finalizar ${list.length} programas?`, text:'Los procesos se cerrarán inmediatamente.', body, icon:'x', tone:'danger', ok:'Finalizar', okClass:'danger'});
    if (!ok) return;
  }
  const targets = list.flatMap(g => g.members.map(m => ({pid:m.pid, ct:m.ct})));
  try{
    const r = await api('kill', {targets});
    summarizeKill(r.results, list);
  }catch(e){ toast('Error al finalizar: ' + e.message, 'bad'); }
  list.forEach(g => pSel.delete(g.key));
  if (page === 'processes') refreshProcesses(); else if (page === 'overview') refreshOverview();
}

async function cleanup(){
  let data;
  try{ data = await api('processes'); }catch(e){ return toast('No se pudo analizar los procesos', 'bad'); }
  const list = (data.groups||[]).filter(g => g.auto && !g.locked);
  if (!list.length){
    return modal({title:'El equipo ya está limpio', text:'No hay procesos innecesarios en segundo plano ahora mismo.', icon:'check', cancel:'', ok:'Entendido'});
  }
  const body = `<div class="mlist" id="clList">${list.map((g,i) => `<label><input type="checkbox" class="check" data-i="${i}" checked><div class="nm"><b>${esc(g.name)}</b><small>${esc(g.reason)}</small></div><span class="num muted" style="width:76px;text-align:right">${fmtBytes(g.ws,1)}</span></label>`).join('')}</div>`;
  const sumOf = m => list.filter((g,i) => m.querySelector(`[data-i="${i}"]`).checked);
  const res = await modal({
    title:'Limpieza recomendada', text:`Se han encontrado ${list.length} programas en segundo plano que no son necesarios. Desmarca los que quieras conservar.`,
    body, icon:'zap', ok:'Cerrar seleccionados', wide:true,
    footLeft:'<span id="clSum"></span>',
    onMount: m => {
      const upd = () => { const s = sumOf(m); m.querySelector('#clSum').textContent = `${s.length} seleccionados · ${fmtBytes(s.reduce((a,g)=>a+g.ws,0),0)} liberables`; };
      m.addEventListener('change', upd); upd();
    },
    onOk: m => sumOf(m),
  });
  if (!res || !res.length) return;
  const targets = res.flatMap(g => g.members.map(m => ({pid:m.pid, ct:m.ct})));
  try{ const r = await api('kill', {targets}); summarizeKill(r.results, res); }
  catch(e){ toast('Error durante la limpieza', 'bad'); }
  if (page === 'processes') refreshProcesses(); else if (page === 'overview') refreshOverview();
}

async function explorerRestart(){
  const ok = await modal({title:'¿Reiniciar el Explorador de Windows?', text:'La barra de tareas y el escritorio desaparecerán un momento y volverán a cargarse. Las ventanas de carpetas abiertas se cerrarán.', icon:'refresh', ok:'Reiniciar'});
  if (!ok) return;
  try{ await api('explorer/restart', {}); toast('Explorador reiniciado', 'ok'); }
  catch(e){ toast('No se pudo reiniciar el Explorador', 'bad', e.message); }
}

// ---------------------------------------------------------------- ELIMINACIÓN FORZADA
let uData = null;
async function pick(kind){
  try{
    const r = await api('pick?kind=' + kind);
    if (r.error) return toast(r.error, 'bad');
    if (r.path){ $('#uPath').value = r.path; analyze(); }
  }catch(e){ toast('No se pudo abrir el selector', 'bad'); }
}
$('#uPickDir').onclick = () => pick('folder');
$('#uPickFile').onclick = () => pick('file');
$('#uAnalyze').onclick = () => analyze();
$('#uReanalyze').onclick = () => analyze();
$('#uPath').addEventListener('keydown', e => { if (e.key === 'Enter') analyze(); });

function setBusy(btn, busy, label){
  if (busy){ btn.dataset.html = btn.innerHTML; btn.innerHTML = `<span class="spinner"></span>${label}`; btn.disabled = true; }
  else { btn.innerHTML = btn.dataset.html || btn.innerHTML; btn.disabled = false; }
}

async function analyze(){
  const path = $('#uPath').value.trim().replace(/^"|"$/g,'');
  if (!path){ toast('Indica una carpeta o un archivo', 'warn'); $('#uPath').focus(); return; }
  $('#uOutcome').innerHTML = '';
  const b = $('#uAnalyze'); setBusy(b, true, 'Analizando…');
  try{
    const a = await api('unlock/analyze', {path});
    if (a.error){ $('#uResult').style.display='none'; $('#uOutcome').innerHTML = `<div class="alert bad">${ic('alert')}<div><b>No se puede continuar</b>${esc(a.error)}</div></div>`; uData = null; return; }
    uData = a; renderAnalysis();
  }catch(e){ toast('Error en el análisis', 'bad', e.message); }
  finally{ setBusy(b, false); }
}
function renderAnalysis(){
  const a = uData;
  $('#uResult').style.display = '';
  $('#uSub').textContent = a.path;
  $('#uStats').innerHTML = [
    ['Tipo', a.isDir ? 'Carpeta' : 'Archivo'],
    ['Archivos', fmtN(a.files) + (a.partial ? '+' : '')],
    ['Subcarpetas', a.isDir ? fmtN(a.dirs) : '—'],
    ['Tamaño', fmtBytes(a.bytes, 1)],
    ['Procesos implicados', fmtN((a.procs||[]).length)],
  ].map(([k,v]) => `<div><small>${k}</small><b class="num">${v}</b></div>`).join('');
  const procs = a.procs || [];
  let html = '';
  if (!procs.length){
    html = `<div class="alert ok">${ic('check')}<div><b>No se ha detectado ningún proceso usando este contenido</b>Debería poder eliminarse directamente. Si aun así falla, la aplicación lo reintentará y te ofrecerá alternativas.</div></div>`;
  } else {
    html = `<div class="tablewrap" style="max-height:330px"><table class="tbl"><colgroup><col style="width:44px"><col style="width:34%"><col><col style="width:100px"></colgroup>
      <thead><tr><th><input type="checkbox" class="check" id="uAll" checked></th><th>Proceso</th><th>Motivo</th><th class="r-right">Memoria</th></tr></thead><tbody>
      ${procs.map((p,i) => `<tr><td>${p.locked?'':`<input type="checkbox" class="check" data-u="${i}" checked>`}</td>
        <td><div class="pname"><div class="pi">${esc(initials(p.name))}</div><div class="tx"><b>${esc(p.name)}</b><small>${esc(p.exe)} · PID ${p.pid}</small></div></div></td>
        <td style="white-space:normal;padding-top:6px;padding-bottom:6px"><div class="reasons">${p.reasons.map(r => `<span class="badge">${esc(r)}</span>`).join('')}${p.locked?`<span class="badge b-system">${ic('lock')}Protegido</span>`:''}</div>${p.note?`<div class="faint" style="font-size:12px;margin-top:4px">${esc(p.note)}</div>`:''}</td>
        <td class="r-right num">${fmtBytes(p.ws,1)}</td></tr>`).join('')}
      </tbody></table></div>`;
  }
  if (a.partial) html += `<div class="alert info" style="margin-top:12px">${ic('info')}<div>La carpeta contiene muchísimos archivos; el análisis de bloqueos se ha hecho sobre los primeros 60.000. La eliminación afectará a todo el contenido.</div></div>`;
  $('#uProcs').innerHTML = html;
  const all = $('#uAll');
  if (all) all.onchange = () => $$('[data-u]').forEach(c => c.checked = all.checked);
  $('#uDelete').querySelector('span').textContent = procs.some(p => !p.locked) ? 'Finalizar procesos y eliminar' : 'Eliminar';
}
$('#uDelete').onclick = async () => {
  if (!uData) return;
  const procs = uData.procs || [];
  const chosen = $$('[data-u]').filter(c => c.checked).map(c => procs[+c.dataset.u]);
  const recycle = $('#uRecycle').checked;
  const what = uData.isDir ? 'la carpeta y todo su contenido' : 'el archivo';
  const ok = await modal({
    title: recycle ? '¿Enviar a la Papelera?' : '¿Eliminar definitivamente?',
    text: recycle ? `Se enviará ${what} a la Papelera de reciclaje.` : `Se eliminará ${what}. <strong>Esta acción no se puede deshacer.</strong>`,
    body: `<div class="pathbox mono">${esc(uData.path)}</div>
      <div class="muted" style="font-size:12.5px;margin-top:8px">${fmtN(uData.files)} archivos · ${fmtBytes(uData.bytes,1)}${chosen.length?` · se finalizarán ${chosen.length} ${chosen.length===1?'proceso':'procesos'}: ${esc(chosen.map(p=>p.name).join(', '))}`:''}</div>`,
    icon:'trash', tone:'danger', ok: recycle ? 'Enviar a la Papelera' : 'Eliminar', okClass:'danger',
  });
  if (!ok) return;
  const b = $('#uDelete'); setBusy(b, true, 'Eliminando…');
  try{
    const r = await api('unlock/delete', {path: uData.path, targets: chosen.map(p => ({pid:p.pid, ct:p.ct})), recycle});
    renderOutcome(r);
  }catch(e){ toast('Error al eliminar', 'bad', e.message); }
  finally{ setBusy(b, false); }
};
function renderOutcome(r){
  const failedKills = (r.killed||[]).filter(k => !k.ok);
  let html = '';
  if (r.ok){
    $('#uResult').style.display = 'none';
    html = `<div class="alert ok">${ic('check')}<div><b>${r.recycled ? 'Enviado a la Papelera correctamente' : 'Eliminado correctamente'}</b><span class="mono">${esc(uData.path)}</span>${r.explorer?'<br>El Explorador de Windows se ha reiniciado.':''}</div></div>`;
    toast(r.recycled ? 'Enviado a la Papelera' : 'Eliminado correctamente', 'ok');
    uData = null;
  } else if (r.error && !r.remaining){
    html = `<div class="alert bad">${ic('alert')}<div><b>No se pudo eliminar</b>${esc(r.error)}</div></div>`;
  } else {
    const needAdmin = !STATUS.admin;
    html = `<div class="card"><div class="ch"><h3>3. Resultado</h3></div><div class="cb">
      <div class="alert warn">${ic('alert')}<div><b>Quedan ${fmtN(r.remaining)} ${r.remaining===1?'elemento bloqueado':'elementos bloqueados'}</b>El resto del contenido se ha eliminado. Lo que queda está en uso por un proceso del sistema o requiere más permisos.</div></div>
      ${r.sample?.length?`<div class="sample mono">${r.sample.map(s=>`<div title="${esc(s)}">‎${esc(s)}</div>`).join('')}</div>`:''}
      ${failedKills.length?`<p class="muted" style="font-size:12.5px">${failedKills.length} ${failedKills.length===1?'proceso no se pudo':'procesos no se pudieron'} finalizar: ${esc(failedKills[0].error||'')}</p>`:''}
      <div class="detacts" style="margin-top:14px">
        <button class="btn" id="oRetry">${ic('refresh')}Volver a analizar</button>
        ${needAdmin?`<button class="btn primary" id="oElev">${ic('shield')}Ejecutar como administrador</button>`:''}
        <button class="btn" id="oSched" ${needAdmin?'disabled title="Requiere permisos de administrador"':''}>${ic('clock')}Programar eliminación al reiniciar</button>
      </div></div></div>`;
  }
  $('#uOutcome').innerHTML = html;
  const rt = $('#oRetry'); if (rt) rt.onclick = analyze;
  const el = $('#oElev'); if (el) el.onclick = elevate;
  const sc = $('#oSched'); if (sc) sc.onclick = async () => {
    try{
      const x = await api('unlock/schedule', {path: uData.path});
      if (x.error) return toast(x.error, 'bad');
      $('#uOutcome').innerHTML = `<div class="alert ok">${ic('check')}<div><b>Eliminación programada</b>${fmtN(x.count)} elementos se eliminarán automáticamente la próxima vez que reinicies el equipo.</div></div>`;
      $('#uResult').style.display = 'none';
    }catch(e){ toast('No se pudo programar', 'bad', e.message); }
  };
}

// ---------------------------------------------------------------- BÚSQUEDA
let mode = 'contiene', type = 'all', drives = [], all = [], view = [];
let sortK = null, sortDir = 1, es = null, running = false, lastQuery = '', selIdx = -1, menuItem = null, dirty = false;
const ROW = 36;
const SKEY = 'vx_search';
function loadPrefs(){
  try{
    const p = JSON.parse(localStorage.getItem(SKEY) || '{}');
    if (p.mode) mode = p.mode; if (p.type) type = p.type;
    for (const id of ['optCase','optAcc','optHidden','optSys']) if (id in p) $('#'+id).checked = p[id];
    for (const id of ['optExt','optExcl','optLimit','optFolder']) if (p[id] != null) $('#'+id).value = p[id];
    return p;
  }catch(e){ return {}; }
}
function savePrefs(){
  try{
    const p = {mode, type, drives: drives.filter(d=>d.on).map(d=>d.path)};
    for (const id of ['optCase','optAcc','optHidden','optSys']) p[id] = $('#'+id).checked;
    for (const id of ['optExt','optExcl','optLimit','optFolder']) p[id] = $('#'+id).value;
    localStorage.setItem(SKEY, JSON.stringify(p));
  }catch(e){}
}
let prefs = loadPrefs();
function syncChips(){
  $$('#modes .chip').forEach(c => c.classList.toggle('on', c.dataset.m === mode));
  $$('#types .chip').forEach(c => c.classList.toggle('on', c.dataset.t === type));
}
syncChips();
['optCase','optAcc','optHidden','optSys','optExt','optExcl','optLimit','optFolder'].forEach(id => $('#'+id).addEventListener('change', savePrefs));
$('#modes').onclick = e => { const c = e.target.closest('.chip'); if (!c) return; mode = c.dataset.m; syncChips(); savePrefs(); $('#q').focus(); };
$('#types').onclick = e => { const c = e.target.closest('.chip'); if (!c) return; type = c.dataset.t; syncChips(); savePrefs(); };

async function loadDrives(){
  try{
    const j = await api('drives');
    drives = (j.drives||[]).map(d => ({...d, on: prefs.drives ? prefs.drives.includes(d.path) : d.selected}));
    if (!drives.some(d=>d.on) && drives.length) drives.forEach(d => d.on = d.selected);
    drives.home = j.home;
  }catch(e){ drives = []; }
  renderDrives();
}
function renderDrives(){
  const box = $('#drives'); box.innerHTML = '';
  for (const d of drives){
    const c = document.createElement('div');
    c.className = 'chip' + (d.on ? ' on' : '');
    c.innerHTML = `${ic(d.kind==='removable'?'usb':'disk')}${esc(d.path.replace(/\\$/,''))} <small>${esc(d.label)}</small>`;
    c.onclick = () => { d.on = !d.on; renderDrives(); savePrefs(); };
    box.appendChild(c);
  }
  const allC = document.createElement('div');
  allC.className = 'chip'; allC.textContent = 'Todas';
  allC.onclick = () => { const v = !drives.every(d=>d.on); drives.forEach(d=>d.on=v); renderDrives(); savePrefs(); };
  box.appendChild(allC);
  if (drives.home){
    const h = document.createElement('div');
    h.className = 'chip'; h.innerHTML = ic('folder') + 'Solo mi usuario'; h.title = drives.home;
    h.onclick = () => { $('#optFolder').value = drives.home; $('#adv').open = true; savePrefs(); toast('Se buscará solo en ' + drives.home, 'info'); };
    box.appendChild(h);
  }
}
function startSearch(){
  stopSearch(true);
  const q = $('#q').value.trim(), ext = $('#optExt').value.trim();
  if (!q && !ext){ toast('Escribe algo que buscar', 'warn'); $('#q').focus(); return; }
  const folder = $('#optFolder').value.trim().replace(/^"|"$/g,'');
  const roots = folder ? [folder] : drives.filter(d=>d.on).map(d=>d.path);
  if (!roots.length){ toast('Selecciona al menos una unidad', 'warn'); return; }
  const p = new URLSearchParams({t:T, q, mode, case:$('#optCase').checked?1:0, acc:$('#optAcc').checked?1:0,
    hidden:$('#optHidden').checked?1:0, skipsys:$('#optSys').checked?1:0, files:type!=='dirs'?1:0, dirs:type!=='files'?1:0,
    ext, excl:$('#optExcl').value, limit:$('#optLimit').value || 10000});
  roots.forEach(r => p.append('root', r));
  all = []; view = []; selIdx = -1; lastQuery = q;
  vrender(); running = true; setRunningUI(true); setStatus('run', 'Buscando…', '');
  es = new EventSource('/api/search?' + p);
  es.addEventListener('r', e => { for (const r of JSON.parse(e.data)) all.push(r); dirty = true; });
  es.addEventListener('p', e => { const s = JSON.parse(e.data); setStatus('run', `${fmtN(s.found)} encontrados · ${fmtN(s.dirs)} carpetas · ${fmtMs(s.ms)}`, s.cur || ''); });
  es.addEventListener('done', e => {
    const s = JSON.parse(e.data); finish();
    const base = `${fmtN(s.found)} encontrados · ${fmtN(s.dirs)} carpetas · ${fmtN(s.entries)} elementos revisados · ${fmtMs(s.ms)}`;
    if (s.reason === 'limite') setStatus('warn', base + ' · límite alcanzado', 'Aumenta el límite en Opciones avanzadas o concreta la búsqueda');
    else setStatus(s.found ? 'ok' : 'warn', s.found ? base : `Sin resultados · ${fmtN(s.dirs)} carpetas revisadas · ${fmtMs(s.ms)}`, s.denied ? `${fmtN(s.denied)} carpetas sin permiso de acceso` : '');
  });
  es.addEventListener('err', e => { finish(); setStatus('bad', JSON.parse(e.data), ''); });
  es.onerror = () => { if (running){ finish(); setStatus('bad', 'Se perdió la conexión con el buscador', ''); } };
}
function finish(){ running = false; setRunningUI(false); if (es){ es.close(); es = null; } dirty = true; }
function stopSearch(silent){ if (!running) return; finish(); if (!silent) setStatus('warn', `Detenido · ${fmtN(all.length)} encontrados`, ''); }
function setRunningUI(on){ const b = $('#go'); b.textContent = on ? 'Detener' : 'Buscar'; b.className = 'btn ' + (on ? 'danger' : 'primary'); }
$('#go').onclick = () => running ? stopSearch() : startSearch();
$('#q').addEventListener('keydown', e => { if (e.key === 'Enter') startSearch(); });
function setStatus(kind, text, cur){
  $('#sDot').className = 'dot ' + kind; $('#stText').textContent = text;
  const isPath = /^([a-zA-Z]:\\|\\\\)/.test(cur || '');
  $('#stCur').style.direction = isPath ? 'rtl' : 'ltr';
  $('#stCur').textContent = cur ? (isPath ? '‎' : '') + cur : ''; $('#stCur').title = cur || '';
}
function fold(s){ s = $('#optCase').checked ? s : s.toLowerCase(); return $('#optAcc').checked ? s.normalize('NFD').replace(/[̀-ͯ]/g,'') : s; }
function rebuild(){
  const f = fold($('#flt').value.trim()); const idx = [];
  for (let i=0;i<all.length;i++){ if (f && !fold(all[i].p).includes(f)) continue; idx.push(i); }
  if (sortK){ const k = sortK, d = sortDir; idx.sort((a,b) => { const A=all[a], B=all[b]; if (k==='s'||k==='m') return (A[k]-B[k])*d; return A[k].localeCompare(B[k],'es',{numeric:true,sensitivity:'base'})*d; }); }
  view = idx;
}
$('#flt').addEventListener('input', () => { dirty = true; });
$('#vhead').onclick = e => {
  const k = e.target.dataset.k; if (!k) return;
  if (sortK === k) sortDir = -sortDir; else { sortK = k; sortDir = (k==='s'||k==='m') ? -1 : 1; }
  $$('#vhead div').forEach(d => { d.classList.toggle('sorted', d.dataset.k === sortK); const base = d.textContent.replace(/ [▲▼]$/,''); d.textContent = d.dataset.k === sortK ? base + (sortDir>0?' ▲':' ▼') : base; });
  dirty = true;
};
const vlist = $('#vlist'), vrowsEl = $('#vrows'), vspacer = $('#vspacer');
function vrender(){
  vspacer.style.height = (view.length * ROW) + 'px';
  $('#vempty').style.display = view.length ? 'none' : 'grid';
  if (!view.length && !running && lastQuery !== '') $('#vempty').innerHTML = `<div class="empty"><div class="ic">${ic('search')}</div><b>Sin resultados</b>Prueba otro modo (por ejemplo «Contiene») o revisa las unidades seleccionadas.</div>`;
  vpaint();
}
function vpaint(){
  const top = vlist.scrollTop, h = vlist.clientHeight;
  const start = Math.max(0, Math.floor(top/ROW) - 8), end = Math.min(view.length, Math.ceil((top+h)/ROW) + 8);
  vrowsEl.style.top = (start*ROW) + 'px';
  const parts = [];
  for (let i=start;i<end;i++){
    const r = all[view[i]];
    parts.push(`<div class="vr${i===selIdx?' sel':''}" data-i="${i}">
      <div class="ft">${r.d ? ic('folder') : `<span>${esc(extOf(r.n))}</span>`}</div>
      <div class="vnm" title="${esc(r.n)}">${hl(r.n)}</div>
      <div class="vpth" title="${esc(r.p)}">‎${esc(parentOf(r.p))}</div>
      <div class="vsz">${r.d ? '' : fmtBytes(r.s)}</div>
      <div class="vdt">${r.m ? fmtDate(r.m) : ''}</div>
      <div><div class="vacts"><div class="ib" data-a="open" title="Abrir">${ic('external')}</div><div class="ib" data-a="reveal" title="Mostrar en su carpeta">${ic('folder')}</div><div class="ib" data-a="copy" title="Copiar ruta">${ic('copy')}</div></div></div></div>`);
  }
  vrowsEl.innerHTML = parts.join('');
}
vlist.addEventListener('scroll', () => requestAnimationFrame(vpaint));
window.addEventListener('resize', () => { if (page==='search') vpaint(); });
function ensureVisible(i){ const y = i*ROW; if (y < vlist.scrollTop) vlist.scrollTop = y; else if (y + ROW > vlist.scrollTop + vlist.clientHeight) vlist.scrollTop = y + ROW - vlist.clientHeight; }
setInterval(() => { if (dirty){ dirty = false; rebuild(); vrender(); } }, 250);
vrowsEl.addEventListener('click', e => { const tr = e.target.closest('.vr'); if (!tr) return; selIdx = +tr.dataset.i; const a = e.target.closest('.ib'); if (a) act(a.dataset.a, all[view[selIdx]]); vpaint(); });
vrowsEl.addEventListener('dblclick', e => { const tr = e.target.closest('.vr'); if (!tr || e.target.closest('.ib')) return; act('open', all[view[+tr.dataset.i]]); });
vrowsEl.addEventListener('contextmenu', e => {
  const tr = e.target.closest('.vr'); if (!tr) return; e.preventDefault();
  selIdx = +tr.dataset.i; vpaint(); menuItem = all[view[selIdx]];
  const m = $('#ctx'); m.style.display = 'block';
  m.style.left = Math.min(e.clientX, innerWidth - m.offsetWidth - 8) + 'px';
  m.style.top = Math.min(e.clientY, innerHeight - m.offsetHeight - 8) + 'px';
});
const hideMenu = () => { $('#ctx').style.display = 'none'; };
document.addEventListener('click', e => { if (!e.target.closest('#ctx')) hideMenu(); });
document.addEventListener('contextmenu', e => { if (!e.target.closest('.vr') && !e.target.closest('input,textarea,.sel,.doc,.mono')) e.preventDefault(); });
$('#ctx').onclick = e => { const d = e.target.closest('[data-a]'); if (d && menuItem) act(d.dataset.a, menuItem); hideMenu(); };
async function act(a, r){
  if (!r) return;
  if (a === 'copy') return copyText(r.p, 'Ruta copiada');
  if (a === 'copyname') return copyText(r.n, 'Nombre copiado');
  if (a === 'copydir') return copyText(parentOf(r.p), 'Ruta de la carpeta copiada');
  if (a === 'unlock'){ $('#uPath').value = r.p; go('unlock'); analyze(); return; }
  try{ await api(a === 'open' ? 'open' : 'reveal', {path:r.p}); }catch(e){ toast('No se pudo abrir', 'bad', e.message); }
}
$('#csv').onclick = () => {
  if (!view.length) return toast('No hay resultados que exportar', 'warn');
  const q = v => '"' + String(v).replace(/"/g,'""') + '"';
  const lines = ['Nombre;Tipo;Ruta;Tamaño (bytes);Modificado'];
  for (const i of view){ const r = all[i]; lines.push([q(r.n), r.d?'Carpeta':'Archivo', q(r.p), r.d?'':r.s, r.m?q(fmtDate(r.m)):''].join(';')); }
  const blob = new Blob(['﻿' + lines.join('\r\n')], {type:'text/csv;charset=utf-8'});
  const a = document.createElement('a'); a.href = URL.createObjectURL(blob);
  a.download = 'busqueda_' + (lastQuery || 'resultados').replace(/[\\/:*?"<>|]+/g,'_') + '.csv';
  a.click(); setTimeout(() => URL.revokeObjectURL(a.href), 2000);
  toast('Resultados exportados', 'ok');
};
function parentOf(p){ const i = p.lastIndexOf('\\'); return i > 0 ? p.slice(0, i) + (p[i-1]===':'?'\\':'') : p; }
function extOf(n){ const i = n.lastIndexOf('.'); const e = i > 0 ? n.slice(i+1) : ''; return (e && e.length <= 4 ? e : (e ? e.slice(0,4) : '—')).toUpperCase(); }
function hl(name){
  const q = lastQuery;
  if (!q || mode === 'regex' || mode === 'comodin') return esc(name);
  const chars = [...name], fn = chars.map(c => fold(c)[0] || c).join('');
  const words = mode === 'palabras' ? fold(q).split(/\s+/).filter(Boolean) : [fold(q)];
  const marks = new Array(chars.length).fill(false);
  for (const w of words){
    const at = mode==='empieza' ? (fn.startsWith(w)?0:-1) : mode==='termina' ? fn.lastIndexOf(w) : fn.indexOf(w);
    if (at >= 0) for (let k=at;k<at+[...w].length && k<chars.length;k++) marks[k] = true;
  }
  let out = '', open = false;
  chars.forEach((c,i) => { if (marks[i] && !open){ out += '<mark>'; open = true; } if (!marks[i] && open){ out += '</mark>'; open = false; } out += esc(c); });
  return out + (open ? '</mark>' : '');
}

// ---------------------------------------------------------------- GUÍA
$('#toc').onclick = e => { const a = e.target.closest('a'); if (!a) return; document.getElementById(a.dataset.s).scrollIntoView({behavior:'smooth', block:'start'}); };
$('#guideBody').addEventListener('scroll', () => {
  const body = $('#guideBody'), top = body.getBoundingClientRect().top + 30;
  let cur = null;
  $$('.doc section').forEach(s => { if (s.getBoundingClientRect().top <= top + 40) cur = s.id; });
  if (!cur) cur = 'g-intro';
  $$('#toc a').forEach(a => a.classList.toggle('on', a.dataset.s === cur));
});
$('#toc a').classList.add('on');

// ---------------------------------------------------------------- AJUSTES
function applySettingsUI(){
  $('#sConfirm').checked = !!CFG.confirmKill;
  $('#sHeavy').checked = !!CFG.includeHeavy;
  $('#sHeavyMB').value = CFG.heavyMB || 400;
  $('#sRefresh').value = String(CFG.refresh || 3);
  const tags = (list, el) => { el.innerHTML = (CFG[list]||[]).length ? CFG[list].map((x,i) => `<span class="tag">${esc(x)}<button data-rm="${list}:${i}" title="Quitar">${ic('x')}</button></span>`).join('') : '<span class="faint" style="font-size:12.5px">Ninguno</span>'; };
  tags('exclusions', $('#tagsExcl')); tags('alwaysClose', $('#tagsAlways'));
}
$('#sConfirm').onchange = e => { CFG.confirmKill = e.target.checked; saveConfig(); };
$('#sHeavy').onchange = e => { CFG.includeHeavy = e.target.checked; saveConfig(); };
$('#sHeavyMB').onchange = e => { CFG.heavyMB = Math.max(50, +e.target.value || 400); saveConfig(); };
$('#sRefresh').onchange = e => { CFG.refresh = +e.target.value; saveConfig(); };
$('#p-settings').addEventListener('click', async e => {
  const b = e.target.closest('[data-rm]'); if (!b) return;
  const [list, i] = b.dataset.rm.split(':'); CFG[list].splice(+i, 1); await saveConfig(); applySettingsUI();
});
function addTo(list, input){
  let v = input.value.trim(); if (!v) return;
  if (!/\.exe$/i.test(v)) v += '.exe';
  if (!(CFG[list]||[]).some(x => x.toLowerCase() === v.toLowerCase())) (CFG[list] = CFG[list]||[]).push(v);
  const other = list === 'exclusions' ? 'alwaysClose' : 'exclusions';
  CFG[other] = (CFG[other]||[]).filter(x => x.toLowerCase() !== v.toLowerCase());
  input.value = ''; saveConfig().then(applySettingsUI);
}
$('#addExcl').onclick = () => addTo('exclusions', $('#inExcl'));
$('#addAlways').onclick = () => addTo('alwaysClose', $('#inAlways'));
$('#inExcl').addEventListener('keydown', e => { if (e.key==='Enter') addTo('exclusions', $('#inExcl')); });
$('#inAlways').addEventListener('keydown', e => { if (e.key==='Enter') addTo('alwaysClose', $('#inAlways')); });
$('#sResetSearch').onclick = () => {
  try{ localStorage.removeItem(SKEY); }catch(e){}
  mode = 'contiene'; type = 'all'; prefs = {};
  $('#optCase').checked = false; $('#optAcc').checked = true; $('#optHidden').checked = true; $('#optSys').checked = false;
  $('#optExt').value = ''; $('#optExcl').value = ''; $('#optLimit').value = 10000; $('#optFolder').value = '';
  syncChips(); drives.forEach(d => d.on = d.selected); renderDrives();
  toast('Opciones de búsqueda restablecidas', 'ok');
};

// ---------------------------------------------------------------- teclado
document.addEventListener('keydown', e => {
  if (modalOpen) return;
  if (e.ctrlKey && /^[1-4]$/.test(e.key)){ e.preventDefault(); go(['overview','processes','unlock','search'][+e.key-1]); return; }
  if (e.ctrlKey && e.key.toLowerCase() === 'f'){
    e.preventDefault();
    const target = page === 'processes' ? '#pFilter' : page === 'unlock' ? '#uPath' : page === 'search' ? '#q' : null;
    if (!target){ go('search'); return; }
    $(target).focus(); $(target).select(); return;
  }
  if (e.key === 'F5'){ e.preventDefault(); if (page==='unlock' && uData) analyze(); else schedule(true); return; }
  if (e.key === 'Escape'){ if ($('#ctx').style.display === 'block') hideMenu(); else if (page==='search') stopSearch(); }
  if (page !== 'search' || document.activeElement.tagName === 'INPUT') return;
  if (e.key === 'ArrowDown' || e.key === 'ArrowUp'){
    e.preventDefault();
    selIdx = Math.max(0, Math.min(view.length-1, selIdx + (e.key==='ArrowDown'?1:-1)));
    ensureVisible(selIdx); vpaint();
  }
  if (e.key === 'Enter' && selIdx >= 0) act('open', all[view[selIdx]]);
});

// ---------------------------------------------------------------- inicio
loadStatus();
loadConfig().then(() => schedule(true));
loadDrives();
