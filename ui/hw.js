'use strict';
/* Hardware, Red, Modo juego e Información del sistema */

const fmtRate = b => fmtBytes(b, (b||0) < 10*1024 ? 1 : 0) + '/s';
const fmtMHz = m => m >= 1000 ? (m/1000).toFixed(2).replace('.',',') + ' GHz' : Math.round(m) + ' MHz';
const tempClass = t => t >= 85 ? 'bad' : t >= 70 ? 'warn' : 'ok';
const fmtTemp = t => Math.round(t) + ' °C';

// ================================================================ HARDWARE
const hwHist = {cpu:[], gpu:[]};
let hwFirst = true;
function lhmFind(list, re, hwRe){
  return (list||[]).find(s => /temp/i.test(s.group) && re.test(s.name) && (!hwRe || hwRe.test(s.hw)));
}
function parseTemp(v){ const m = String(v||'').replace(',', '.').match(/-?\d+(\.\d+)?/); return m ? +m[0] : 0; }

PAGES.hardware = {
  interval: 2000,
  enter(){ if (hwFirst){ $('#hwSensors').innerHTML = '<div class="faint">Cargando…</div>'; } },
  async poll(){ const d = await api('hardware'); renderHardware(d); hwFirst = false; },
};

function renderHardware(d){
  const lhm = d.lhm || [];
  // ---- temperaturas destacadas
  const chips = [];
  const cpuT = lhmFind(lhm, /package|tctl|tdie|cpu \(tctl|core \(tctl|cpu$/i, /ryzen|intel|amd|core|cpu/i);
  if (cpuT) chips.push(['Procesador', parseTemp(cpuT.value), 'cpu']);
  else if ((d.thermal||[]).length) chips.push(['Zona térmica (placa)', Math.max(...d.thermal.map(t => t.temp)), 'thermo']);
  (d.gpu.nvidia||[]).forEach(g => { if (g.temp) chips.push([g.name.replace(/NVIDIA |GeForce /g,''), g.temp, 'monitor']); });
  if (!(d.gpu.nvidia||[]).length){ const gt = lhmFind(lhm, /gpu core|gpu hot spot|gpu$/i); if (gt) chips.push(['Gráfica', parseTemp(gt.value), 'monitor']); }
  (d.disks||[]).forEach(k => { if (k.temp) chips.push([(k.volumes||[]).join(' ') + ' ' + (k.bus==='NVMe'?'NVMe':k.kind||'Disco'), k.temp, 'disk']); });
  const mb = lhmFind(lhm, /system|motherboard|mainboard/i); if (mb) chips.push(['Placa base', parseTemp(mb.value), 'thermo']);
  $('#hwTemps').innerHTML = chips.length ? chips.map(([n,t,i]) => `<div class="tchip ${tempClass(t)}"><div class="ti">${ic(i)}</div><div><small>${esc(n.trim())}</small><div class="tv num">${fmtTemp(t)}</div></div></div>`).join('')
    : `<div class="alert info" style="flex:1">${ic('thermo')}<div>Windows no expone la temperatura del procesador sin un controlador especial. Para verla, abre <strong>LibreHardwareMonitor</strong> con su servidor web activado (ver «Todos los sensores»).</div></div>`;

  // ---- CPU
  const c = d.cpu, info = c.info || {};
  hwHist.cpu.push(c.usage); if (hwHist.cpu.length > 60) hwHist.cpu.shift();
  $('#hwCpuName').textContent = info.name || '';
  const cores = c.cores || [];
  let cpuHtml = `<div class="bigrow"><div><small>Uso</small><b class="num">${Math.round(c.usage)}<span>%</span></b></div>
    <div><small>Frecuencia</small><b class="num">${c.mhz ? fmtMHz(c.mhz) : '–'}</b></div>
    ${cpuT ? `<div><small>Temperatura</small><b class="num">${fmtTemp(parseTemp(cpuT.value))}</b></div>` : ''}
    <div><small>Núcleos / hilos</small><b class="num">${info.cores||'–'} / ${info.threads||'–'}</b></div></div>
    <canvas id="hwCpuSpark" style="width:100%;height:46px;display:block"></canvas>
    <div class="sub-h">Uso por núcleo lógico</div>
    <div class="cores">${cores.map((k,i) => `<div class="core"><i style="height:${Math.min(100,k.v).toFixed(0)}%"></i><span>${i}</span><b class="num">${Math.round(k.v)}%</b></div>`).join('')}</div>
    <div class="kv cols2">
      <div>Velocidad base</div><div>${info.baseMHz ? fmtMHz(info.baseMHz) : '–'}</div>
      <div>Zócalo</div><div>${esc(info.socket||'–')}</div>
      <div>Caché L2</div><div>${info.l2 ? fmtBytes(info.l2,0) : '–'}</div>
      <div>Caché L3</div><div>${info.l3 ? fmtBytes(info.l3,0) : '–'}</div>
      <div>Procesos</div><div class="num">${fmtN(c.procs)}</div>
      <div>Hilos</div><div class="num">${fmtN(c.threads)}</div>
      <div>Identificadores</div><div class="num">${fmtN(c.handles)}</div>
      <div>Tiempo encendido</div><div>${fmtUptime(c.uptime)}</div>
    </div>`;
  $('#hwCpu').innerHTML = cpuHtml;
  spark($('#hwCpuSpark'), hwHist.cpu, '#3d8bfd');

  // ---- GPU
  const nv = d.gpu.nvidia || [], names = d.gpu.names || [];
  $('#hwGpuName').textContent = nv.length ? nv[0].name : (names[0]?.name || '');
  let g = '';
  if (nv.length){
    for (const x of nv){
      g += `<div class="bigrow"><div><small>Uso</small><b class="num">${Math.round(x.util)}<span>%</span></b></div>
        <div><small>Temperatura</small><b class="num">${x.temp ? fmtTemp(x.temp) : '–'}</b></div>
        <div><small>Consumo</small><b class="num">${x.power ? Math.round(x.power) : '–'}<span>W${x.powerMax ? ' / ' + Math.round(x.powerMax) : ''}</span></b></div>
        <div><small>Ventilador</small><b class="num">${x.fan ?? '–'}<span>%</span></b></div></div>
        <div class="sub-h">Memoria de vídeo (VRAM)</div>
        <div class="bar" style="height:8px"><i style="width:${x.memTotal ? (x.memUsed/x.memTotal*100).toFixed(1) : 0}%"></i></div>
        <div class="faint" style="font-size:12px;margin:5px 0 12px">${fmtBytes(x.memUsed,1)} de ${fmtBytes(x.memTotal,0)}</div>
        <div class="kv cols2"><div>Reloj gráfico</div><div class="num">${x.clock ? x.clock + ' MHz' : '–'}</div><div>Reloj memoria</div><div class="num">${x.memClock ? x.memClock + ' MHz' : '–'}</div><div>Controlador</div><div>${esc(x.driver||'–')}</div><div>Uso de memoria</div><div class="num">${Math.round(x.memUtil)} %</div></div>`;
    }
  } else {
    const a = (d.gpu.adapters||[])[0];
    g += `<div class="bigrow"><div><small>Uso</small><b class="num">${Math.round(d.gpu.util||0)}<span>%</span></b></div>
      ${a ? `<div><small>Memoria dedicada en uso</small><b class="num">${fmtBytes(a.ded,1)}</b></div>` : ''}
      ${names[0]?.vram ? `<div><small>VRAM total</small><b class="num">${fmtBytes(names[0].vram,0)}</b></div>` : ''}</div>
      <div class="kv"><div>Controlador</div><div>${esc(names[0]?.driver||'–')}</div></div>`;
  }
  const eng = d.gpu.engines || {};
  const engNames = {'3D':'3D', 'Copy':'Copia', 'VideoDecode':'Decodificación de vídeo', 'VideoEncode':'Codificación de vídeo', 'Compute_0':'Cálculo', 'Compute_1':'Cálculo 1'};
  const keys = Object.keys(eng).filter(k => engNames[k]).sort((a,b) => eng[b]-eng[a]).slice(0,4);
  if (keys.length) g += `<div class="sub-h">Motores</div><div class="engines">${keys.map(k => `<div class="eng"><div><span>${engNames[k]}</span><b class="num">${Math.round(eng[k])} %</b></div><div class="bar"><i style="width:${Math.min(100,eng[k]).toFixed(0)}%"></i></div></div>`).join('')}</div>`;
  if (names.length > 1) g += `<div class="sub-h">Adaptadores</div><div class="kv">${names.map(n => `<div>${esc(n.name)}</div><div>${n.vram ? fmtBytes(n.vram,0) : ''}</div>`).join('')}</div>`;
  $('#hwGpu').innerHTML = g || '<div class="faint">No se ha detectado ninguna gráfica.</div>';

  // ---- Memoria
  const m = d.mem, used = m.total - m.avail;
  const inUse = Math.max(0, m.total - m.avail - m.modified);
  const parts = [['En uso', inUse, '#3d8bfd'], ['Modificada', m.modified, '#e3a23b'], ['En espera (caché)', m.standby, '#9384f7'], ['Libre', m.free, '#2fb67c']];
  const mods = m.modules || [];
  $('#hwMemSub').textContent = mods.length ? `${mods[0].type||''} ${mods[0].confSpeed||mods[0].speed ? (mods[0].confSpeed||mods[0].speed) + ' MT/s' : ''}`.trim() : '';
  $('#hwMem').innerHTML = `<div class="bigrow"><div><small>En uso</small><b class="num">${fmtBytes(used,1)}<span>de ${fmtBytes(m.total,0)}</span></b></div>
    <div><small>Disponible</small><b class="num">${fmtBytes(m.avail,1)}</b></div><div><small>Carga</small><b class="num">${m.load}<span>%</span></b></div></div>
    <div class="segbar">${parts.map(([,v,c]) => `<i style="width:${(v/m.total*100).toFixed(2)}%;background:${c}"></i>`).join('')}</div>
    <div class="legend">${parts.map(([n,v,c]) => `<span style="--c:${c}">${n} <b class="num">${fmtBytes(v,1)}</b></span>`).join('')}</div>
    <div class="kv cols2"><div>Comprometida</div><div class="num">${fmtBytes(m.commit,1)} / ${fmtBytes(m.commitLimit,0)}</div><div>Caché del sistema</div><div class="num">${fmtBytes(m.cache,1)}</div>
      <div>Bloque paginado</div><div class="num">${fmtBytes(m.paged,0)}</div><div>Bloque no paginado</div><div class="num">${fmtBytes(m.nonpaged,0)}</div>
      ${m.installed ? `<div>Instalada</div><div class="num">${fmtBytes(m.installed,0)}</div>` : ''}<div>Fallos de página/s</div><div class="num">${fmtN(Math.round(m.faults))}</div></div>
    ${mods.length ? `<div class="sub-h">Módulos</div><table class="sitbl"><tr><th>Ranura</th><th>Capacidad</th><th>Tipo</th><th>Velocidad</th><th>Fabricante</th><th>Modelo</th></tr>
      ${mods.map(x => `<tr><td>${esc(x.slot)}</td><td class="num">${fmtBytes(x.size,0)}</td><td>${esc(x.type||'–')} ${esc(x.form||'')}</td><td class="num">${x.confSpeed||x.speed ? (x.confSpeed||x.speed)+' MT/s' : '–'}${x.confSpeed && x.speed && x.confSpeed !== x.speed ? ` <span class="faint">(máx. ${x.speed})</span>` : ''}</td><td>${esc(x.maker||'–')}</td><td>${esc(x.part||'–')}</td></tr>`).join('')}</table>` : ''}`;
  $('#hwPurge').style.display = d.admin ? '' : 'none';

  // ---- Discos
  $('#hwDisks').innerHTML = (d.disks||[]).map(k => `<div class="dsk"><div class="top">${ic('disk')}<b>${esc(k.model || 'Disco ' + k.index)}</b>
      ${k.bus ? `<span class="badge">${esc(k.bus)}</span>` : ''}${k.kind ? `<span class="badge b-app">${k.kind}</span>` : ''}${k.temp ? `<span class="badge b-${tempClass(k.temp)==='ok'?'security':tempClass(k.temp)==='warn'?'bloat':'system'}">${ic('thermo')}${fmtTemp(k.temp)}</span>` : ''}
      <span class="faint" style="margin-left:auto;font-size:12.5px">${fmtBytes(k.size,0)} ${(k.volumes||[]).length ? '· ' + k.volumes.join(' ') : ''}</span></div>
      <div class="bar"><i style="width:${(k.active||0).toFixed(0)}%"></i></div>
      <div class="meta"><span>Actividad <b class="num">${Math.round(k.active||0)} %</b></span><span>Lectura <b class="num">${fmtRate(k.read)}</b></span><span>Escritura <b class="num">${fmtRate(k.write)}</b></span></div></div>`).join('') || '<div class="faint">Sin discos.</div>';

  // ---- Sensores LHM
  if (lhm.length){
    $('#hwLhmState').innerHTML = `<span class="badge b-security">${ic('check')}LibreHardwareMonitor conectado</span>`;
    const byHw = {};
    lhm.forEach(x => { (byHw[x.hw] = byHw[x.hw] || {}); (byHw[x.hw][x.group] = byHw[x.hw][x.group] || []).push(x); });
    const gname = {Temperatures:'Temperaturas', Fans:'Ventiladores', Voltages:'Voltajes', Powers:'Consumo', Clocks:'Frecuencias', Load:'Carga', Data:'Datos', Throughput:'Transferencia', Controls:'Control', Levels:'Niveles', Factors:'Factores', Currents:'Intensidad', SmallData:'Datos', Energy:'Energía', Noise:'Ruido', Timing:'Tiempos'};
    $('#hwSensors').innerHTML = `<div class="sensors">${Object.entries(byHw).map(([hw, groups]) => `<div class="sgroup"><h4>${esc(hw)}</h4><table>
      ${Object.entries(groups).map(([gr, ss]) => `<tr><td colspan="3" class="gl">${esc(gname[gr]||gr)}</td></tr>${ss.map(x => `<tr><td>${esc(x.name)}</td><td class="v">${esc(x.value)}</td><td class="mm">${x.min||x.max ? esc(x.min) + ' – ' + esc(x.max) : ''}</td></tr>`).join('')}`).join('')}
      </table></div>`).join('')}</div>`;
  } else {
    $('#hwLhmState').innerHTML = `<span class="badge">No conectado</span>`;
    $('#hwSensors').innerHTML = `<div class="alert info">${ic('info')}<div><b>Ver temperatura del procesador, placa base, ventiladores y voltajes</b>
      Windows no permite leer esos sensores sin un controlador. Vantix puede mostrarlos usando <strong>LibreHardwareMonitor</strong> (gratuito y de código abierto):
      <ol style="margin:8px 0 4px;padding-left:18px"><li>Descárgalo desde <a href="https://github.com/LibreHardwareMonitor/LibreHardwareMonitor/releases/latest">GitHub</a> y ábrelo como administrador.</li>
      <li>Menú <em>Options › Remote Web Server › Run</em> (puerto ${CFG.lhmPort||8085}).</li><li>Vuelve aquí: los sensores aparecerán solos.</li></ol>
      ${(d.thermal||[]).length ? `<span class="faint">Zonas térmicas ACPI detectadas: ${d.thermal.map(t => `${esc(t.name)} ${fmtTemp(t.temp)}`).join(' · ')}</span>` : ''}</div></div>`;
  }
}
$('#hwPurge').onclick = async () => {
  try{ const r = await api('memory/purge', {}); if (r.error) return toast(r.error, 'warn'); toast('Memoria en espera liberada', 'ok', r.freed ? `Ahora hay ${fmtBytes(r.freed,0)} más de memoria libre` : ''); }
  catch(e){ toast('No se pudo liberar la memoria', 'bad'); }
};

// ================================================================ RED
const netHist = {d:[], u:[]};
let netPaused = false, netOpen = null;
PAGES.network = {
  interval: 1500,
  async poll(){ if (netPaused) return; renderNetwork(await api('network')); },
};
function netChart(){
  const cv = $('#nChart'), dpr = devicePixelRatio||1, w = cv.clientWidth, h = cv.clientHeight; if (!w) return;
  if (cv.width !== w*dpr){ cv.width = w*dpr; cv.height = h*dpr; }
  const c = cv.getContext('2d'); c.setTransform(dpr,0,0,dpr,0,0); c.clearRect(0,0,w,h);
  const max = Math.max(1024*50, ...netHist.d, ...netHist.u) * 1.15, N = 80, step = w/(N-1);
  const line = (arr, col) => { const off = N - arr.length; c.beginPath(); arr.forEach((v,i) => { const x=(i+off)*step, y=h-2-(v/max)*(h-6); i?c.lineTo(x,y):c.moveTo(x,y); }); c.strokeStyle = col; c.lineWidth = 1.6; c.stroke(); if (arr.length>1){ c.lineTo((N-1)*step,h); c.lineTo(off*step,h); c.closePath(); const g=c.createLinearGradient(0,0,0,h); g.addColorStop(0,col+'38'); g.addColorStop(1,col+'00'); c.fillStyle=g; c.fill(); } };
  line(netHist.d, '#3d8bfd'); line(netHist.u, '#2fb67c');
  c.fillStyle = '#6e7785'; c.font = '11px Segoe UI'; c.fillText(fmtRate(max/1.15), 4, 12);
}
function renderNetwork(d){
  const t = d.totals || {};
  netHist.d.push(t.down||0); netHist.u.push(t.up||0); if (netHist.d.length > 80){ netHist.d.shift(); netHist.u.shift(); }
  $('#nDown').textContent = fmtRate(t.down||0); $('#nUp').textContent = fmtRate(t.up||0);
  $('#nDownTot').textContent = `Total recibido: ${fmtBytes(t.totalIn||0,1)}`; $('#nUpTot').textContent = `Total enviado: ${fmtBytes(t.totalOut||0,1)}`;
  netChart();
  $('#netAdminNote').innerHTML = d.admin ? '' : `<div class="alert warn" style="margin-bottom:14px">${ic('shield')}<div style="flex:1"><b>Tráfico por programa</b>Para medir cuánto descarga y sube cada programa, Windows exige permisos de administrador. Sin ellos se muestran sus conexiones y la velocidad total.</div><button class="btn sm primary" onclick="elevate()">${ic('shield')}Ejecutar como administrador</button></div>`;
  const q = $('#netFilter').value.trim().toLowerCase();
  const procs = (d.procs||[]).filter(p => !q || (p.name+' '+p.exe).toLowerCase().includes(q));
  $('#netCount').textContent = `${procs.filter(p => p.est).length} con conexiones activas`;
  const rows = [];
  for (const p of procs){
    const key = p.exe + p.pid;
    rows.push(`<tr class="r${netOpen===key?' open':''}" data-k="${esc(key)}"><td><div class="pname"><div class="pi">${esc(initials(p.name))}</div><div class="tx"><b>${esc(p.name)}</b><small>${esc(p.exe)} · PID ${p.pid}</small></div></div></td>
      <td class="r-right num">${p.est}${p.listen||p.udp ? ` <span class="faint">+${p.listen+p.udp}</span>` : ''}</td>
      <td class="r-right num">${d.admin ? fmtRate(p.inRate) : '<span class="faint">—</span>'}</td><td class="r-right num">${d.admin ? fmtRate(p.outRate) : '<span class="faint">—</span>'}</td>
      <td class="r-right num">${d.admin ? fmtBytes(p.in,1) : '<span class="faint">—</span>'}</td><td class="r-right num">${d.admin ? fmtBytes(p.out,1) : '<span class="faint">—</span>'}</td></tr>`);
    if (netOpen === key) rows.push(`<tr class="det"><td colspan="6" style="padding-left:52px"><table class="sitbl"><tr><th>Protocolo</th><th>Local</th><th>Remoto</th><th>Estado</th></tr>${(p.conns||[]).map(c => `<tr><td>${c.proto}</td><td class="mono">${esc(c.local)}</td><td class="mono">${esc(c.remote||'—')}</td><td>${esc(c.state||'')}</td></tr>`).join('')}</table>${p.udp ? `<div class="faint" style="font-size:12px;margin-top:6px">${p.udp} puertos UDP abiertos</div>` : ''}</td></tr>`);
  }
  $('#netBody').innerHTML = rows.join('') || `<tr><td colspan="6" style="max-width:none"><div class="empty"><b>Sin conexiones</b></div></td></tr>`;
  $('#netIfaces').innerHTML = (t.ifaces||[]).map(i => `<tr><td style="width:40%"><div class="pname"><div class="pi">${ic(i.type===71?'wifi':'activity')}</div><div class="tx"><b>${esc(i.name)}</b><small>${esc(i.desc)}</small></div></div></td><td class="num">${i.speed ? fmtSpeed(i.speed) : ''}</td><td class="num">↓ ${fmtRate(i.inRate)}</td><td class="num">↑ ${fmtRate(i.outRate)}</td><td class="mono faint">${esc(i.mac||'')}</td></tr>`).join('') || '<tr><td class="faint" style="padding:12px 16px">Sin adaptadores conectados.</td></tr>';
}
function fmtSpeed(bps){ return bps >= 1e9 ? (bps/1e9).toFixed(bps%1e9?1:0).replace('.',',') + ' Gbps' : Math.round(bps/1e6) + ' Mbps'; }
$('#netBody').addEventListener('click', e => { const tr = e.target.closest('tr.r'); if (!tr) return; netOpen = netOpen === tr.dataset.k ? null : tr.dataset.k; PAGES.network.poll(); });
$('#netFilter').addEventListener('input', () => PAGES.network.poll());
$('#netPause').onclick = () => { netPaused = !netPaused; $('#netPause').innerHTML = netPaused ? ic('play')+'<span>Reanudar</span>' : ic('pause')+'<span>Pausar</span>'; };

// ================================================================ MODO JUEGO
let GAME = null;
PAGES.game = { async enter(){ await loadGame(); } };
async function loadGame(){
  try{ GAME = await api('game'); }catch(e){ return; }
  const g = GAME.config;
  const on = GAME.active;
  const plan = (GAME.plans||[]).find(p => p.active);
  $('#gameHero').className = 'card gamehero' + (on ? ' on' : '');
  $('#gameHero').innerHTML = `<div class="gi">${ic('gamepad')}</div><div><h2>${on ? 'Modo juego activado' : 'Modo juego desactivado'}</h2>
    <p>${on ? `Activo desde las ${new Date(GAME.since*1000).toLocaleTimeString('es-ES',{hour:'2-digit',minute:'2-digit'})}. Plan de energía: ${esc(plan?.name||'—')}.` : `Plan de energía actual: ${esc(plan?.name||'—')}.`}</p></div>
    <button class="btn ${on ? '' : 'primary'}" id="gToggle">${on ? 'Desactivar' : ic('zap') + 'Activar modo juego'}</button>`;
  $('#gToggle').onclick = toggleGame;
  $('#gCloseBloat').checked = g.closeBloat; $('#gPower').checked = g.power; $('#gStandby').checked = g.standby;
  $('#gStandby').disabled = !GAME.admin;
  $('#gStandby').closest('.srow').querySelector('span').innerHTML = 'Vacía la caché de memoria en espera para dejar más RAM libre al juego.' + (GAME.admin ? '' : ' <span class="badge b-bloat" style="margin-left:4px">Requiere administrador</span>');
  const plans = GAME.plans || [];
  $('#gPlan').innerHTML = `<option value="">Automático (el más rápido)</option>` + plans.map(p => `<option value="${p.guid}" ${g.planGuid===p.guid?'selected':''}>${esc(p.name)}</option>`).join('');
  $('#gUltimate').style.display = plans.some(p => p.kind === 'ultimate') ? 'none' : '';
  $('#gPlans').innerHTML = plans.map(p => `<div class="planrow"><span class="dot ${p.active?'ok':''}"></span><b>${esc(p.name)}</b>${p.active ? '<span class="badge b-security">Activo</span>' : `<button class="btn sm" data-plan="${p.guid}">Usar ahora</button>`}</div>`).join('') || '<div class="cb faint">No se pudieron leer los planes de energía.</div>';
  $('#gExtraTags').innerHTML = (g.extra||[]).length ? g.extra.map((x,i) => `<span class="tag">${esc(x)}<button data-gx="${i}">${ic('x')}</button></span>`).join('') : '<span class="faint" style="font-size:12.5px">Ninguno</span>';
  try{
    const pr = await api('processes');
    const opts = (pr.groups||[]).filter(x => !x.locked && (x.cat === 'background' || x.cat === 'app' || x.cat === 'hardware')).map(x => x.exe).filter((v,i,a) => a.indexOf(v) === i && !(g.extra||[]).includes(v)).sort((a,b) => a.localeCompare(b));
    $('#gExtraSel').innerHTML = `<option value="">Elige un programa en ejecución…</option>` + opts.map(o => `<option>${esc(o)}</option>`).join('');
  }catch(e){}
}
async function saveGame(){ CFG.game = GAME.config; await saveConfig(); }
['gCloseBloat','gPower','gStandby'].forEach(id => $('#'+id).onchange = e => { GAME.config[{gCloseBloat:'closeBloat',gPower:'power',gStandby:'standby'}[id]] = e.target.checked; saveGame(); });
$('#gPlan').onchange = e => { GAME.config.planGuid = e.target.value; saveGame(); };
$('#gExtraAdd').onclick = async () => { const v = $('#gExtraSel').value; if (!v) return; GAME.config.extra = [...(GAME.config.extra||[]), v]; await saveGame(); loadGame(); };
$('#gExtraTags').onclick = async e => { const b = e.target.closest('[data-gx]'); if (!b) return; GAME.config.extra.splice(+b.dataset.gx, 1); await saveGame(); loadGame(); };
$('#gPlans').onclick = async e => { const b = e.target.closest('[data-plan]'); if (!b) return; const r = await api('power/set', {guid:b.dataset.plan}); if (r.error) toast(r.error,'bad'); loadGame(); };
$('#gUltimate').onclick = async () => { const r = await api('power/ultimate', {}); if (r.error) toast(r.error, 'warn', STATUS.admin ? '' : 'Prueba a ejecutar la aplicación como administrador.'); else toast('Plan «Máximo rendimiento» añadido', 'ok'); loadGame(); };
async function toggleGame(){
  const b = $('#gToggle'); setBusy(b, true, GAME.active ? 'Desactivando…' : 'Activando…');
  try{
    if (GAME.active){ const r = await api('game/off', {}); if (r.error) toast(r.error, 'warn'); else toast('Modo juego desactivado', 'ok', 'Se ha restaurado tu plan de energía.'); }
    else {
      const r = await api('game/on', {});
      const parts = [];
      if (r.closed) parts.push(`${r.closed} procesos cerrados (${fmtBytes(r.freed,0)})`);
      if (r.plan) parts.push('plan de alto rendimiento');
      if (r.standby) parts.push('memoria en espera liberada');
      toast('Modo juego activado', 'ok', parts.join(' · ') || '');
      (r.notes||[]).forEach(n => toast(n, 'warn'));
    }
  }catch(e){ toast('Error: ' + e.message, 'bad'); }
  await loadGame();
}

// ================================================================ INFORMACIÓN DEL SISTEMA
let SYS = null;
PAGES.sysinfo = { async enter(){ if (!SYS) await loadSys(); } };
async function loadSys(){
  try{ SYS = await api('sysinfo'); }catch(e){ $('#siBody').innerHTML = '<div class="alert bad">No se pudo leer la información del sistema.</div>'; return; }
  const secs = sysSections(SYS);
  $('#siBody').innerHTML = secs.map(s => `<div class="card"${s.wide?' style="grid-column:1/-1"':''}><div class="ch"><h3>${esc(s.title)}</h3></div><div class="cb">
    ${s.kv && s.kv.length ? `<div class="kv">${s.kv.filter(([,v]) => v !== '' && v != null).map(([k,v]) => `<div>${esc(k)}</div><div>${esc(v)}</div>`).join('')}</div>` : ''}
    ${s.table ? `<table class="sitbl"><tr>${s.table.head.map(h => `<th>${esc(h)}</th>`).join('')}</tr>${s.table.rows.map(r => `<tr>${r.map(c => `<td>${esc(c)}</td>`).join('')}</tr>`).join('')}</table>` : ''}
    </div></div>`).join('');
}
function sysSections(d){
  const o = d.os||{}, c = d.cpu||{}, b = d.board||{}, m = d.memory||{};
  const S = [];
  S.push({title:'Sistema operativo', kv:[['Edición', o.name], ['Versión', o.displayVersion], ['Compilación', o.build], ['Arquitectura', o.arch], ['Instalado', o.installDate ? new Date(o.installDate*1000).toLocaleDateString('es-ES') : ''], ['Último arranque', o.bootTime ? new Date(o.bootTime*1000).toLocaleString('es-ES') : ''], ['Nombre del equipo', o.computer], ['Usuario', o.user]]});
  S.push({title:'Placa base y BIOS', kv:[['Fabricante', b.boardMaker], ['Modelo', b.boardModel], ['Revisión', b.boardVersion], ['Equipo', [b.sysMaker, b.sysModel].filter(x => x && !/to be filled|default string|system product/i.test(x)).join(' ')], ['BIOS', [b.biosVendor, b.biosVersion].filter(Boolean).join(' ')], ['Fecha BIOS', (b.biosDate||'').replace(/^(\d\d)\/(\d\d)\/(\d{4})$/, '$2/$1/$3')]]});
  S.push({title:'Procesador', kv:[['Modelo', c.name], ['Núcleos', c.cores], ['Hilos', c.threads], ['Velocidad base', c.baseMHz ? fmtMHz(c.baseMHz) : ''], ['Velocidad máxima (BIOS)', c.maxMHz ? fmtMHz(c.maxMHz) : ''], ['Zócalo', c.socket], ['Caché L2', c.l2 ? fmtBytes(c.l2,0) : ''], ['Caché L3', c.l3 ? fmtBytes(c.l3,0) : '']]});
  const mods = m.modules || [];
  S.push({title:'Memoria RAM', kv:[['Instalada', fmtBytes(m.installed || m.total, 0)], ['Utilizable', fmtBytes(m.total,1)], ['Ranuras', m.slots ? `${mods.length} de ${m.slots} ocupadas` : (mods.length ? mods.length + ' módulos' : '')], ['Tipo', mods[0] ? `${mods[0].type||''} ${mods[0].form||''}`.trim() : ''], ['Velocidad', mods[0] ? `${mods[0].confSpeed||mods[0].speed} MT/s` : '']],
    table: mods.length ? {head:['Ranura','Capacidad','Velocidad','Fabricante','Modelo'], rows: mods.map(x => [x.slot, fmtBytes(x.size,0), (x.confSpeed||x.speed) + ' MT/s', x.maker, x.part])} : null});
  S.push({title:'Gráficos', table:{head:['Tarjeta','Memoria','Controlador','Fecha'], rows:(d.gpus||[]).map(g => [g.name, g.vram ? fmtBytes(g.vram,0) : '', g.driver, g.date])}});
  S.push({title:'Pantallas', table:{head:['Monitor','Resolución','Frecuencia','Principal'], rows:(d.displays||[]).map(x => [x.name || x.adapter, `${x.w} × ${x.h}`, x.hz + ' Hz', x.primary ? 'Sí' : ''])}});
  S.push({title:'Almacenamiento', wide:true, table:{head:['Disco','Tipo','Conexión','Capacidad','Unidades','Temperatura'], rows:(d.disks||[]).map(k => [k.model, k.kind, k.bus, fmtBytes(k.size,0), (k.volumes||[]).join(' '), k.temp ? fmtTemp(k.temp) : ''])}});
  S.push({title:'Unidades', table:{head:['Unidad','Nombre','Libre','Total','Uso'], rows:(d.volumes||[]).map(v => [v.path, v.label, fmtBytes(v.free,1), fmtBytes(v.total,0), fmtPct(v.total ? (1-v.free/v.total)*100 : 0)])}});
  S.push({title:'Red', table:{head:['Adaptador','Direcciones','MAC'], rows:(d.network||[]).map(n => [n.name + (n.up ? '' : ' (desconectado)'), (n.addrs||[]).join(', '), n.mac])}});
  if (d.battery) S.push({title:'Batería', kv:[['Carga', d.battery.percent + ' %'], ['Estado', d.battery.charging ? 'Cargando' : d.battery.ac ? 'Conectado' : 'Con batería'], ['Tiempo restante', d.battery.seconds > 0 ? fmtUptime(d.battery.seconds) : '']]});
  return S;
}
function sysText(){
  const lines = ['INFORMACIÓN DEL SISTEMA — Vantix Utilities', 'Generado: ' + new Date().toLocaleString('es-ES'), ''];
  for (const s of sysSections(SYS)){
    lines.push('== ' + s.title.toUpperCase() + ' ==');
    (s.kv||[]).filter(([,v]) => v !== '' && v != null).forEach(([k,v]) => lines.push(`${k}: ${v}`));
    if (s.table){ s.table.rows.forEach(r => lines.push('- ' + r.filter(Boolean).join(' | '))); }
    lines.push('');
  }
  return lines.join('\r\n');
}
function download(name, content, type){
  const a = document.createElement('a'); a.href = URL.createObjectURL(new Blob([content], {type})); a.download = name; a.click(); setTimeout(() => URL.revokeObjectURL(a.href), 2000);
}
$('#siCopy').onclick = () => SYS && copyText(sysText(), 'Información copiada');
$('#siTxt').onclick = () => { if (!SYS) return; download(`Sistema_${SYS.os?.computer||'equipo'}.txt`, '﻿' + sysText(), 'text/plain;charset=utf-8'); toast('Informe exportado', 'ok'); };
$('#siHtml').onclick = () => {
  if (!SYS) return;
  const body = sysSections(SYS).map(s => `<section><h2>${esc(s.title)}</h2>${s.kv ? `<table class="kv">${s.kv.filter(([,v]) => v !== '' && v != null).map(([k,v]) => `<tr><th>${esc(k)}</th><td>${esc(v)}</td></tr>`).join('')}</table>` : ''}${s.table ? `<table><tr>${s.table.head.map(h => `<th>${esc(h)}</th>`).join('')}</tr>${s.table.rows.map(r => `<tr>${r.map(c => `<td>${esc(c)}</td>`).join('')}</tr>`).join('')}</table>` : ''}</section>`).join('');
  const html = `<!DOCTYPE html><html lang="es"><head><meta charset="utf-8"><title>Informe del sistema — ${esc(SYS.os?.computer||'')}</title><style>body{font:14px/1.5 "Segoe UI",system-ui,sans-serif;color:#1d2330;max-width:900px;margin:40px auto;padding:0 24px}h1{font-size:24px;margin:0}header p{color:#667;margin:4px 0 28px}h2{font-size:15px;border-bottom:2px solid #3d8bfd;padding-bottom:4px;margin:26px 0 10px}table{width:100%;border-collapse:collapse;font-size:13px}th,td{text-align:left;padding:6px 8px;border-bottom:1px solid #e5e8ee;vertical-align:top}th{color:#556;font-weight:600}table.kv th{width:220px}footer{margin-top:40px;color:#889;font-size:12px}</style></head><body><header><h1>Informe del sistema</h1><p>${esc(SYS.os?.computer||'')} · ${new Date().toLocaleString('es-ES')}</p></header>${body}<footer>Generado con Vantix Utilities · HECHO POR @BYALVARO__</footer></body></html>`;
  download(`Informe_${SYS.os?.computer||'equipo'}.html`, html, 'text/html;charset=utf-8'); toast('Informe exportado', 'ok', 'Puedes abrirlo en el navegador e imprimirlo como PDF.');
};
