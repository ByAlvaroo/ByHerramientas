'use strict';
/* Limpiador, Espacio en disco y Duplicados */

// ================================================================ LIMPIADOR
let CL = null, clSel = null, clBusy = false;
PAGES.cleaner = { async enter(){ if (!CL && !clBusy) await cleanerScan(); } };
async function cleanerScan(){
  clBusy = true;
  $('#clSummary').innerHTML = '';
  $('#clList').innerHTML = `<div class="card"><div class="empty"><div class="ic"><span class="spinner" style="border-color:rgba(255,255,255,.2);border-top-color:var(--accent)"></span></div><b>Analizando…</b>Calculando cuánto ocupa cada categoría.</div></div>`;
  try{ CL = await api('cleaner/scan'); }catch(e){ $('#clList').innerHTML = '<div class="alert bad">No se pudo analizar.</div>'; clBusy = false; return; }
  clBusy = false;
  if (!clSel){ clSel = new Set(CL.cats.filter(c => c.default && (!c.admin || CL.admin)).map(c => c.id)); }
  renderCleaner();
}
function renderCleaner(){
  const groups = {};
  CL.cats.forEach(c => { if (c.id !== 'recycle' && !c.paths?.length && !c.size) return; (groups[c.group] = groups[c.group] || []).push(c); });
  $('#clList').innerHTML = Object.entries(groups).map(([g, cats]) => `<div class="card clgroup"><div class="ch"><h3>${esc(g)}</h3><span class="sub">${fmtBytes(cats.reduce((s,c) => s+c.size, 0),1)}</span></div>
    ${cats.map(c => { const dis = c.admin && !CL.admin; return `<label class="clrow" style="cursor:${dis?'default':'pointer'}"><input type="checkbox" class="check" data-cl="${c.id}" ${clSel.has(c.id)&&!dis?'checked':''} ${dis?'disabled':''}>
      <div class="st"><b>${esc(c.name)}${dis ? '<span class="badge b-bloat">Requiere administrador</span>' : ''}${c.running ? `<span class="badge b-app">${esc(c.running.replace('.exe',''))} está abierto</span>` : ''}</b><span>${esc(c.desc)}${c.running ? ' Ciérralo para poder limpiarlo por completo.' : ''}</span></div>
      <div class="sz num">${c.size ? fmtBytes(c.size,1) : '<span class="faint">0 B</span>'}<small>${c.files ? fmtN(c.files) + (c.id==='recycle'?' elementos':' archivos') : ''}</small></div></label>`; }).join('')}</div>`).join('');
  updateCleanBtn();
}
function updateCleanBtn(){
  const sel = CL.cats.filter(c => clSel.has(c.id) && !(c.admin && !CL.admin));
  const total = sel.reduce((s,c) => s+c.size, 0);
  $('#clRun').querySelector('span').textContent = `Limpiar ${fmtBytes(total,1)}`;
  $('#clRun').disabled = !sel.length || !total;
}
$('#clList').addEventListener('change', e => { const c = e.target.dataset.cl; if (!c) return; e.target.checked ? clSel.add(c) : clSel.delete(c); updateCleanBtn(); });
$('#clScan').onclick = () => cleanerScan();
$('#clRun').onclick = async () => {
  const ids = [...clSel].filter(id => { const c = CL.cats.find(x => x.id === id); return c && !(c.admin && !CL.admin) && c.size; });
  if (!ids.length) return;
  if (ids.includes('recycle')){
    const ok = await modal({title:'¿Vaciar la Papelera?', text:'Lo que hay en la Papelera se eliminará definitivamente.', icon:'trash', tone:'danger', ok:'Continuar', okClass:'danger'});
    if (!ok) return;
  }
  const b = $('#clRun'); setBusy(b, true, 'Limpiando…');
  try{
    const r = await api('cleaner/run', {ids});
    const freed = r.results.reduce((s,x) => s + x.freed, 0), del = r.results.reduce((s,x) => s + x.deleted, 0), skip = r.results.reduce((s,x) => s + x.skipped, 0);
    $('#clSummary').innerHTML = `<div class="card bigsum"><div class="ti" style="width:44px;height:44px;border-radius:11px;display:grid;place-items:center;background:var(--ok-w);color:var(--ok)">${ic('check')}</div><div><div class="n num">${fmtBytes(freed,1)}</div><div class="t">liberados · ${fmtN(del)} archivos eliminados${skip ? ` · ${fmtN(skip)} omitidos porque estaban en uso` : ''}</div></div></div>`;
    toast(`Se han liberado ${fmtBytes(freed,1)}`, 'ok');
  }catch(e){ toast('Error durante la limpieza', 'bad'); }
  finally{ setBusy(b, false); }
  const keep = $('#clSummary').innerHTML;
  await cleanerScan();
  $('#clSummary').innerHTML = keep;
};

// ================================================================ ESPACIO EN DISCO
let spRoot = null, spEs = null, spData = null, spTab = 'list', spDrives = [];
const spColors = ['#3d7cf5','#7a5cf0','#2aa876','#d9922e','#d2545b','#2b9bb8','#a35cc7','#5f8a2d','#c46b3a','#4a6fd1'];
PAGES.space = {
  async enter(){
    if (spDrives.length) return;
    try{ const j = await api('drives'); spDrives = (j.drives||[]).filter(d => d.kind === 'fixed' || d.kind === 'removable'); }catch(e){}
    if (!spRoot && spDrives[0]) spRoot = spDrives[0].path;
    renderSpDrives();
  },
};
function renderSpDrives(){
  $('#spDrives').innerHTML = spDrives.map(d => `<div class="chip${spRoot===d.path?' on':''}" data-p="${esc(d.path)}">${ic(d.kind==='removable'?'usb':'disk')}${esc(d.path.replace(/\\$/,''))} <small>${esc(d.label)} · ${fmtBytes(d.total-d.free,0)} / ${fmtBytes(d.total,0)}</small></div>`).join('')
    + (spRoot && !spDrives.some(d => d.path === spRoot) ? `<div class="chip on">${ic('folder')}${esc(spRoot)}</div>` : '');
}
$('#spDrives').onclick = e => { const c = e.target.closest('[data-p]'); if (!c) return; spRoot = c.dataset.p; renderSpDrives(); };
$('#spPick').onclick = async () => { const r = await api('pick?kind=folder'); if (r.path){ spRoot = r.path; renderSpDrives(); spScan(); } };
$('#spGo').onclick = () => spScan();
function spScan(){
  if (!spRoot) return toast('Elige una unidad o carpeta', 'warn');
  if (spEs){ spEs.close(); spEs = null; }
  $('#spResult').style.display = 'none';
  $('#spDot').className = 'dot run'; $('#spText').textContent = 'Analizando…'; $('#spCur').textContent = '';
  $('#spGo').innerHTML = ic('x') + 'Detener'; $('#spGo').className = 'btn danger sm';
  spEs = new EventSource('/api/space/scan?t=' + T + '&root=' + encodeURIComponent(spRoot));
  spEs.addEventListener('p', e => { const s = JSON.parse(e.data); $('#spText').textContent = `${fmtN(s.files)} archivos · ${fmtN(s.dirs)} carpetas · ${fmtMs(s.ms)}`; $('#spCur').textContent = s.cur; });
  spEs.addEventListener('done', e => {
    const s = JSON.parse(e.data); spStop();
    $('#spDot').className = 'dot ok';
    $('#spText').textContent = `${fmtBytes(s.size,1)} en ${fmtN(s.files)} archivos y ${fmtN(s.dirs)} carpetas · ${fmtMs(s.ms)}`;
    $('#spCur').textContent = s.total ? `Unidad: ${fmtBytes(s.total - s.free,1)} usados de ${fmtBytes(s.total,0)} · ${fmtBytes(s.free,1)} libres` : '';
    spOpen(spRoot);
  });
  spEs.addEventListener('err', e => { spStop(); $('#spDot').className = 'dot bad'; $('#spText').textContent = JSON.parse(e.data); });
  spEs.onerror = () => { if (spEs){ spStop(); $('#spDot').className = 'dot warn'; $('#spText').textContent = 'Análisis detenido'; } };
}
function spStop(){ if (spEs){ spEs.close(); spEs = null; } $('#spGo').innerHTML = ic('scan') + 'Analizar'; $('#spGo').className = 'btn primary sm'; $('#spGo').onclick = () => spScan(); }
$('#spGo').addEventListener('click', e => { if (spEs){ e.stopImmediatePropagation(); spStop(); $('#spDot').className='dot warn'; $('#spText').textContent='Análisis detenido'; } }, true);
async function spOpen(path){
  const d = await api('space/node?path=' + encodeURIComponent(path));
  if (d.error) return toast(d.error, 'warn');
  if (d.types) spData = {...(spData||{}), types: d.types, big: d.big, free: d.free, total: d.total, rootSize: d.size};
  spData = {...spData, node: d};
  $('#spResult').style.display = 'flex';
  $('#spCrumbs').innerHTML = d.crumbs.map((c,i) => `${i?ic('chev'):''}<a data-p="${esc(c.path)}">${esc(c.name.replace(/\\$/,''))}</a>`).join('');
  spRenderMap(); spRenderSide();
}
$('#spCrumbs').onclick = e => { const a = e.target.closest('[data-p]'); if (a) spOpen(a.dataset.p); };

// treemap «squarified»
function squarify(items, x, y, w, h){
  const out = [];
  const total = items.reduce((s,i) => s + i.v, 0); if (!total || w <= 0 || h <= 0) return out;
  const scale = w*h/total;
  let rest = items.map(i => ({...i, a: i.v*scale}));
  while (rest.length){
    const short = Math.min(w, h); let row = [], best = Infinity;
    for (const it of rest){
      const r = [...row, it], s = r.reduce((t,i) => t+i.a, 0), mx = Math.max(...r.map(i=>i.a)), mn = Math.min(...r.map(i=>i.a));
      const worst = Math.max(short*short*mx/(s*s), (s*s)/(short*short*mn));
      if (worst > best) break; best = worst; row = r;
    }
    const s = row.reduce((t,i) => t+i.a, 0);
    if (w >= h){ const cw = s/h; let cy = y; row.forEach(i => { const ch = i.a/cw; out.push({...i, x, y:cy, w:cw, h:ch}); cy += ch; }); x += cw; w -= cw; }
    else { const ch = s/w; let cx = x; row.forEach(i => { const cw = i.a/ch; out.push({...i, x:cx, y, w:cw, h:ch}); cx += cw; }); y += ch; h -= ch; }
    rest = rest.slice(row.length);
  }
  return out;
}
function spRenderMap(){
  const box = $('#spMap'), W = box.clientWidth, H = box.clientHeight, d = spData.node;
  const items = d.items.filter(i => i.size > 0).slice(0, 60).map((i,k) => ({v: i.size, i, k}));
  const tiles = squarify(items, 0, 0, W, H);
  box.innerHTML = tiles.map(t => {
    const it = t.i, col = it.dir ? spColors[t.k % spColors.length] : '#3a414d';
    const show = t.w > 60 && t.h > 30;
    return `<div class="tile${it.dir?'':' file'}" data-p="${esc(it.path||'')}" data-dir="${it.dir?1:0}" style="left:${t.x}px;top:${t.y}px;width:${t.w}px;height:${t.h}px;background:${col}" title="${esc(it.name)} — ${fmtBytes(it.size,1)}">${show ? `<b>${esc(it.name)}</b><span>${fmtBytes(it.size,1)}</span>` : ''}</div>`;
  }).join('') || '<div class="empty">Carpeta vacía</div>';
}
window.addEventListener('resize', () => { if (page === 'space' && spData?.node) spRenderMap(); });
$('#spMap').onclick = e => { const t = e.target.closest('.tile'); if (t && t.dataset.dir === '1') spOpen(t.dataset.p); };
$$('#spTabs button').forEach(b => b.onclick = () => { spTab = b.dataset.t; $$('#spTabs button').forEach(x => x.classList.toggle('on', x===b)); spRenderSide(); });
function spRenderSide(){
  const d = spData.node, el = $('#spList');
  if (spTab === 'list'){
    const max = d.size || 1;
    el.innerHTML = `<table class="tbl"><colgroup><col><col style="width:110px"><col style="width:84px"></colgroup><tbody>${d.items.slice(0, 400).map(it => `<tr class="${it.dir?'r':''}" data-p="${esc(it.path||'')}" data-dir="${it.dir?1:0}">
      <td><div class="pname"><div class="pi">${it.dir ? ic('folder') : esc(extOf(it.name))}</div><div class="tx"><b>${esc(it.name)}</b><div class="pbar"><i style="width:${(it.size/max*100).toFixed(1)}%"></i></div></div></div></td>
      <td class="r-right num">${fmtBytes(it.size,1)}<div class="faint" style="font-size:11px;margin-top:-3px">${(it.size/max*100).toFixed(1).replace('.',',')} %</div></td>
      <td class="r-right">${it.path ? `<span class="ib" style="display:inline-grid" data-act="reveal" title="Mostrar en el Explorador">${ic('folder')}</span> <span class="ib" style="display:inline-grid" data-act="del" title="Enviar a la Papelera">${ic('trash')}</span>` : ''}</td></tr>`).join('')}</tbody></table>`;
  } else if (spTab === 'big'){
    el.innerHTML = `<table class="tbl"><colgroup><col><col style="width:100px"><col style="width:84px"></colgroup><tbody>${(spData.big||[]).map(f => `<tr data-p="${esc(f.p)}" data-dir="0"><td><div class="pname"><div class="pi">${esc(extOf(f.p.split('\\').pop()))}</div><div class="tx"><b>${esc(f.p.split('\\').pop())}</b><small style="direction:rtl;text-align:left">\u200E${esc(parentOf(f.p))}</small></div></div></td><td class="r-right num">${fmtBytes(f.s,1)}</td>
      <td class="r-right"><span class="ib" style="display:inline-grid" data-act="reveal">${ic('folder')}</span> <span class="ib" style="display:inline-grid" data-act="del">${ic('trash')}</span></td></tr>`).join('')}</tbody></table>`;
  } else {
    const tot = (spData.types||[]).reduce((s,t) => s+t.size, 0) || 1;
    el.innerHTML = `<div style="padding:4px 0">${(spData.types||[]).map((t,i) => `<div class="typerow"><span class="sw" style="background:${spColors[i%spColors.length]}"></span><b>${esc(t.name)}</b><span class="num muted">${fmtBytes(t.size,1)}</span><span class="num faint" style="width:56px;text-align:right">${(t.size/tot*100).toFixed(1).replace('.',',')} %</span></div>`).join('')}</div>`;
  }
}
$('#spList').addEventListener('click', async e => {
  const tr = e.target.closest('tr'); if (!tr) return;
  const act = e.target.closest('[data-act]')?.dataset.act, p = tr.dataset.p;
  if (act === 'reveal') return api('reveal', {path:p}).catch(() => toast('Ya no existe', 'warn'));
  if (act === 'del'){
    const ok = await modal({title:'¿Enviar a la Papelera?', body:`<div class="pathbox mono">${esc(p)}</div>`, icon:'trash', tone:'danger', ok:'Enviar a la Papelera', okClass:'danger'});
    if (!ok) return;
    const r = await api('files/delete', {paths:[p], recycle:true});
    if (r.ok){ toast('Enviado a la Papelera', 'ok'); if (spTab === 'big') spData.big = spData.big.filter(f => f.p !== p); spOpen(spData.node.path); }
    else toast('No se pudo eliminar', 'bad', 'Puede estar en uso. Prueba con Eliminación forzada.');
    return;
  }
  if (tr.dataset.dir === '1') spOpen(p);
});

// ================================================================ DUPLICADOS
let dpRoots = [], dpEs = null, dpRes = null, dpExt = '';
PAGES.dupes = { enter(){ if (!dpRoots.length && STATUS.user){ /* sin carpetas por defecto */ } renderDpRoots(); } };
function renderDpRoots(){
  $('#dpRoots').innerHTML = dpRoots.length ? dpRoots.map((r,i) => `<span class="tag">${ic('folder')}${esc(r)}<button data-i="${i}">${ic('x')}</button></span>`).join('') : '<span class="faint" style="font-size:12.5px">Añade las carpetas donde buscar (por ejemplo, Imágenes o Descargas).</span>';
}
$('#dpRoots').onclick = e => { const b = e.target.closest('[data-i]'); if (b){ dpRoots.splice(+b.dataset.i, 1); renderDpRoots(); } };
$('#dpAdd').onclick = async () => { const r = await api('pick?kind=folder'); if (r.path && !dpRoots.includes(r.path)){ dpRoots.push(r.path); renderDpRoots(); } };
$('#dpHome').onclick = async () => { const j = await api('drives'); if (j.home && !dpRoots.includes(j.home)){ dpRoots.push(j.home); renderDpRoots(); } };
$('#dpTypes').onclick = e => { const c = e.target.closest('.chip'); if (!c) return; dpExt = c.dataset.x; $$('#dpTypes .chip').forEach(x => x.classList.toggle('on', x===c)); };
$('#dpGo').onclick = () => {
  if (dpEs){ dpEs.close(); dpEs = null; dpUI(false); $('#dpText').textContent = 'Búsqueda detenida'; $('#dpDot').className='dot warn'; return; }
  if (!dpRoots.length) return toast('Añade al menos una carpeta', 'warn');
  const p = new URLSearchParams({t:T, min:$('#dpMin').value, hidden:$('#dpHidden').checked?1:0, ext:dpExt});
  dpRoots.forEach(r => p.append('root', r));
  $('#dpResult').innerHTML = ''; $('#dpStatus').style.display = 'flex'; $('#dpDot').className = 'dot run'; $('#dpText').textContent = 'Buscando archivos…'; $('#dpBar').style.width = '0';
  dpUI(true);
  dpEs = new EventSource('/api/dupes?' + p);
  dpEs.addEventListener('p', e => { const s = JSON.parse(e.data); $('#dpText').textContent = `${s.phase} · ${fmtN(s.n)} archivos`; $('#dpCur').textContent = s.cur || (s.total ? `${fmtBytes(s.bytes,0)} de ${fmtBytes(s.total,0)}` : ''); $('#dpBar').style.width = s.total ? Math.min(100, s.bytes/s.total*100).toFixed(1)+'%' : '0'; });
  dpEs.addEventListener('done', e => { dpRes = JSON.parse(e.data); dpEs.close(); dpEs = null; dpUI(false); $('#dpDot').className = 'dot ok'; $('#dpText').textContent = `${fmtN(dpRes.scanned)} archivos comparados · ${fmtMs(dpRes.ms)}`; $('#dpCur').textContent = ''; $('#dpBar').style.width = '100%'; dpAuto('oldest'); });
  dpEs.addEventListener('err', e => { dpEs.close(); dpEs = null; dpUI(false); $('#dpDot').className='dot bad'; $('#dpText').textContent = JSON.parse(e.data); });
  dpEs.onerror = () => { if (dpEs){ dpEs.close(); dpEs = null; dpUI(false); } };
};
function dpUI(run){ $('#dpGo').innerHTML = run ? ic('x') + 'Detener' : ic('search') + 'Buscar duplicados'; $('#dpGo').className = 'btn ' + (run ? 'danger' : 'primary'); }
function dpAuto(mode){
  // marca para borrar todos menos uno por grupo
  dpRes.groups.forEach(g => {
    let keep = 0;
    if (mode === 'newest') keep = g.files.reduce((b,f,i,a) => f.m > a[b].m ? i : b, 0);
    else if (mode === 'oldest') keep = g.files.reduce((b,f,i,a) => f.m < a[b].m ? i : b, 0);
    else if (mode === 'short') keep = g.files.reduce((b,f,i,a) => f.p.length < a[b].p.length ? i : b, 0);
    g.files.forEach((f,i) => f.del = mode === 'none' ? false : i !== keep);
  });
  renderDupes();
}
function renderDupes(){
  const r = dpRes;
  if (!r.groups.length){ $('#dpResult').innerHTML = `<div class="alert ok">${ic('check')}<div><b>No hay duplicados</b>No se han encontrado archivos con el mismo contenido.</div></div>`; return; }
  const selN = r.groups.reduce((s,g) => s + g.files.filter(f => f.del).length, 0);
  const selB = r.groups.reduce((s,g) => s + g.files.filter(f => f.del).length * g.size, 0);
  $('#dpResult').innerHTML = `<div class="dbar"><b>${fmtN(r.groups.length)} grupos</b><span class="muted">· ${fmtBytes(r.wasted,1)} ocupados por copias</span>
    <span class="faint" style="margin-left:14px">Marcar:</span><button class="btn sm ghost" data-auto="oldest">Conservar el más antiguo</button><button class="btn sm ghost" data-auto="newest">Conservar el más reciente</button><button class="btn sm ghost" data-auto="short">Conservar ruta más corta</button><button class="btn sm ghost" data-auto="none">Ninguno</button>
    <button class="btn sm danger" id="dpDel" style="margin-left:auto" ${selN?'':'disabled'}>${ic('trash')}Enviar ${fmtN(selN)} a la Papelera (${fmtBytes(selB,1)})</button></div>
    ${r.groups.slice(0, 500).map((g,gi) => `<div class="dgroup"><div class="dh"><b>${fmtBytes(g.size,1)}</b><span class="muted">${g.files.length} copias idénticas</span><span class="faint mono" style="margin-left:auto">${g.hash}</span></div>
      ${g.files.map((f,fi) => `<label class="${f.del?'':'keep'}"><input type="checkbox" class="check" data-g="${gi}" data-f="${fi}" ${f.del?'checked':''}><span class="pth" title="${esc(f.p)}">\u200E${esc(f.p)}</span><span class="dt">${fmtDate(f.m)}</span>${f.del?'':'<span class="badge b-security">Se conserva</span>'}<span class="ib" data-open="${esc(f.p)}" title="Abrir">${ic('external')}</span></label>`).join('')}</div>`).join('')}
    ${r.groups.length > 500 ? `<div class="faint">Se muestran los 500 grupos con más espacio recuperable.</div>` : ''}`;
}
$('#dpResult').addEventListener('click', async e => {
  const a = e.target.closest('[data-auto]'); if (a) return dpAuto(a.dataset.auto);
  const o = e.target.closest('[data-open]'); if (o){ e.preventDefault(); return api('open', {path:o.dataset.open}).catch(()=>{}); }
  if (e.target.id === 'dpDel' || e.target.closest('#dpDel')){
    const paths = dpRes.groups.flatMap(g => g.files.filter(f => f.del).map(f => f.p));
    const ok = await modal({title:`¿Enviar ${fmtN(paths.length)} archivos a la Papelera?`, text:'En cada grupo se conserva al menos una copia. Podrás recuperarlos desde la Papelera.', icon:'trash', tone:'danger', ok:'Enviar a la Papelera', okClass:'danger'});
    if (!ok) return;
    const b = $('#dpDel'); setBusy(b, true, 'Eliminando…');
    const r = await api('files/delete', {paths, recycle:true});
    const failed = new Set((r.failed||[]).map(x => x.toLowerCase()));
    dpRes.groups.forEach(g => { g.files = g.files.filter(f => !f.del || failed.has(f.p.toLowerCase())); });
    dpRes.groups = dpRes.groups.filter(g => g.files.length > 1);
    dpRes.wasted = dpRes.groups.reduce((s,g) => s + g.size*(g.files.length-1), 0);
    toast(`${fmtN(r.ok)} archivos enviados a la Papelera`, failed.size ? 'warn' : 'ok', failed.size ? `${failed.size} no se pudieron eliminar (en uso o sin permiso).` : '');
    dpAuto('none');
  }
});
$('#dpResult').addEventListener('change', e => {
  const c = e.target; if (c.dataset.g === undefined) return;
  const g = dpRes.groups[+c.dataset.g];
  g.files[+c.dataset.f].del = c.checked;
  if (g.files.every(f => f.del)){ g.files[+c.dataset.f].del = false; toast('Debe quedar al menos una copia de cada archivo', 'warn'); }
  renderDupes();
});
