// Key wiring: the same block goes into page.html (see the spec).
(function(){
  var fc = document.getElementById('fakechart');
  if(fc){ for(var i = 0; i < 90; i++){ var b = document.createElement('i'); b.style.height = (10 + Math.abs(Math.sin(i / 6)) * 70 + (i % 7) * 3) + '%'; fc.appendChild(b); } }
  var station = null, stationMode = null, panelSaved = null;
  function ensure(){
    if(!station) station = BMStation.create({atlases: {space: window.BM_STATION_ATLAS, site: window.BM_STATION_ATLAS_SITE},
      theme: /[?#&]theme=space/.test(location.href) ? 'space' : (window.BM_STATION_THEME || 'site'),
      onAgentClick: function(sid){ console.log('station click', sid); }});
    return station;
  }
  function close(){
    if(!station || !stationMode) return;
    station.unmount();
    if(stationMode === 'full') document.getElementById('stationOverlay').classList.remove('on');
    if(stationMode === 'panel' && panelSaved){ panelSaved.host.removeChild(panelSaved.box); panelSaved.kids.forEach(function(k){ k.style.display = ''; }); panelSaved = null; }
    stationMode = null;
  }
  function open(m){
    close();
    var st = ensure();
    if(m === 'full'){
      var ov = document.getElementById('stationOverlay'); ov.classList.add('on'); st.mount(ov, 'full');
    } else {
      var host = document.getElementById('burnBody');
      var kids = Array.prototype.slice.call(host.children);
      kids.forEach(function(k){ k.style.display = 'none'; });
      var box = document.createElement('div'); box.style.cssText = 'position:absolute;inset:0';
      host.appendChild(box); panelSaved = {host: host, box: box, kids: kids};
      st.mount(box, 'panel');
    }
    stationMode = m;
    st.update(window.fakeModel());
  }
  document.addEventListener('keydown', function(e){
    var t = e.target, tag = t && t.tagName;
    if(tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT' || (t && t.isContentEditable)) return;
    if(e.ctrlKey || e.altKey || e.metaKey) return;
    var k = (e.key || '').toLowerCase();
    if(k === 'p'){ stationMode === 'full' ? close() : open('full'); e.preventDefault(); }
    else if(k === 'o'){ stationMode === 'panel' ? close() : open('panel'); e.preventDefault(); }
    else if(k === 't' && stationMode){ station.setTheme(station.theme() === 'site' ? 'space' : 'site'); e.preventDefault(); }
    else if((k === 'escape' || k === 'esc') && stationMode === 'full'){ close(); }
  });
  setInterval(function(){ if(station && stationMode) station.update(window.fakeModel()); }, 1000);
  // preview only: open the full Station on load when the URL says so
  if(/[?#&]station=full/.test(location.href) || window.BMS_AUTOOPEN === 'full') open('full');
  if(/[?#&]station=panel/.test(location.href) || window.BMS_AUTOOPEN === 'panel') open('panel');
})();
