'use strict';
/* Programas de inicio, Desinstalador, Renombrar en masa y Actualizaciones */

// ================================================================ INICIO
let SU = null;
PAGES.startup = { async enter(){ await loadStartup(); } };
async function loadStartup(){
  try{ SU = await api('startup'); }catch(e){ return; }
  const items = SU.items || [];
  const en = items.filter(i => i.enabled).length, miss = items.filter(i => i.missing).length;
  $('#suStrip').innerHTML = `<span>Programas<b>${items.length}</b></span><span>Se inician con Windows<b>${en}</b></span><span>Desactivados<b>${items.length - en}</b></span>${miss ? `<span style="color:var(--warn)">Entradas rotas<b>${miss}</b></span>` : ''}`;
  $('#suBody').innerHTML = items.map((it,i) => `<tr class="${it.enabled?'':'dim'}">
    <td><div class="pname"><div class="pi">${esc(initials(it.desc || it.name))}</div><div class="tx"><b>${esc(it.desc && it.desc.length < 50 ? it.desc : it.name)}${it.running ? ' <span class="badge b-app" style="margin-left:4px">En ejecución</span>' : ''}${it.missing ? ' <span class="badge b-bloat" style="margin-left:4px">Archivo no encontrado</span>' : ''}</b><small title="${esc(it.command)}">${esc(it.exe || it.command)}</small></div></div></td>
    <td class="muted">${esc(it.publisher || '—')}</td>
    <td><span class="muted">${esc(it.source)}</span><div class="faint" style="font-size:11.5px;margin-top:-2px">${esc(it.scope)}${it.admin && !SU.admin ? ' · requiere admin.' : ''}</div></td>
    <td><label class="switch" title="${it.enabled ? 'Desactivar' : 'Activar'}"><input type="checkbox" data-su="${i}" ${it.enabled?'checked':''} ${it.admin && !SU.admin ? 'disabled' : ''}><span></span></label></td>
    <td class="r-right">${it.exe && !it.missing && /[\\/]/.test(it.exe) ? `<span class="ib" style="display:inline-grid" data-rev="${i}" title="Abrir ubicación">${ic('folder')}</span>` : ''} ${it.source !== 'Aplicación de la Tienda' ? `<span class="ib" style="display:inline-grid" data-del="${i}" title="Eliminar entrada">${ic('trash')}</span>` : ''}</td></tr>`).join('') || `<tr><td colspan="5" style="max-width:none"><div class="empty"><b>No hay programas de inicio</b></div></td></tr>`;
}
$('#suRefresh').onclick = () => loadStartup();
$('#suBody').addEventListener('change', async e => {
  const i = e.target.dataset.su; if (i === undefined) return;
  const it = SU.items[+i];
  const r = await api('startup/set', {id: it.id, enable: e.target.checked});
  if (r.error){ toast(r.error, 'warn'); e.target.checked = !e.target.checked; return; }
  toast(e.target.checked ? `${it.name} se iniciará con Windows` : `${it.name} ya no se iniciará con Windows`, 'ok');
  loadStartup();
});
$('#suBody').addEventListener('click', async e => {
  const rv = e.target.closest('[data-rev]'); if (rv) return api('reveal', {path: SU.items[+rv.dataset.rev].exe}).catch(() => toast('No se encontró el archivo', 'warn'));
  const dl = e.target.closest('[data-del]'); if (!dl) return;
  const it = SU.items[+dl.dataset.del];
  const ok = await modal({title:`¿Eliminar «${it.name}» del inicio?`, text: it.missing ? 'Es una entrada rota: el programa ya no existe.' : 'Se quita la entrada de inicio (el programa no se desinstala). Si solo quieres que no arranque, es mejor desactivarlo.', body:`<div class="pathbox mono">${esc(it.command)}</div>`, icon:'trash', tone:'danger', ok:'Eliminar entrada', okClass:'danger'});
  if (!ok) return;
  const r = await api('startup/set', {id: it.id, delete: true});
  if (r.error) toast(r.error, 'warn'); else toast('Entrada eliminada', 'ok');
  loadStartup();
});

// ================================================================ DESINSTALADOR
let PR = null, unSort = 'name';
PAGES.uninstall = { async enter(){ if (!PR) await loadPrograms(); } };
async function loadPrograms(){
  $('#unBody').innerHTML = `<tr><td colspan="5" style="max-width:none"><div class="empty"><b>Cargando programas…</b></div></td></tr>`;
  try{ PR = await api('programs'); }catch(e){ return; }
  renderPrograms();
}
function dateKey(d){ if (!d) return ''; const [dd,mm,yy] = d.split('/'); return yy+mm+dd; }
function renderPrograms(){
  const q = $('#unFilter').value.trim().toLowerCase();
  let list = (PR.programs||[]).filter(p => !q || (p.name+' '+p.publisher).toLowerCase().includes(q));
  if (unSort === 'size') list.sort((a,b) => b.size - a.size);
  else if (unSort === 'date') list.sort((a,b) => dateKey(b.date).localeCompare(dateKey(a.date)));
  $('#unCount').textContent = `${list.length} programas · ${fmtBytes(list.reduce((s,p) => s+p.size, 0), 1)}`;
  $('#unBody').innerHTML = list.map(p => `<tr><td><div class="pname"><div class="pi">${esc(initials(p.name))}</div><div class="tx"><b>${esc(p.name)}</b><small>${esc(p.publisher || 'Editor desconocido')}${p.arch ? ' · ' + p.arch : ''}${p.scope === 'Usuario' ? ' · solo tu usuario' : ''}</small></div></div></td>
    <td class="muted">${esc(p.version||'')}</td><td class="r-right num">${p.size ? fmtBytes(p.size,1) : '<span class="faint">—</span>'}</td><td class="muted num">${esc(p.date||'')}</td>
    <td class="r-right"><button class="btn sm" data-un="${esc(p.id)}">Desinstalar</button></td></tr>`).join('') || `<tr><td colspan="5" style="max-width:none"><div class="empty"><b>Sin resultados</b></div></td></tr>`;
}
$('#unFilter').addEventListener('input', renderPrograms);
$$('#unSort button').forEach(b => b.onclick = () => { unSort = b.dataset.s; $$('#unSort button').forEach(x => x.classList.toggle('on', x===b)); renderPrograms(); });
$('#unBody').addEventListener('click', e => { const b = e.target.closest('[data-un]'); if (b) uninstallFlow(PR.programs.find(p => p.id === b.dataset.un)); });

async function uninstallFlow(p){
  if (!p) return;
  const ok = await modal({title:`¿Desinstalar ${p.name}?`, text:'Se abrirá el desinstalador oficial del programa. Sigue sus pasos; al terminar, Vantix buscará los restos que deje.', icon:'package', tone:'danger', ok:'Desinstalar', okClass:'danger',
    body: `<div class="kv"><div>Editor</div><div>${esc(p.publisher||'—')}</div><div>Versión</div><div>${esc(p.version||'—')}</div>${p.location ? `<div>Ubicación</div><div class="mono">${esc(p.location)}</div>` : ''}</div>`});
  if (!ok) return;
  const r = await api('programs/uninstall', {id: p.id});
  if (r.error) return toast(r.error, 'bad');
  // diálogo de espera
  modalOpen = true;
  const bg = $('#modalBg');
  bg.innerHTML = `<div class="modal"><div class="mh"><div class="mi">${ic('package')}</div><div><h3>Desinstalando ${esc(p.name)}</h3><p>Completa los pasos en la ventana del desinstalador. Esta ventana se actualizará sola.</p></div></div><div class="mb"><div style="display:flex;align-items:center;gap:12px"><span class="spinner" style="border-color:rgba(255,255,255,.15);border-top-color:var(--accent)"></span><span class="muted">Esperando a que termine el desinstalador…</span></div></div><div class="mf"><button class="btn" id="unWaitClose">Seguir en segundo plano</button></div></div>`;
  bg.classList.add('on');
  let stop = false;
  $('#unWaitClose').onclick = () => { stop = true; bg.classList.remove('on'); bg.innerHTML = ''; modalOpen = false; };
  let st;
  for (;;){
    await new Promise(r => setTimeout(r, 1500));
    try{ st = await api('programs/status?id=' + encodeURIComponent(p.id)); }catch(e){ continue; }
    if (st.state !== 'running') break;
  }
  if (!stop){ bg.classList.remove('on'); bg.innerHTML = ''; modalOpen = false; }
  const lo = await api('programs/leftovers?id=' + encodeURIComponent(p.id));
  const items = lo.items || [];
  if (!st.removed && !items.length){
    toast(`${p.name} sigue instalado`, 'warn', 'Parece que la desinstalación se canceló o no terminó.');
    return loadPrograms();
  }
  if (!items.length){ toast(`${p.name} se ha desinstalado`, 'ok', 'No se han encontrado restos.'); return loadPrograms(); }
  const body = `<div class="mlist">${items.map((it,i) => `<label><input type="checkbox" class="check" data-i="${i}" checked><div class="nm"><b class="mono" style="font-size:12px">${esc(it.path)}</b><small>${esc(it.label)}${it.kind==='folder' ? ' · carpeta' : ' · registro'}</small></div><span class="num muted" style="width:76px;text-align:right">${it.size ? fmtBytes(it.size,1) : ''}</span></label>`).join('')}</div>
    <p class="faint" style="font-size:12px;margin:10px 0 0">Las carpetas se envían a la Papelera. Revisa la lista antes de continuar.</p>`;
  const sel = await modal({title: st.removed ? `${p.name} desinstalado. Se han encontrado restos` : 'Se han encontrado restos', text:`${items.length} elementos que el desinstalador ha dejado en el equipo.`, body, icon:'sparkles', ok:'Eliminar seleccionados', wide:true,
    onOk: m => items.filter((_,i) => m.querySelector(`[data-i="${i}"]`).checked)});
  if (sel && sel.length){
    const r2 = await api('programs/clean', {items: sel});
    toast(`${r2.done} restos eliminados`, (r2.errors||[]).length ? 'warn' : 'ok', (r2.errors||[]).length ? `${r2.errors.length} no se pudieron eliminar: ${r2.errors[0]}` : '');
  }
  loadPrograms();
}

// ================================================================ RENOMBRAR
let RN = [], rnPlan = [];
const rnIds = ['rFind','rRepl','rRegex','rCase','rBase','rPre','rSuf','rNum','rStart','rStep','rDigits','rSep','rNumPos','rOrder','rDate','rDateFmt','rDatePos','rCaseMode','rExtMode','rExtNew','rUnder','rSpaces','rAccents'];
$('#rnPick').onclick = async () => { const r = await api('pick?kind=folder'); if (r.path){ $('#rnPath').value = r.path; rnLoad(); } };
$('#rnLoad').onclick = () => rnLoad();
$('#rnPath').addEventListener('keydown', e => { if (e.key === 'Enter') rnLoad(); });
['rnRec','rnDirs'].forEach(id => $('#'+id).onchange = () => RN.length && rnLoad());
$('#rnExt').addEventListener('input', () => rnPreview());
async function rnLoad(){
  const path = $('#rnPath').value.trim(); if (!path) return toast('Elige una carpeta', 'warn');
  const r = await api('rename/list', {path, recursive:$('#rnRec').checked, dirs:$('#rnDirs').checked});
  if (r.error) return toast(r.error, 'bad');
  RN = r.files || []; rnPreview();
  if (RN.length >= 20000) toast('Se han cargado los primeros 20.000 elementos', 'warn');
}
rnIds.forEach(id => { const el = $('#'+id); el.addEventListener(el.type === 'checkbox' || el.tagName === 'SELECT' ? 'change' : 'input', () => { if (id === 'rExtMode') $('#rExtNew').style.display = el.value === 'set' ? '' : 'none'; rnPreview(); }); });
$('#rnReset').onclick = () => { rnIds.forEach(id => { const el = $('#'+id); if (el.type === 'checkbox') el.checked = false; else if (el.tagName !== 'SELECT') el.value = ''; else el.selectedIndex = 0; }); $('#rStart').value = 1; $('#rStep').value = 1; $('#rDigits').value = 3; $('#rSep').value = ' '; $('#rExtNew').style.display = 'none'; rnPreview(); };
const pad = (n, d) => String(n).padStart(d, '0');
function fmtD(ts, f){ const d = new Date(ts*1000); const Y=d.getFullYear(), m=pad(d.getMonth()+1,2), dd=pad(d.getDate(),2), H=pad(d.getHours(),2), M=pad(d.getMinutes(),2), S=pad(d.getSeconds(),2);
  return {'Y-m-d':`${Y}-${m}-${dd}`, 'Ymd':`${Y}${m}${dd}`, 'd-m-Y':`${dd}-${m}-${Y}`, 'Y-m-d_His':`${Y}-${m}-${dd}_${H}${M}${S}`}[f]; }
function titleCase(s){ return s.toLowerCase().replace(/(^|[\s_\-.(\[])(\p{L})/gu, (m,a,b) => a + b.toUpperCase()); }
function rnPreview(){
  const v = id => $('#'+id).type === 'checkbox' ? $('#'+id).checked : $('#'+id).value;
  const exts = $('#rnExt').value.split(/[,; ]+/).map(x => x.trim().toLowerCase().replace(/^\./,'')).filter(Boolean);
  let files = RN.filter(f => !exts.length || f.isDir || exts.includes(f.name.split('.').pop().toLowerCase()));
  const order = v('rOrder');
  const sorted = [...files].sort((a,b) => order === 'date' ? a.m - b.m : order === 'size' ? a.size - b.size : a.name.localeCompare(b.name, 'es', {numeric:true, sensitivity:'base'}));
  let re = null, err = '';
  if (v('rFind') && v('rRegex')){ try{ re = new RegExp(v('rFind'), v('rCase') ? 'g' : 'gi'); }catch(e){ err = 'Expresión regular no válida'; } }
  const counters = {};
  rnPlan = sorted.map(f => {
    const dot = f.isDir ? -1 : f.name.lastIndexOf('.');
    let base = dot > 0 ? f.name.slice(0, dot) : f.name, ext = dot > 0 ? f.name.slice(dot) : '';
    if (v('rFind')){
      if (re) base = base.replace(re, v('rRepl'));
      else { const fnd = v('rFind'); base = v('rCase') ? base.split(fnd).join(v('rRepl')) : base.replace(new RegExp(fnd.replace(/[.*+?^${}()|[\]\\]/g, '\\$&'), 'gi'), () => v('rRepl')); }
    }
    if (v('rBase')) base = v('rBase');
    if (v('rAccents')) base = base.normalize('NFD').replace(/[̀-ͯ]/g, '');
    if (v('rSpaces')) base = base.replace(/\s{2,}/g, ' ').trim();
    if (v('rUnder')) base = base.replace(/ /g, '_');
    const cm = v('rCaseMode');
    if (cm === 'lower') base = base.toLowerCase(); else if (cm === 'upper') base = base.toUpperCase(); else if (cm === 'title') base = titleCase(base); else if (cm === 'sentence') base = base.charAt(0).toUpperCase() + base.slice(1).toLowerCase();
    base = v('rPre') + base + v('rSuf');
    if (v('rDate')){ const ds = fmtD(f.m, v('rDateFmt')); base = v('rDatePos') === 'start' ? ds + ' ' + base : base + ' ' + ds; }
    if (v('rNum')){
      const k = f.dir; counters[k] = counters[k] ?? 0;
      const n = (+v('rStart')||0) + counters[k]++ * (+v('rStep')||1);
      const ns = pad(n, Math.max(1, Math.min(8, +v('rDigits')||1)));
      base = v('rNumPos') === 'start' ? ns + v('rSep') + base : base + v('rSep') + ns;
    }
    const em = v('rExtMode');
    if (ext){ if (em === 'lower') ext = ext.toLowerCase(); else if (em === 'upper') ext = ext.toUpperCase(); else if (em === 'set' && v('rExtNew').trim()) ext = '.' + v('rExtNew').trim().replace(/^\./,''); }
    return {f, to: base + ext, on: true};
  });
  // validación
  const seen = new Map();
  rnPlan.forEach(p => { const k = (p.f.dir + '\\' + p.to).toLowerCase(); seen.set(k, (seen.get(k)||0) + 1); });
  const existing = new Set(RN.map(f => (f.dir + '\\' + f.name).toLowerCase()));
  const renamedFrom = new Set(rnPlan.filter(p => p.to !== p.f.name).map(p => (p.f.dir + '\\' + p.f.name).toLowerCase()));
  rnPlan.forEach(p => {
    const k = (p.f.dir + '\\' + p.to).toLowerCase();
    if (p.to === p.f.name) p.st = 'same';
    else if (!p.to || /[<>:"/\\|?*\x00-\x1f]/.test(p.to) || /[ .]$/.test(p.to)) p.st = 'bad', p.msg = 'Nombre no válido';
    else if (seen.get(k) > 1) p.st = 'bad', p.msg = 'Nombre repetido';
    else if (existing.has(k) && !renamedFrom.has(k) && k !== (p.f.dir + '\\' + p.f.name).toLowerCase()) p.st = 'bad', p.msg = 'Ya existe';
    else p.st = 'ok';
  });
  const ok = rnPlan.filter(p => p.st === 'ok').length, bad = rnPlan.filter(p => p.st === 'bad').length;
  $('#rnSum').textContent = RN.length ? `${fmtN(files.length)} elementos · ${fmtN(ok)} cambiarán${bad ? ` · ${bad} con problemas` : ''}${err ? ' · ' + err : ''}` : '';
  const multiDir = new Set(files.map(f => f.dir)).size > 1;
  $('#rnBody').innerHTML = rnPlan.slice(0, 2000).map((p,i) => `<tr><td>${p.st === 'ok' ? `<input type="checkbox" class="check" data-rn="${i}" ${p.on?'checked':''}>` : ''}</td>
    <td title="${esc(p.f.dir + '\\' + p.f.name)}">${p.f.isDir ? ic('folder') + ' ' : ''}${esc(p.f.name)}${multiDir ? `<div class="faint" style="font-size:11px;margin-top:-3px">${esc(p.f.dir)}</div>` : ''}</td>
    <td class="${p.st==='ok'?'newname':''}">${esc(p.to)}</td>
    <td class="${p.st==='ok'?'st-ok':p.st==='bad'?'st-bad':'st-same'}">${p.st==='ok' ? 'Se renombrará' : p.st==='bad' ? esc(p.msg) : 'Sin cambios'}</td></tr>`).join('')
    || `<tr><td colspan="4" style="max-width:none"><div class="empty"><div class="ic">${ic('edit')}</div><b>Elige una carpeta y pulsa Cargar</b>Después ajusta las reglas: la vista previa se actualiza al momento.</div></td></tr>`;
  if (rnPlan.length > 2000) $('#rnBody').insertAdjacentHTML('beforeend', `<tr><td colspan="4" class="faint" style="max-width:none">Se muestran 2.000 de ${fmtN(rnPlan.length)}; se aplicarán todos.</td></tr>`);
  rnBtn();
}
function rnBtn(){ const n = rnPlan.filter(p => p.st === 'ok' && p.on).length; $('#rnApply').disabled = !n; $('#rnApply').querySelector('span').textContent = n ? `Aplicar ${fmtN(n)} cambios` : 'Aplicar cambios'; }
$('#rnBody').addEventListener('change', e => { const i = e.target.dataset.rn; if (i === undefined) return; rnPlan[+i].on = e.target.checked; rnBtn(); });
$('#rnAll').onchange = e => { rnPlan.forEach(p => p.on = e.target.checked); $$('[data-rn]').forEach(c => c.checked = e.target.checked); rnBtn(); };
$('#rnApply').onclick = async () => {
  const ops = rnPlan.filter(p => p.st === 'ok' && p.on).map(p => ({dir: p.f.dir, from: p.f.name, to: p.to}));
  const ok = await modal({title:`¿Renombrar ${fmtN(ops.length)} elementos?`, text:'Podrás deshacerlo con «Deshacer último».', icon:'edit', ok:'Renombrar'});
  if (!ok) return;
  const b = $('#rnApply'); setBusy(b, true, 'Renombrando…');
  try{ const r = await api('rename/apply', {ops}); toast(`${fmtN(r.done)} elementos renombrados`, (r.errors||[]).length ? 'warn' : 'ok', (r.errors||[]).length ? `${r.errors.length} errores: ${r.errors[0]}` : ''); }
  catch(e){ toast('Error al renombrar', 'bad'); }
  finally{ setBusy(b, false); }
  rnLoad();
};
$('#rnUndo').onclick = async () => {
  const r = await api('rename/undo', {});
  if (!r.done && (r.errors||[]).length) return toast(r.errors[0], 'warn');
  toast(`Deshecho: ${fmtN(r.done)} nombres restaurados`, 'ok');
  if (RN.length) rnLoad();
};

// ================================================================ ACTUALIZACIONES
let updPoll = null;
async function updStatus(){
  let s; try{ s = await api('update/status'); }catch(e){ return; }
  const rel = s.release;
  $('#sVer').textContent = 'Versión ' + (STATUS.version || '');
  if (rel && rel.available){
    $('#updBox').style.display = ''; $('#updTitle').textContent = `Versión ${rel.version} disponible`; $('#updSub').textContent = 'Tienes la ' + rel.current;
    $('#sUpdTitle').textContent = `Nueva versión ${rel.version} disponible`; $('#sUpdText').textContent = (rel.notes || '').split('\n')[0] || `Tienes la versión ${rel.current}.`;
    $('#sUpdInstall').style.display = '';
  } else if (rel){
    $('#updBox').style.display = 'none'; $('#sUpdInstall').style.display = 'none';
    $('#sUpdTitle').textContent = rel.error ? 'No se pudo comprobar' : 'Tienes la última versión';
    $('#sUpdText').textContent = rel.error || `Versión ${rel.current} · comprobado ${new Date(rel.checked*1000).toLocaleTimeString('es-ES',{hour:'2-digit',minute:'2-digit'})}`;
  }
  const bar = $('#sUpdBar');
  if (s.state === 'downloading'){
    bar.style.display = ''; bar.firstElementChild.style.width = s.total ? (s.done/s.total*100).toFixed(1)+'%' : '30%';
    $('#sUpdText').textContent = `Descargando… ${fmtBytes(s.done||0,1)}${s.total ? ' de ' + fmtBytes(s.total,1) : ''}`;
    $('#updSub').textContent = $('#sUpdText').textContent;
  } else bar.style.display = 'none';
  if (s.state === 'restarting'){ $('#sUpdText').textContent = 'Reiniciando con la nueva versión…'; elevatingNow = false; }
  if (s.state === 'error' && s.error){ toast(s.error, 'bad'); $('#sUpdText').textContent = s.error; }
  if (s.state !== 'downloading' && updPoll){ clearInterval(updPoll); updPoll = null; }
}
async function updInstall(){
  const r = await api('update/install', {});
  if (r.error) return toast(r.error, 'warn');
  toast('Descargando la actualización…', 'info', 'La aplicación se reiniciará sola al terminar.');
  if (!updPoll) updPoll = setInterval(updStatus, 700);
  updStatus();
}
$('#updBtn').onclick = updInstall;
$('#sUpdInstall').onclick = updInstall;
$('#sUpdCheck').onclick = async () => { const b = $('#sUpdCheck'); setBusy(b, true, 'Comprobando…'); try{ await api('update/check'); }catch(e){} setBusy(b, false); updStatus(); };
setTimeout(updStatus, 7000);
setInterval(updStatus, 30 * 60 * 1000);
PAGES.settings = { enter(){ updStatus(); } };
