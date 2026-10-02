// BurnMon Dev Station: the secret isometric screen (P full window, O in the
// burn zone). Spec: 02_roadmap/2026-10-01_station_secret_screen.md.
// Art: Kenney Space Kit 2.0 (CC0) via station_atlas.js. Everything else
// (deck, glow, holograms, stars) is drawn here. No dependencies.
//
// API:
//   var st = BMStation.create({atlas: window.BM_STATION_ATLAS,
//                              onAgentClick: function(sid){...}});
//   st.mount(el, 'full' | 'panel');   st.unmount();
//   st.update({now: ms, sessions: [{id, parent, agent, color, project,
//              model, stage, since, tool, ctx, tokens}]});
//   BMStation.fromSnapshot(snap, colorFn) turns a bmLive snapshot into
//   that update shape.
(function(){
  'use strict';

  // Native tile size in atlas pixels (the kit's 2:1 view), and the deck's
  // tile axes on screen at 1x as the atlas's sprites were rendered:
  // [x to sx, y to sx, x to sy, y to sy]. The kit's own view is
  // [65, -65, 32.5, 32.5]; since 2026-10-01 the Station turns the deck 15
  // degrees (tools/station_atlas/render_iso.py, azimuth 30: Wilco, the left
  // corner closer and the long edges flatter), elevation still 30.
  var TW = 130, TH = 65;
  var PROJ = (window.BM_STATION_ATLAS && window.BM_STATION_ATLAS.proj) || [TW / 2, -TW / 2, TH / 2, TH / 2];
  var PA = PROJ[0], PB = PROJ[1], PC = PROJ[2], PD = PROJ[3];
  // Draw order: a tile's screen depth, scaled to equal x + y at the kit's view.
  function dep(x, y){ return (x * PC + y * PD) * 2 / (PC + PD); }
  // Deck: a 4 x 3 grid of 7 x 7 rooms with 1-tile corridors between them.
  var RW = 7, CW = 1, COLS = 4, ROWS = 3;
  var GX = COLS * RW + (COLS - 1) * CW, GY = ROWS * RW + (ROWS - 1) * CW;
  var AGENT_SCALE = 1.5, SUB_SCALE = 1.1;
  var SLAB = 34;                  // deck edge thickness, native px
  var MIN_DWELL = 3500;           // ms an agent stays in a room before it moves on
  // Frame rates: fast while anything walks, fades or is dragged or zoomed,
  // slow while everyone stands in a room (see loop()).
  var BUSY_MS = 1000 / 15, IDLE_MS = 1000 / 8;

  // ---- stages and rooms ---------------------------------------------------
  var STAGES = {
    arriving:   {room: 'airlock',  label: 'Arriving',      color: '#cbd5e1'},
    reading:    {room: 'archive',  label: 'Reading',       color: '#60a5fa'},
    fetching:   {room: 'uplink',   label: 'Getting data',  color: '#2dd4bf'},
    planning:   {room: 'plan',     label: 'Planning',      color: '#a3e635'},
    coding:     {room: 'fab',      label: 'Coding',        color: '#f59e0b'},
    running:    {room: 'test',     label: 'Running checks',color: '#34d399'},
    thinking:   {room: 'think',    label: 'Thinking',      color: '#c084fc'},
    delegating: {room: 'brief',    label: 'Delegating',    color: '#f472b6'},
    waiting:    {room: 'lounge',   label: 'Waiting on you',color: '#fb923c'},
    dormant:    {room: 'cryo',     label: 'Dormant',       color: '#7dd3fc'},
    compacting: {room: 'recycler', label: 'Compacting',    color: '#facc15'}
  };
  var STAGE_ORDER = ['arriving','reading','fetching','planning','coding','running',
                     'thinking','delegating','waiting','compacting','dormant'];

  // Rooms by grid cell. Each template lists props as [sprite, lx, ly, blocks, scale]
  // and slots as [lx, ly, face?] in room-local tiles (0..6). Agents face the
  // room focus unless the slot names a face.
  var LAYOUT = [
    ['archive', 'uplink', 'plan', 'airlock'],
    ['fab', 'think', 'test', 'brief'],
    ['lounge', 'recycler', 'cryo', 'core']
  ];
  var TEMPLATES = {
    archive: {name: 'ARCHIVE', hue: '#60a5fa', focus: [3.5, 0.5],
      props: [['structure_closed_SE',1,1,1,1.05],['structure_closed_SE',2,1,1,1.05],['structure_closed_SE',4,1,1,1.05],['structure_closed_SE',5,1,1,1.05],
              ['structure_detailed_SE',1,4,1,1.05],['structure_detailed_SE',2,4,1,1.05],['structure_detailed_SE',4,4,1,1.05],['structure_detailed_SE',5,4,1,1.05],
              ['desk_computerScreen_SW',3,0,1],['barrel_SE',0,6,1],['barrel_SE',6,6,1]],
      slots: [[1,2,'NE'],[2,2,'NE'],[4,2,'NE'],[5,2,'NE'],[1,5,'NE'],[2,5,'NE'],[4,5,'NE'],[5,5,'NE'],[3,1,'NE'],[3,4]]},
    uplink: {name: 'UPLINK', hue: '#2dd4bf', focus: [3.5, 2.5],
      props: [['satelliteDish_large_SW',3,2,1,2.3],['satelliteDish_SW',1,0,1,1.7],['satelliteDish_SW',5,0,1,1.7],
              ['machine_wireless_SW',0,2,1],['machine_wireless_SW',6,2,1],
              ['desk_computer_SW',1,5,1],['desk_computer_SW',3,5,1],['desk_computer_SW',5,5,1],
              ['desk_chair_NE',1,6,0],['desk_chair_NE',3,6,0],['desk_chair_NE',5,6,0]],
      slots: [[1,6,'NE'],[3,6,'NE'],[5,6,'NE'],[2,3],[4,3],[1,3],[5,3]]},
    plan: {name: 'PLAN TABLE', hue: '#a3e635', focus: [3.5, 3.5],
      props: [['platform_low_SE',2,2,1],['platform_low_SE',3,2,1],['platform_low_SE',4,2,1],
              ['platform_low_SE',2,3,1],['platform_low_SE',3,3,1],['platform_low_SE',4,3,1],
              ['platform_low_SE',2,4,1],['platform_low_SE',3,4,1],['platform_low_SE',4,4,1],
              ['desk_computerScreen_SW',1,0,1],['desk_computerScreen_SW',5,0,1],['desk_computerScreen_SW',3,0,1]],
      slots: [[1,2,'SE'],[1,3,'SE'],[1,4,'SE'],[5,2,'NW'],[5,3,'NW'],[5,4,'NW'],[2,1,'SW'],[3,1,'SW'],[4,1,'SW'],[2,5,'NE'],[3,5,'NE'],[4,5,'NE']]},
    airlock: {name: 'AIRLOCK', hue: '#cbd5e1', focus: [3.5, 0.5],
      props: [['gate_complex_SW',3,0,1,1.9],['barrels_SE',0,1,1],['barrels_SE',6,1,1],['barrel_SE',0,5,1],['barrel_SE',6,5,1],
              ['machine_generator_SE',0,3,1],['machine_generator_SW',6,3,1]],
      slots: [[3,1,'SW'],[2,2],[4,2],[3,3],[2,4],[4,4],[3,5],[1,6],[5,6]]},
    fab: {name: 'FABRICATOR', hue: '#f59e0b', focus: [3.5, 0.5],
      props: [['desk_computer_SW',1,1,1],['desk_computer_SW',3,1,1],['desk_computer_SW',5,1,1],
              ['desk_computer_SW',1,4,1],['desk_computer_SW',3,4,1],['desk_computer_SW',5,4,1],
              ['desk_chair_NE',1,2,0],['desk_chair_NE',3,2,0],['desk_chair_NE',5,2,0],
              ['desk_chair_NE',1,5,0],['desk_chair_NE',3,5,0],['desk_chair_NE',5,5,0],
              ['desk_computerCorner_SE',0,0,1],['desk_computerCorner_SW',6,0,1]],
      slots: [[1,2,'NE'],[3,2,'NE'],[5,2,'NE'],[1,5,'NE'],[3,5,'NE'],[5,5,'NE'],[0,3],[6,3],[3,6]]},
    think: {name: 'THINK PODS', hue: '#c084fc', focus: [3.5, 3.5],
      props: [['pipe_ringSupport_SE',3,3,1,1.7],
              ['desk_chairArms_SE',1,1,0],['desk_chairArms_SW',5,1,0],['desk_chairArms_NE',1,5,0],['desk_chairArms_NW',5,5,0],
              ['desk_chairArms_SE',1,3,0],['desk_chairArms_NW',5,3,0],['desk_chairArms_SW',3,1,0],['desk_chairArms_NE',3,5,0]],
      slots: [[1,1],[5,1],[1,5],[5,5],[1,3],[5,3],[3,1],[3,5],[2,2],[4,4]]},
    test: {name: 'TEST CHAMBER', hue: '#34d399', focus: [3.5, 2.5],
      props: [['gate_simple_SW',1,2,1,1.6],['gate_simple_SW',3,2,1,1.6],['gate_simple_SW',5,2,1,1.6],
              ['machine_generator_SE',0,5,1],['machine_generator_SW',6,5,1],
              ['desk_computerScreen_SW',1,0,1],['desk_computerScreen_SW',3,0,1],['desk_computerScreen_SW',5,0,1]],
      slots: [[1,3,'NE'],[3,3,'NE'],[5,3,'NE'],[2,4,'NE'],[4,4,'NE'],[3,5],[1,5],[5,5]]},
    brief: {name: 'BRIEFING', hue: '#f472b6', focus: [3.5, 3.5],
      props: [['platform_small_SE',2,3,1],['platform_small_SE',3,3,1],['platform_small_SE',4,3,1],
              ['desk_chair_SW',2,2,0],['desk_chair_SW',3,2,0],['desk_chair_SW',4,2,0],
              ['desk_chair_NE',2,4,0],['desk_chair_NE',3,4,0],['desk_chair_NE',4,4,0],
              ['desk_chair_SE',1,3,0],['desk_chair_NW',5,3,0],
              ['desk_computerScreen_SW',3,0,1,1.3],['machine_wireless_SW',0,0,1],['machine_wireless_SW',6,0,1]],
      slots: [[2,2],[3,2],[4,2],[2,4],[3,4],[4,4],[1,3],[5,3],[3,6]]},
    lounge: {name: 'LOUNGE', hue: '#fb923c', focus: [3.5, 3.5],
      props: [['platform_small_SE',1,2,1],['desk_chairArms_SE',0,2,0],['desk_chairArms_NW',2,2,0],['desk_chairArms_SW',1,1,0],['desk_chairArms_NE',1,3,0],
              ['platform_small_SE',5,2,1],['desk_chairArms_SE',4,2,0],['desk_chairArms_NW',6,2,0],['desk_chairArms_SW',5,1,0],['desk_chairArms_NE',5,3,0],
              ['platform_small_SE',3,5,1],['desk_chairArms_SE',2,5,0],['desk_chairArms_NW',4,5,0],['desk_chairArms_NE',3,6,0],
              ['machine_barrel_SE',3,0,1],['barrels_SE',0,6,1],['barrels_SE',6,6,1]],
      slots: [[0,2],[2,2],[1,1],[1,3],[4,2],[6,2],[5,1],[5,3],[2,5],[4,5],[3,6],[3,3]]},
    recycler: {name: 'RECYCLER', hue: '#facc15', focus: [3.5, 3.5],
      props: [['machine_generatorLarge_SE',3,3,1,1.25],['machine_barrel_SE',1,1,1],['machine_barrel_SW',5,1,1],
              ['machine_barrel_SE',1,5,1],['machine_barrel_SW',5,5,1],['barrel_SE',3,0,1],['barrel_SE',0,3,1]],
      slots: [[3,1],[1,3],[5,3],[3,5],[2,2],[4,4]]},
    cryo: {name: 'CRYO BAY', hue: '#7dd3fc', focus: [3.5, 0.5],
      props: [['machine_barrelLarge_SW',1,1,1,1.3],['machine_barrelLarge_SW',3,1,1,1.3],['machine_barrelLarge_SW',5,1,1,1.3],
              ['machine_barrelLarge_SW',1,4,1,1.3],['machine_barrelLarge_SW',3,4,1,1.3],['machine_barrelLarge_SW',5,4,1,1.3],
              ['machine_generator_SE',0,6,1],['machine_generator_SW',6,6,1]],
      slots: [[1,2,'NE'],[3,2,'NE'],[5,2,'NE'],[1,5,'NE'],[3,5,'NE'],[5,5,'NE'],[2,6],[4,6]]},
    core: {name: 'CORE', hue: '#38bdf8', focus: [3.5, 3.5],
      props: [['structure_detailed_SE',3,3,1,1.5],['supports_low_SE',1,1,1,1.3],['supports_low_SE',5,1,1,1.3],
              ['supports_low_SE',1,5,1,1.3],['supports_low_SE',5,5,1,1.3],['machine_generator_SE',0,3,1],['machine_generator_SW',6,3,1],
              ['machine_barrel_SE',3,0,1],['machine_barrel_SW',3,6,1]],
      slots: []}
  };

  // Every room has a door in the middle of each side that faces a corridor;
  // agents enter and leave rooms only through doors.
  var ROOMS = {}, PROPS = [], SLOTS = {}, DOORS = {}, ROOM_IDS = [], PLANTS = [];
  var roomAt = new Int8Array(GX * GY).fill(-1), doorAt = new Uint8Array(GX * GY);
  LAYOUT.forEach(function(row, ry){
    row.forEach(function(id, cx){
      var T = TEMPLATES[id], x0 = cx * (RW + CW), y0 = ry * (RW + CW), ri = ROOM_IDS.length;
      ROOM_IDS.push(id);
      ROOMS[id] = {name: T.name, r: [x0, y0, x0 + RW, y0 + RW], hue: T.hue, focus: [x0 + T.focus[0], y0 + T.focus[1]], x0: x0, y0: y0};
      var D = DOORS[id] = {top: ry > 0, bottom: ry < ROWS - 1, left: cx > 0, right: cx < COLS - 1};
      var isDoor = function(lx, ly){ return (D.top && ly === 0 && lx === 3) || (D.bottom && ly === RW - 1 && lx === 3) || (D.left && lx === 0 && ly === 3) || (D.right && lx === RW - 1 && ly === 3); };
      for(var ly = 0; ly < RW; ly++) for(var lx = 0; lx < RW; lx++){ roomAt[(y0 + ly) * GX + x0 + lx] = ri; if(isDoor(lx, ly)) doorAt[(y0 + ly) * GX + x0 + lx] = 1; }
      T.props.forEach(function(p){ if(!isDoor(p[1], p[2])) PROPS.push([p[0], x0 + p[1], y0 + p[2], p[3], p[4]]); });
      SLOTS[id] = T.slots.map(function(sl){ return [x0 + sl[0], y0 + sl[1], sl[2]]; });
      // plants in free room corners (plan view only)
      [[0, RW - 1], [RW - 1, RW - 1]].forEach(function(cn){
        var taken = T.props.some(function(p){ return p[1] === cn[0] && p[2] === cn[1]; }) || T.slots.some(function(sl){ return sl[0] === cn[0] && sl[1] === cn[1]; });
        if(!taken && id !== 'core') PLANTS.push(['plant', x0 + cn[0], y0 + cn[1], 0]);
      });
    });
  });
  // Pylons where corridors cross: iso view only, they never block (index 5 = iso only).
  for(var pc = 1; pc < COLS; pc++) for(var pr = 1; pr < ROWS; pr++)
    PROPS.push(['structure_SE', pc * (RW + CW) - 1, pr * (RW + CW) - 1, 0, 0.9, 'iso']);
  function roomXY(id, lx, ly){ return [ROOMS[id].x0 + lx, ROOMS[id].y0 + ly]; }
  var AIRLOCK_DOOR = [ROOMS.airlock.x0 + 3.5, 0.9];

  var VENDOR_SPRITE = {
    'claude-code': 'astronautA', 'cowork': 'astronautA', 'codex': 'astronautB',
    'copilot-cli': 'alien', 'copilot-vscode': 'alien', 'hermes': 'alien'
  };
  var AGENT_LABEL = {
    'claude-code': 'Claude Code', 'cowork': 'Cowork', 'codex': 'Codex',
    'copilot-cli': 'Copilot CLI', 'copilot-vscode': 'Copilot VS Code', 'hermes': 'Hermes'
  };

  // ---- small helpers ------------------------------------------------------
  function clamp(v, a, b){ return v < a ? a : v > b ? b : v; }
  function hexA(hex, a){
    var n = parseInt(hex.slice(1), 16);
    return 'rgba(' + (n >> 16 & 255) + ',' + (n >> 8 & 255) + ',' + (n & 255) + ',' + a + ')';
  }
  function hash(s){ var h = 2166136261; for(var i = 0; i < s.length; i++){ h ^= s.charCodeAt(i); h = Math.imul(h, 16777619); } return h >>> 0; }
  function rng(seed){ return function(){ seed = (seed * 1664525 + 1013904223) >>> 0; return seed / 4294967296; }; }
  function shortProject(p){
    if(!p) return 'no project';
    var parts = String(p).replace(/\\/g, '/').split('/').filter(Boolean);
    return parts.length ? parts[parts.length - 1] : p;
  }
  function fmtTokens(n){
    if(!n) return '0';
    if(n >= 1e9) return (n / 1e9).toFixed(1) + 'B';
    if(n >= 1e6) return (n / 1e6).toFixed(1) + 'M';
    if(n >= 1e3) return (n / 1e3).toFixed(0) + 'k';
    return String(n);
  }
  function fmtAgo(ms){
    var s = Math.max(0, Math.round(ms / 1000));
    if(s < 60) return s + 's';
    if(s < 3600) return Math.floor(s / 60) + 'm ' + (s % 60) + 's';
    return Math.floor(s / 3600) + 'h ' + Math.floor(s % 3600 / 60) + 'm';
  }
  function esc(s){ return String(s == null ? '' : s).replace(/[&<>"]/g, function(c){ return {'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;'}[c]; }); }

  // 8-way sprite suffix from a tile-space direction.
  var DIR8 = ['SE','S','SW','W','NW','N','NE','E'];
  function dirOf(dx, dy){
    var a = Math.atan2(dy, dx);
    var i = Math.round(a / (Math.PI / 4));
    return DIR8[(i + 8) % 8];
  }

  // ---- grid and paths -----------------------------------------------------
  var blocked = new Uint8Array(GX * GY);
  PROPS.forEach(function(p){ if(p[3]) blocked[p[2] * GX + p[1]] = 1; });
  function walkable(x, y){ return x >= 0 && y >= 0 && x < GX && y < GY && !blocked[y * GX + x]; }

  // A* over tiles, 8-way, no corner cutting. Start/goal are always allowed.
  function findPath(sx, sy, gx, gy){
    sx = clamp(Math.floor(sx), 0, GX - 1); sy = clamp(Math.floor(sy), 0, GY - 1);
    gx = clamp(Math.floor(gx), 0, GX - 1); gy = clamp(Math.floor(gy), 0, GY - 1);
    var N = GX * GY, g = new Float32Array(N).fill(Infinity), came = new Int32Array(N).fill(-1);
    var open = [sy * GX + sx], inOpen = new Uint8Array(N), goal = gy * GX + gx;
    g[open[0]] = 0; inOpen[open[0]] = 1;
    function h(i){ var x = i % GX, y = (i / GX) | 0, dx = Math.abs(x - gx), dy = Math.abs(y - gy); return Math.max(dx, dy) + 0.414 * Math.min(dx, dy); }
    var guard = 0;
    while(open.length && guard++ < 4000){
      var bi = 0, bf = Infinity;
      for(var k = 0; k < open.length; k++){ var f = g[open[k]] + h(open[k]); if(f < bf){ bf = f; bi = k; } }
      var cur = open.splice(bi, 1)[0]; inOpen[cur] = 0;
      if(cur === goal) break;
      var cx = cur % GX, cy = (cur / GX) | 0;
      for(var dy = -1; dy <= 1; dy++) for(var dx = -1; dx <= 1; dx++){
        if(!dx && !dy) continue;
        var nx = cx + dx, ny = cy + dy, ni = ny * GX + nx;
        if(nx < 0 || ny < 0 || nx >= GX || ny >= GY) continue;
        if(ni !== goal && !walkable(nx, ny)) continue;
        if(dx && dy && (!walkable(cx + dx, cy) || !walkable(cx, cy + dy))) continue;
        var ra = roomAt[cur], rb = roomAt[ni];
        if(ra !== rb){
          if(dx && dy) continue;                       // through a door only straight on
          if(ra >= 0 && !doorAt[cur]) continue;
          if(rb >= 0 && !doorAt[ni]) continue;
        }
        var ng = g[cur] + (dx && dy ? 1.414 : 1);
        if(ng < g[ni]){ g[ni] = ng; came[ni] = cur; if(!inOpen[ni]){ open.push(ni); inOpen[ni] = 1; } }
      }
    }
    if(came[goal] === -1 && goal !== sy * GX + sx) return [[gx + 0.5, gy + 0.5]];
    var path = [], c = goal;
    while(c !== -1 && c !== sy * GX + sx){ path.push([c % GX + 0.5, ((c / GX) | 0) + 0.5]); c = came[c]; }
    return path.reverse();
  }

  // ---- the station instance -----------------------------------------------
  function create(opts){
    opts = opts || {};
    var atlas = opts.atlas || window.BM_STATION_ATLAS;
    var MAP = atlas ? atlas.map : {};
    var img = new Image(), imgNight = null, ready = false;
    img.onload = function(){ imgNight = tint(img, '#8f9fc4'); ready = true; dirtyFloor = true; };
    if(atlas) img.src = atlas.png;

    var view = 'iso'; // 'iso' (full window) or 'top' (plan view, burn zone)
    var host = null, mode = 'full', canvas = null, ctx = null, hud = null, tip = null;
    var dpr = 1, W = 0, H = 0, raf = 0, timer = 0, kickUntil = 0, lastFrame = 0, ro = null;
    var cam = {s: 0.5, ox: 0, oy: 0, user: false};
    var floorCv = null, dirtyFloor = true, starCv = null;
    // Static walls and prop sprites, baked once per camera like the floor
    // (2026-10-01: drawing all of them every frame cost about 36 percent of
    // a core in WebView2, measured; uicheck d23, STATUS alpha.9).
    var propCv = null, propItems = [], dirtyProps = true;
    var agents = {}, order = [], slotTaken = {}, first = true, hoverId = null, placed = [];
    var lastModelAt = 0, modelNow = 0, clockSkew = 0;
    var rovers = [{x: ROOMS.lounge.x0 + 3.5, y: ROOMS.lounge.y0 + 4.5, path: [], face: 'SE', next: 0, area: 'lounge'},
                  {x: RW + 0.5, y: RW + 0.5, path: [], face: 'SE', next: 1500, area: null}];
    var burn = {last: 0, at: 0, rate: 0};
    var drag = null;

    function tint(src, color){
      var c = document.createElement('canvas'); c.width = src.width; c.height = src.height;
      var x = c.getContext('2d');
      x.drawImage(src, 0, 0);
      x.globalCompositeOperation = 'multiply'; x.fillStyle = color; x.fillRect(0, 0, c.width, c.height);
      x.globalCompositeOperation = 'destination-in'; x.drawImage(src, 0, 0);
      return c;
    }

    // world (tile units) to screen (css px)
    function sx(x, y){ return view === 'top' ? cam.ox + (x + 1) * 16 * cam.s : cam.ox + (x * PA + y * PB) * cam.s; }
    function sy(x, y){ return view === 'top' ? cam.oy + (y + 1) * 16 * cam.s : cam.oy + (x * PC + y * PD) * cam.s; }

    function fit(){
      if(view === 'top'){
        var top = 44; // room for the HUD strip above the deck
        var pw = (GX + 2) * 16, ph = (GY + 2) * 16;
        cam.s = Math.min(W * 0.98 / pw, (H - top - 4) / ph);
        cam.ox = Math.round((W - pw * cam.s) / 2); cam.oy = Math.round(top + (H - top - 4 - ph * cam.s) / 2);
        cam.user = false; dirtyFloor = true; return;
      }
      // the deck's four corners on screen at 1x
      var cx = [0, GX * PA, GY * PB, GX * PA + GY * PB], cy = [0, GX * PC, GY * PD, GX * PC + GY * PD];
      var x0 = Math.min.apply(null, cx), x1 = Math.max.apply(null, cx);
      var y0 = Math.min.apply(null, cy), y1 = Math.max.apply(null, cy);
      var worldW = x1 - x0, worldH = y1 - y0 + SLAB + 110;
      var pad = mode === 'full' ? 0.92 : 0.98;
      cam.s = Math.min(W * pad / worldW, H * pad / worldH);
      cam.ox = W / 2 - (x0 + x1) / 2 * cam.s;
      cam.oy = (H - (worldH - 110) * cam.s) / 2 + 55 * cam.s - y0 * cam.s;
      cam.user = false; dirtyFloor = true;
    }

    function resize(){
      if(!host) return;
      var r = host.getBoundingClientRect();
      W = Math.max(50, r.width); H = Math.max(50, r.height);
      dpr = window.devicePixelRatio || 1;
      canvas.width = Math.round(W * dpr); canvas.height = Math.round(H * dpr);
      canvas.style.width = W + 'px'; canvas.style.height = H + 'px';
      if(!cam.user) fit();
      buildStars(); dirtyFloor = true;
    }

    function buildStars(){
      starCv = document.createElement('canvas');
      starCv.width = canvas.width; starCv.height = canvas.height;
      var x = starCv.getContext('2d'), r = rng(7);
      var g = x.createRadialGradient(starCv.width * 0.5, starCv.height * 0.45, 0, starCv.width * 0.5, starCv.height * 0.45, starCv.width * 0.75);
      g.addColorStop(0, '#0b1424'); g.addColorStop(1, '#03050a');
      x.fillStyle = g; x.fillRect(0, 0, starCv.width, starCv.height);
      // faint nebula
      for(var n = 0; n < 3; n++){
        var nx = r() * starCv.width, ny = r() * starCv.height, nr = (0.25 + r() * 0.3) * starCv.width;
        var ng = x.createRadialGradient(nx, ny, 0, nx, ny, nr);
        ng.addColorStop(0, ['rgba(120,60,200,0.10)', 'rgba(30,120,200,0.09)', 'rgba(200,80,120,0.06)'][n]);
        ng.addColorStop(1, 'rgba(0,0,0,0)');
        x.fillStyle = ng; x.fillRect(0, 0, starCv.width, starCv.height);
      }
      var count = Math.round(starCv.width * starCv.height / 2600);
      for(var i = 0; i < count; i++){
        var a = r();
        x.fillStyle = 'rgba(220,230,255,' + (0.15 + a * 0.6) + ')';
        var sz = a > 0.97 ? 2 * dpr : dpr;
        x.fillRect(r() * starCv.width, r() * starCv.height, sz, sz);
      }
      // planet, bottom left, behind the deck
      var px = starCv.width * 0.12, py = starCv.height * 0.86, pr = Math.min(starCv.width, starCv.height) * 0.22;
      var pg = x.createRadialGradient(px - pr * 0.35, py - pr * 0.4, pr * 0.1, px, py, pr);
      pg.addColorStop(0, '#3b5f8f'); pg.addColorStop(0.6, '#1c2f52'); pg.addColorStop(1, '#0a1222');
      x.fillStyle = pg; x.beginPath(); x.arc(px, py, pr, 0, Math.PI * 2); x.fill();
      x.strokeStyle = 'rgba(125,211,252,0.25)'; x.lineWidth = 2 * dpr;
      x.beginPath(); x.arc(px, py, pr + 1.5 * dpr, Math.PI * 1.05, Math.PI * 1.75); x.stroke();
      x.strokeStyle = 'rgba(180,200,255,0.18)'; x.lineWidth = 3 * dpr;
      x.beginPath(); x.ellipse(px, py, pr * 1.7, pr * 0.32, -0.25, 0, Math.PI * 2); x.stroke();
    }

    // Floor and deck slab, baked once per camera scale.
    function buildFloor(){
      dirtyFloor = false; dirtyProps = true;
      floorCv = document.createElement('canvas');
      floorCv.width = canvas.width; floorCv.height = canvas.height;
      var x = floorCv.getContext('2d'); x.scale(dpr, dpr);
      var s = cam.s;
      // slab sides (front-left and front-right faces)
      var L = [sx(0, GY), sy(0, GY)], B = [sx(GX, GY), sy(GX, GY)], R = [sx(GX, 0), sy(GX, 0)];
      var d = SLAB * s;
      x.fillStyle = '#0a1018';
      x.beginPath(); x.moveTo(L[0], L[1]); x.lineTo(B[0], B[1]); x.lineTo(B[0], B[1] + d); x.lineTo(L[0], L[1] + d); x.closePath(); x.fill();
      x.fillStyle = '#070b12';
      x.beginPath(); x.moveTo(B[0], B[1]); x.lineTo(R[0], R[1]); x.lineTo(R[0], R[1] + d); x.lineTo(B[0], B[1] + d); x.closePath(); x.fill();
      // neon strip along the slab
      x.strokeStyle = 'rgba(56,189,248,0.75)'; x.lineWidth = Math.max(1, 2 * s);
      x.shadowColor = '#38bdf8'; x.shadowBlur = 10 * s;
      x.beginPath(); x.moveTo(L[0], L[1] + d * 0.55); x.lineTo(B[0], B[1] + d * 0.55); x.lineTo(R[0], R[1] + d * 0.55); x.stroke();
      x.shadowBlur = 0;
      // engine glow under the deck
      var eg = x.createRadialGradient(B[0], B[1] + d * 2, 0, B[0], B[1] + d * 2, 260 * s);
      eg.addColorStop(0, 'rgba(56,189,248,0.22)'); eg.addColorStop(1, 'rgba(56,189,248,0)');
      x.fillStyle = eg; x.fillRect(B[0] - 300 * s, B[1], 600 * s, 320 * s);
      // tiles
      for(var ty = 0; ty < GY; ty++) for(var tx = 0; tx < GX; tx++){
        x.fillStyle = (tx + ty) % 2 ? '#0d1420' : '#0f1724';
        diamond(x, tx, ty); x.fill();
      }
      // grid lines
      x.strokeStyle = 'rgba(80,110,150,0.18)'; x.lineWidth = Math.max(0.5, s);
      for(var i = 0; i <= GX; i++){ x.beginPath(); x.moveTo(sx(i, 0), sy(i, 0)); x.lineTo(sx(i, GY), sy(i, GY)); x.stroke(); }
      for(var j = 0; j <= GY; j++){ x.beginPath(); x.moveTo(sx(0, j), sy(0, j)); x.lineTo(sx(GX, j), sy(GX, j)); x.stroke(); }
      // corridors: darker strips with a dashed light line down the middle
      x.save();
      for(var cc = 1; cc < COLS; cc++){
        var cxp = cc * (RW + CW) - 1;
        x.fillStyle = '#080d16'; quad(x, cxp, 0, cxp + 1, GY); x.fill();
        x.strokeStyle = 'rgba(56,189,248,0.35)'; x.lineWidth = Math.max(1, 2 * s); x.setLineDash([10 * s, 14 * s]);
        x.beginPath(); x.moveTo(sx(cxp + 0.5, 0), sy(cxp + 0.5, 0)); x.lineTo(sx(cxp + 0.5, GY), sy(cxp + 0.5, GY)); x.stroke();
      }
      for(var rr2 = 1; rr2 < ROWS; rr2++){
        var cyp = rr2 * (RW + CW) - 1;
        x.setLineDash([]); x.fillStyle = '#080d16'; quad(x, 0, cyp, GX, cyp + 1); x.fill();
        x.strokeStyle = 'rgba(56,189,248,0.35)'; x.lineWidth = Math.max(1, 2 * s); x.setLineDash([10 * s, 14 * s]);
        x.beginPath(); x.moveTo(sx(0, cyp + 0.5), sy(0, cyp + 0.5)); x.lineTo(sx(GX, cyp + 0.5), sy(GX, cyp + 0.5)); x.stroke();
      }
      x.restore();
      // room tint and floor labels
      Object.keys(ROOMS).forEach(function(k){
        var rm = ROOMS[k], r = rm.r;
        x.fillStyle = hexA(rm.hue, 0.05);
        quad(x, r[0], r[1], r[2], r[3]); x.fill();
        x.save();
        x.setTransform(dpr * PA * s, dpr * PC * s, dpr * PB * s, dpr * PD * s, dpr * cam.ox, dpr * cam.oy);
        // label painted on the floor along the room's front-left edge
        x.translate(r[0] + 1.15, r[3] - 0.3);
        x.scale(0.01, 0.01);
        x.font = '800 58px "Segoe UI", system-ui, sans-serif';
        x.fillStyle = hexA(rm.hue, 0.6);
        x.textBaseline = 'alphabetic';
        x.fillText(rm.name, 0, 0);
        x.restore();
      });
    }
    function diamond(x, tx, ty){
      x.beginPath();
      x.moveTo(sx(tx, ty), sy(tx, ty)); x.lineTo(sx(tx + 1, ty), sy(tx + 1, ty));
      x.lineTo(sx(tx + 1, ty + 1), sy(tx + 1, ty + 1)); x.lineTo(sx(tx, ty + 1), sy(tx, ty + 1)); x.closePath();
    }
    function quad(x, x0, y0, x1, y1){
      x.beginPath();
      x.moveTo(sx(x0, y0), sy(x0, y0)); x.lineTo(sx(x1, y0), sy(x1, y0));
      x.lineTo(sx(x1, y1), sy(x1, y1)); x.lineTo(sx(x0, y1), sy(x0, y1)); x.closePath();
    }

    function sprite(name, wx, wy, scale, alpha, night){
      var p = MAP[name]; if(!p || !ready) return null;
      var s = cam.s * (scale || 1);
      var X = sx(wx, wy), Y = sy(wx, wy);
      if(alpha != null && alpha < 1) ctx.globalAlpha = alpha;
      ctx.drawImage(night ? imgNight : img, p[0], p[1], p[2], p[3], X + p[4] * s, Y + p[5] * s, p[2] * s, p[3] * s);
      if(alpha != null && alpha < 1) ctx.globalAlpha = 1;
      return [X + p[4] * s, Y + p[5] * s, p[2] * s, p[3] * s];
    }

    // ---- model reconciliation ---------------------------------------------
    function roomOf(stage){ return (STAGES[stage] || STAGES.waiting).room; }

    function takeSlot(room, id){
      var list = SLOTS[room] || [], key;
      for(var i = 0; i < list.length; i++){
        key = room + ':' + i;
        if(!slotTaken[key] || slotTaken[key] === id){ slotTaken[key] = id; return {key: key, x: list[i][0] + 0.5, y: list[i][1] + 0.5, face: list[i][2]}; }
      }
      // spill: a free walkable tile inside the room, picked by id
      var r = ROOMS[room].r, free = [];
      for(var ty = r[1]; ty < r[3]; ty++) for(var tx = r[0]; tx < r[2]; tx++){
        key = room + ':' + tx + ',' + ty;
        if(walkable(tx, ty) && !slotTaken[key]) free.push([tx, ty, key]);
      }
      if(!free.length) return {key: null, x: ROOMS[room].focus[0], y: ROOMS[room].focus[1] + 1};
      var f = free[hash(id) % free.length]; slotTaken[f[2]] = id;
      return {key: f[2], x: f[0] + 0.5, y: f[1] + 0.5};
    }
    function freeSlot(a){ if(a.slot && a.slot.key && slotTaken[a.slot.key] === a.id) delete slotTaken[a.slot.key]; a.slot = null; }

    function sendTo(a, room, now){
      freeSlot(a);
      a.room = room; a.roomSince = now; a.roomStage = a.stage;
      a.slot = takeSlot(room, a.id);
      a.path = findPath(a.x, a.y, a.slot.x, a.slot.y);
      if(a.path.length) a.path[a.path.length - 1] = [a.slot.x, a.slot.y];
      var dist = 0, px = a.x, py = a.y;
      a.path.forEach(function(p){ dist += Math.hypot(p[0] - px, p[1] - py); px = p[0]; py = p[1]; });
      a.speed = Math.max(3, dist / 3.5); // tiles per second, any trip under about 4 s
    }

    function update(model){
      if(!model) return;
      var now = performance.now();
      modelNow = model.now || Date.now(); lastModelAt = now;
      var seen = {};
      (model.sessions || []).forEach(function(s){
        if(!s || !s.id) return;
        seen[s.id] = 1;
        var a = agents[s.id];
        var stage = STAGES[s.stage] ? s.stage : 'waiting';
        if(!a){
          a = agents[s.id] = {id: s.id, x: AIRLOCK_DOOR[0], y: AIRLOCK_DOOR[1], path: [], face: 'SW',
                              born: now, alpha: first ? 1 : 0, leaving: false, stage: stage, pending: null,
                              phase: (hash(s.id) % 1000) / 1000};
          order.push(s.id);
          if(first){
            var sl = takeSlot(roomOf(stage), a.id);
            a.slot = sl; a.x = sl.x; a.y = sl.y; a.room = roomOf(stage); a.roomSince = now - MIN_DWELL; a.roomStage = stage;
          } else {
            sendTo(a, roomOf(stage === 'arriving' ? 'arriving' : stage), now);
          }
        }
        a.leaving = false;
        a.data = s; a.parent = s.parent || null; a.sub = !!s.parent;
        a.color = s.color || '#66788c';
        a.spriteBase = VENDOR_SPRITE[s.agent] || 'astronautB';
        if(stage !== a.stage){ a.stage = stage; a.stageAt = now; }
        var want = roomOf(stage);
        if(want !== a.room) a.pending = want; else a.pending = null;
      });
      Object.keys(agents).forEach(function(id){
        var a = agents[id];
        if(!seen[id] && !a.leaving){
          a.leaving = true; a.pending = null;
          freeSlot(a);
          a.path = findPath(a.x, a.y, AIRLOCK_DOOR[0], AIRLOCK_DOOR[1] + 0.6);
          a.path.push([AIRLOCK_DOOR[0], AIRLOCK_DOOR[1]]);
          a.speed = 3.5; a.room = null;
        }
      });
      // tokens added since the last update, counting only sessions seen both
      // times, so a session that docks does not spike the reactor
      var prev = burn.prev || {}, cur = {}, delta = 0;
      (model.sessions || []).forEach(function(s){ cur[s.id] = s.tokens || 0; if(prev[s.id] != null && cur[s.id] > prev[s.id]) delta += cur[s.id] - prev[s.id]; });
      if(burn.at && now > burn.at){
        var inst = delta / ((now - burn.at) / 60000);
        burn.rate = burn.rate ? burn.rate * 0.8 + inst * 0.2 : inst;
      }
      burn.prev = cur; burn.at = now;
      first = false;
      if(!raf && !timer && host) loop(); // running = a frame or a timer pending
      renderHud();
    }

    function step(dt, now){
      Object.keys(agents).forEach(function(id){
        var a = agents[id];
        if(a.pending && !a.leaving && now - a.roomSince >= MIN_DWELL){ var r = a.pending; a.pending = null; sendTo(a, r, now); }
        if(a.path && a.path.length){
          var p = a.path[0], dx = p[0] - a.x, dy = p[1] - a.y, d = Math.hypot(dx, dy), mv = a.speed * dt;
          if(d > 0.001) a.face = dirOf(dx, dy);
          if(d <= mv){ a.x = p[0]; a.y = p[1]; a.path.shift(); }
          else { a.x += dx / d * mv; a.y += dy / d * mv; }
          a.walking = true;
        } else {
          a.walking = false;
          if(a.slot){
            var f = a.slot.face;
            if(!f){ var fo = ROOMS[a.room] ? ROOMS[a.room].focus : [a.x, a.y + 1]; f = dirOf(fo[0] - a.x, fo[1] - a.y); }
            a.face = f;
          }
        }
        if(a.leaving){
          if(!a.path.length){ a.alpha -= dt * 1.5; if(a.alpha <= 0){ delete agents[id]; order = order.filter(function(o){ return o !== id; }); } }
        } else if(a.alpha < 1){ a.alpha = Math.min(1, a.alpha + dt * 1.2); }
      });
      // rovers: one keeps to the lounge, one patrols the corridors
      rovers.forEach(function(rv, ri){
        if(!rv.path.length && now > rv.next){
          var r1 = rng(Math.floor(now) + ri * 97)(), r2 = rng(Math.floor(now) + ri * 97 + 5)(), tx, ty;
          if(rv.area){ var ar = ROOMS[rv.area].r; tx = ar[0] + Math.floor(r1 * RW); ty = ar[1] + Math.floor(r2 * RW); }
          else if(r1 < 0.5){ tx = Math.floor(r2 * GX); ty = (1 + Math.floor(r1 * 2 * (ROWS - 1))) * (RW + CW) - 1; }
          else { ty = Math.floor(r2 * GY); tx = (1 + Math.floor((r1 - 0.5) * 2 * (COLS - 1))) * (RW + CW) - 1; }
          if(walkable(tx, ty)) rv.path = findPath(rv.x, rv.y, tx, ty);
          rv.next = now + 2000 + r1 * 4000;
        }
        if(rv.path.length){
          var q = rv.path[0], qx = q[0] - rv.x, qy = q[1] - rv.y, qd = Math.hypot(qx, qy), qm = 1.4 * dt;
          if(qd > 0.001) rv.face = dirOf(qx, qy);
          if(qd <= qm){ rv.x = q[0]; rv.y = q[1]; rv.path.shift(); } else { rv.x += qx / qd * qm; rv.y += qy / qd * qm; }
        }
      });
    }

    // ---- drawing ------------------------------------------------------------
    var ROVER_DIR = {SE: 'SE', S: 'SE', E: 'NE', SW: 'SW', W: 'SW', NW: 'NW', N: 'NW', NE: 'NE'};

    function draw(now){
      var t = now / 1000;
      ctx.setTransform(1, 0, 0, 1, 0, 0);
      ctx.drawImage(starCv, 0, 0);
      ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
      // twinkles
      var tr = rng(11);
      for(var i = 0; i < 26; i++){
        var x = tr() * W, y = tr() * H, ph = tr() * 6.28;
        var a = Math.max(0, Math.sin(t * 1.3 + ph)) * 0.8;
        if(a > 0.05){ ctx.fillStyle = 'rgba(200,220,255,' + a + ')'; ctx.fillRect(x, y, 1.6, 1.6); }
      }
      // a ship crossing every 45 s
      var cyc = (t % 45) / 45;
      if(cyc < 0.35){
        var k = cyc / 0.35, shx = -60 + k * (W + 120), shy = H * 0.12 + k * H * 0.08;
        var trail = ctx.createLinearGradient(shx - 90, shy - 14, shx, shy);
        trail.addColorStop(0, 'rgba(125,211,252,0)'); trail.addColorStop(1, 'rgba(125,211,252,0.7)');
        ctx.strokeStyle = trail; ctx.lineWidth = 2;
        ctx.beginPath(); ctx.moveTo(shx - 90, shy - 14); ctx.lineTo(shx, shy); ctx.stroke();
        ctx.fillStyle = '#e2e8f0';
        ctx.beginPath(); ctx.moveTo(shx + 7, shy + 1); ctx.lineTo(shx - 5, shy - 4); ctx.lineTo(shx - 3, shy + 4); ctx.closePath(); ctx.fill();
      }
      if(dirtyFloor || !floorCv) buildFloor();
      ctx.setTransform(1, 0, 0, 1, 0, 0);
      ctx.drawImage(floorCv, 0, 0);
      ctx.setTransform(dpr, 0, 0, dpr, 0, 0);

      // live room glow: brighter where agents are
      var occ = {};
      Object.keys(agents).forEach(function(id){ var a = agents[id]; if(a.room && !a.walking) occ[a.room] = (occ[a.room] || 0) + 1; });
      Object.keys(ROOMS).forEach(function(k){
        var rm = ROOMS[k], r = rm.r, n = occ[k] || 0;
        var pulse = 0.5 + 0.5 * Math.sin(t * 2 + r[0]);
        ctx.strokeStyle = hexA(rm.hue, n ? 0.55 + pulse * 0.35 : 0.22);
        ctx.lineWidth = Math.max(1, (n ? 2.2 : 1.2) * cam.s * 1.6);
        if(n){ ctx.shadowColor = rm.hue; ctx.shadowBlur = 12 * cam.s; }
        quad(ctx, r[0] + 0.06, r[1] + 0.06, r[2] - 0.06, r[3] - 0.06); ctx.stroke();
        ctx.shadowBlur = 0;
        if(n){ ctx.fillStyle = hexA(rm.hue, 0.05 + pulse * 0.04); ctx.fill(); }
      });

      // depth-sorted drawables
      var list = [];
      // Static walls and props come from the baked layer; only what moves is
      // drawn per frame, and a static piece is drawn again on top only where
      // it stands in front of something moving and overlaps it, so the depth
      // order is the same as drawing everything.
      if(dirtyProps || !propCv) buildProps();
      if(propCv){
        ctx.setTransform(1, 0, 0, 1, 0, 0);
        ctx.drawImage(propCv, 0, 0);
        ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
      }
      var cs = cam.s;
      function box(X, Y, l, tp, w, h){ return [X - l * cs, Y - tp * cs, w * cs, h * cs]; }
      Object.keys(agents).forEach(function(id){
        var a = agents[id], k = a.sub ? SUB_SCALE : AGENT_SCALE;
        list.push({d: dep(a.x, a.y) + 0.05, f: agentFn(a, t, now), r: box(sx(a.x, a.y), sy(a.x, a.y), 45 * k, 150 * k, 90 * k, 175 * k)});
      });
      rovers.forEach(function(rv){ list.push({d: dep(rv.x, rv.y), f: function(){ sprite('rover_' + ROVER_DIR[rv.face], rv.x, rv.y, 1.3, 1, true); }, r: box(sx(rv.x, rv.y), sy(rv.x, rv.y), 50, 70, 100, 90)}); });
      var TF = ROOMS.think.focus, PF = ROOMS.plan.focus, AF = AIRLOCK_DOOR, CF = ROOMS.core.focus;
      function fx(F){ return box(sx(F[0], F[1]), sy(F[0], F[1]), 160, 260, 320, 330); }
      list.push({d: dep(TF[0], TF[1]) + 0.2, f: function(){ orb(t, occ.think || 0); }, r: fx(TF)});
      list.push({d: dep(PF[0], PF[1]) + 1.2, f: function(){ holoTable(t, occ.plan || 0); }, r: fx(PF)});
      list.push({d: dep(AF[0], AF[1]) + 0.2, f: function(){ airlockGlow(t); }, r: fx(AF)});
      list.push({d: dep(CF[0], CF[1]) + 0.3, f: function(){ reactor(t); }, r: fx(CF)});
      propItems.forEach(function(it){ if(it.lights && it.r) list.push({d: it.d + 0.001, f: it.lights, r: it.r}); });
      var moving = list.slice();
      propItems.forEach(function(it){
        if(!it.r) return;
        for(var m = 0; m < moving.length; m++){
          var o = moving[m];
          if(it.d > o.d && overlap(it.r, o.r)){ list.push({d: it.d, f: it.f}); return; }
        }
      });
      list.sort(function(a, b){ return a.d - b.d; });
      for(var q = 0; q < list.length; q++) list[q].f();

      // tethers from subagents to parents, on top
      Object.keys(agents).forEach(function(id){
        var a = agents[id]; if(!a.parent || !agents[a.parent]) return;
        var p = agents[a.parent];
        var x1 = sx(a.x, a.y), y1 = sy(a.x, a.y) - 40 * cam.s, x2 = sx(p.x, p.y), y2 = sy(p.x, p.y) - 55 * cam.s;
        ctx.strokeStyle = hexA(a.color, 0.35 * Math.min(a.alpha, p.alpha)); ctx.lineWidth = Math.max(1, 1.6 * cam.s);
        ctx.setLineDash([4 * cam.s + 2, 5 * cam.s + 2]); ctx.lineDashOffset = -t * 20;
        ctx.beginPath(); ctx.moveTo(x1, y1);
        ctx.quadraticCurveTo((x1 + x2) / 2, Math.min(y1, y2) - 60 * cam.s, x2, y2); ctx.stroke();
        ctx.setLineDash([]);
      });
      // labels last so nothing covers them; front-most first, others nudge up
      placed = [];
      Object.keys(agents).map(function(id){ return agents[id]; })
        .sort(function(a, b){ return (b.head ? b.head[1] : 0) - (a.head ? a.head[1] : 0); })
        .forEach(function(a){ drawLabel(a, t, now); });
    }

    // ---- plan view (O, burn zone): pixel-art 3/4 top-down -----------------------
    // Drawn at 16 px per tile into a small canvas, then scaled up with
    // smoothing off, so every object reads as a pixel sprite. Static layers
    // (floors, walls, furniture) are baked once; agents, lights and effects
    // are drawn per frame on top.
    var PT = 16, PW = (GX + 2) * PT, PH = (GY + 2) * PT;
    var pxBake = null, pxFrame = null, pxLeds = [];
    function bx(x){ return (x + 1) * PT; }
    function by(y){ return (y + 1) * PT; }
    function shade(hex, f){
      var n = parseInt(hex.slice(1), 16), r = n >> 16 & 255, g = n >> 8 & 255, b = n & 255;
      function m(v){ return clamp(Math.round(f < 0 ? v * (1 + f) : v + (255 - v) * f), 0, 255); }
      return 'rgb(' + m(r) + ',' + m(g) + ',' + m(b) + ')';
    }
    function P(c, x, y, w, h, col){ c.fillStyle = col; c.fillRect(Math.round(x), Math.round(y), w, h); }
    // a box seen from 3/4 above: top face, front face, dark outline, top highlight
    function box(c, x, y, w, h, d, top, face){
      P(c, x - 1, y - 1, w + 2, h + d + 2, '#1a1f2e');
      P(c, x, y, w, h, top); P(c, x, y + h, w, d, face);
      P(c, x, y, w, 1, shade(top, 0.25));
    }
    var FLOOR = {
      archive: ['grid', '#4f8f6c'], uplink: ['tiles', '#3d8a94'], plan: ['tiles', '#557aa3'], airlock: ['big', '#8791a3'],
      fab: ['grid', '#4c8f9a'], think: ['tiles', '#7466a3'], test: ['dots', '#4f9068'], brief: ['big', '#93658a'],
      lounge: ['wood', '#946542'], recycler: ['hazard', '#7d7240'], cryo: ['tiles', '#78a9c2'], core: ['tiles', '#2f4b70']
    };
    function floorTile(c, X, Y, style, col, tx, ty){
      P(c, X, Y, PT, PT, col);
      var lt = shade(col, 0.18), dk = shade(col, -0.18);
      if(style === 'grid'){ for(var i = 0; i < PT; i += 4){ P(c, X + i, Y, 1, PT, lt); P(c, X, Y + i, PT, 1, lt); } }
      else if(style === 'tiles'){ P(c, X, Y, PT, 1, lt); P(c, X, Y, 1, PT, lt); P(c, X, Y + PT - 1, PT, 1, dk); P(c, X + PT - 1, Y, 1, PT, dk); P(c, X + 3, Y + 3, PT - 6, PT - 6, shade(col, 0.06)); }
      else if(style === 'big'){ if(tx % 2 === 0) P(c, X, Y, 1, PT, dk); if(ty % 2 === 0) P(c, X, Y, PT, 1, dk); P(c, X + 1, Y + 1, 2, 2, lt); }
      else if(style === 'dots'){ for(var a = 2; a < PT; a += 5) for(var b = 2; b < PT; b += 5) P(c, X + a, Y + b, 1, 1, lt); }
      else if(style === 'wood'){ for(var r = 0; r < 4; r++){ P(c, X, Y + r * 4, PT, 1, dk); P(c, X + ((r + tx) % 2 ? 5 : 11), Y + r * 4, 1, 4, dk); P(c, X, Y + r * 4 + 1, PT, 1, lt); } }
      else if(style === 'hazard'){ P(c, X, Y, PT, 1, dk); P(c, X, Y, 1, PT, dk); for(var hz = 0; hz < PT; hz += 4) P(c, X + hz, Y + hz, 2, 2, lt); }
      else if(style === 'diamond'){
        P(c, X, Y, PT, PT, '#6f7889');
        for(var q = 0; q < 2; q++) for(var w = 0; w < 2; w++){ var cx = X + q * 8 + 4, cy = Y + w * 8 + 4;
          P(c, cx - 1, cy - 3, 2, 1, '#8a93a4'); P(c, cx - 2, cy - 2, 4, 1, '#8a93a4'); P(c, cx - 3, cy - 1, 6, 2, '#8a93a4'); P(c, cx - 2, cy + 1, 4, 1, '#8a93a4'); P(c, cx - 1, cy + 2, 2, 1, '#8a93a4'); }
      }
    }
    var WALL_TOP = '#8ea7bd', WALL_FACE = '#4b6079', WALL_LINE = '#232a3b';
    function pxProp(c, name, tx, ty){
      var X = bx(tx), Y = by(ty), t = '#3a4560', k;
      if(/^structure_closed|^structure_detailed_SE/.test(name)){            // server rack
        box(c, X + 1, Y - 9, 14, 5, 19, '#3c4762', '#262e44');
        for(k = 0; k < 5; k++){ P(c, X + 3, Y - 2 + k * 3, 10, 2, '#141a28'); P(c, X + 9, Y - 2 + k * 3, 3, 1, '#2f8fd8'); pxLeds.push([X + 4 + (k % 2) * 2, Y - 2 + k * 3, k % 3 ? '#5dff9d' : '#ffd84d']); }
      } else if(/^structure_SE|^supports_low/.test(name)){                   // pillar
        box(c, X + 4, Y + 1, 8, 4, 10, '#7a8aa3', '#55657c'); pxLeds.push([X + 7, Y + 2, '#4fd1ff']);
      } else if(/^desk_chairArms/.test(name)){                                // armchair
        box(c, X + 2, Y + 3, 12, 5, 6, '#7a63b8', '#56448a'); P(c, X + 2, Y + 3, 2, 9, '#8b74c9'); P(c, X + 12, Y + 3, 2, 9, '#8b74c9');
      } else if(/^desk_chair/.test(name)){                                    // office chair
        P(c, X + 5, Y + 2, 6, 4, '#2c3448'); P(c, X + 4, Y + 6, 8, 5, '#3c4660'); P(c, X + 4, Y + 6, 8, 1, '#56617d'); P(c, X + 7, Y + 11, 2, 2, '#1a1f2e'); P(c, X + 4, Y + 13, 8, 1, '#1a1f2e');
      } else if(/^desk_computerScreen/.test(name)){                           // wall console
        box(c, X + 1, Y - 6, 14, 3, 13, '#44516e', '#2c3650');
        P(c, X + 3, Y - 1, 10, 6, '#0c2236'); P(c, X + 4, Y + 1, 2, 1, '#4fd1ff'); P(c, X + 6, Y + 2, 2, 1, '#4fd1ff'); P(c, X + 8, Y + 1, 2, 1, '#4fd1ff'); P(c, X + 10, Y, 2, 1, '#4fd1ff');
        pxLeds.push([X + 4, Y + 7, '#5dff9d']); pxLeds.push([X + 11, Y + 7, '#ff6b6b']);
      } else if(/^desk_/.test(name)){                                         // desk with monitor and keyboard
        box(c, X, Y + 2, 16, 7, 4, '#8394ad', '#5b6a83'); P(c, X + 1, Y + 13, 2, 2, '#1a1f2e'); P(c, X + 13, Y + 13, 2, 2, '#1a1f2e');
        P(c, X + 3, Y - 4, 10, 7, '#1a1f2e'); P(c, X + 4, Y - 3, 8, 5, '#24557e'); P(c, X + 5, Y - 2, 4, 1, '#7fd8ff'); P(c, X + 5, Y, 6, 1, '#4fa3d8');
        P(c, X + 7, Y + 3, 2, 1, '#1a1f2e'); P(c, X + 4, Y + 5, 8, 2, '#d5dde8');
        pxLeds.push([X + 11, Y + 1, '#5dff9d']);
      } else if(/^platform_/.test(name)){                                     // table
        box(c, X, Y + 1, 16, 11, 3, '#c3cfdc', '#8796ab'); P(c, X + 2, Y + 3, 12, 7, '#b2bfcf');
      } else if(/^satelliteDish_large/.test(name)){                          // big dish
        box(c, X + 4, Y + 6, 8, 4, 5, '#7a8aa3', '#55657c');
        c.fillStyle = '#1a1f2e'; c.beginPath(); c.ellipse(X + 8, Y - 4, 15, 11, 0, 0, 6.283); c.fill();
        c.fillStyle = '#c8d3df'; c.beginPath(); c.ellipse(X + 8, Y - 4, 14, 10, 0, 0, 6.283); c.fill();
        c.fillStyle = '#9fadbf'; c.beginPath(); c.ellipse(X + 8, Y - 3, 10, 7, 0, 0, 6.283); c.fill();
        P(c, X + 7, Y - 5, 3, 3, '#3c4762'); pxLeds.push([X + 8, Y - 4, '#ff6b6b']);
      } else if(/^satelliteDish/.test(name)){
        box(c, X + 5, Y + 6, 6, 3, 4, '#7a8aa3', '#55657c');
        c.fillStyle = '#1a1f2e'; c.beginPath(); c.ellipse(X + 8, Y + 1, 8, 6, 0, 0, 6.283); c.fill();
        c.fillStyle = '#c8d3df'; c.beginPath(); c.ellipse(X + 8, Y + 1, 7, 5, 0, 0, 6.283); c.fill(); P(c, X + 7, Y, 2, 2, '#3c4762');
      } else if(/^gate_complex/.test(name)){                                  // airlock door in the wall
        box(c, X - 6, Y - 10, 28, 4, 16, '#6c7c93', '#45556d'); P(c, X - 2, Y - 5, 20, 11, '#1a2333');
        P(c, X - 1, Y - 4, 9, 9, '#3a4d66'); P(c, X + 8, Y - 4, 9, 9, '#3a4d66'); P(c, X + 7, Y - 4, 2, 9, '#1a1f2e');
        pxLeds.push([X + 7, Y - 8, '#ffd84d']); pxLeds.push([X - 4, Y, '#5dff9d']); pxLeds.push([X + 19, Y, '#5dff9d']);
      } else if(/^gate_simple/.test(name)){                                   // scanner arch
        P(c, X + 2, Y + 8, 12, 6, '#22303f'); P(c, X + 3, Y + 10, 10, 1, '#34d399');
        box(c, X + 1, Y - 10, 3, 2, 20, '#a3b2c6', '#76869c'); box(c, X + 12, Y - 10, 3, 2, 20, '#a3b2c6', '#76869c');
        box(c, X + 1, Y - 12, 14, 3, 2, '#a3b2c6', '#76869c'); pxLeds.push([X + 7, Y - 11, '#5dff9d']);
      } else if(/^machine_barrelLarge/.test(name)){                          // cryo pod
        box(c, X + 2, Y - 8, 12, 15, 5, '#c1d4e3', '#8098ae'); P(c, X + 4, Y - 6, 8, 11, '#3d8fb8'); P(c, X + 5, Y - 5, 2, 8, '#a8e6ff');
        pxLeds.push([X + 7, Y + 9, '#4fd1ff']);
      } else if(/^machine_generatorLarge/.test(name)){                       // recycler press
        box(c, X - 8, Y - 6, 32, 12, 9, '#d6a23b', '#a37522');
        for(k = 0; k < 6; k++) P(c, X - 5 + k * 5, Y - 4, 3, 8, '#8a6418');
        P(c, X - 2, Y + 7, 20, 3, '#1a1f2e'); pxLeds.push([X + 22, Y - 4, '#ff6b6b']); pxLeds.push([X - 6, Y - 4, '#5dff9d']);
      } else if(/^machine_generator/.test(name)){                            // generator
        box(c, X + 2, Y, 12, 7, 7, '#a8b4c4', '#76839a'); P(c, X + 4, Y + 8, 8, 1, '#4b566b'); P(c, X + 4, Y + 10, 8, 1, '#4b566b');
        pxLeds.push([X + 4, Y + 2, '#5dff9d']); pxLeds.push([X + 7, Y + 2, '#ffd84d']);
      } else if(/^machine_barrel/.test(name)){                               // coffee machine
        box(c, X + 3, Y - 5, 10, 7, 11, '#c9d2dc', '#8d99a8'); P(c, X + 5, Y + 4, 6, 4, '#3a2b22'); P(c, X + 7, Y + 7, 2, 2, '#f2e6d8');
        pxLeds.push([X + 10, Y - 3, '#ff6b6b']);
      } else if(/^machine_wireless/.test(name)){                             // transmitter
        box(c, X + 3, Y + 3, 10, 5, 5, '#a8b4c4', '#76839a'); P(c, X + 7, Y - 8, 2, 11, '#76839a'); P(c, X + 5, Y - 9, 6, 2, '#a8b4c4');
        pxLeds.push([X + 7, Y - 10, '#ff6b6b']);
      } else if(/^barrels/.test(name)){
        for(k = 0; k < 4; k++){ var ox = X + 1 + (k % 2) * 7, oy = Y + (k >> 1) * 6; box(c, ox, oy, 6, 3, 4, '#e09a3a', '#b0721f'); P(c, ox, oy + 5, 6, 1, '#7d5014'); }
      } else if(/^barrel/.test(name)){
        box(c, X + 5, Y + 3, 6, 3, 7, '#e09a3a', '#b0721f'); P(c, X + 5, Y + 8, 6, 1, '#7d5014');
      } else if(/^pipe_ringSupport/.test(name)){                             // orb pedestal
        box(c, X + 4, Y + 5, 8, 4, 5, '#7a6bb0', '#56478a');
      } else if(name === 'plant'){
        P(c, X + 5, Y + 9, 6, 5, '#8a5a3c'); P(c, X + 5, Y + 9, 6, 1, '#a8714c');
        P(c, X + 4, Y + 2, 8, 8, '#2f8f4a'); P(c, X + 2, Y + 4, 4, 4, '#3aa85a'); P(c, X + 10, Y + 3, 4, 5, '#3aa85a'); P(c, X + 6, Y, 4, 4, '#4cc06c'); P(c, X + 7, Y + 5, 2, 2, '#4cc06c');
      } else { box(c, X + 3, Y + 3, 10, 6, 5, '#8394ad', '#5b6a83'); }
    }

    function buildPixel(){
      pxBake = document.createElement('canvas'); pxBake.width = PW; pxBake.height = PH;
      pxFrame = document.createElement('canvas'); pxFrame.width = PW; pxFrame.height = PH;
      var c = pxBake.getContext('2d'); pxLeds = [];
      P(c, 0, 0, PW, PH, '#151827');
      // corridors (diamond plate) under everything
      for(var ty = 0; ty < GY; ty++) for(var tx = 0; tx < GX; tx++) if(roomAt[ty * GX + tx] < 0) floorTile(c, bx(tx), by(ty), 'diamond', '', tx, ty);
      // room floors
      ROOM_IDS.forEach(function(id){
        var r = ROOMS[id].r, f = FLOOR[id] || ['tiles', '#55657c'];
        for(var y = r[1]; y < r[3]; y++) for(var x = r[0]; x < r[2]; x++) floorTile(c, bx(x), by(y), f[0], f[1], x - r[0], y - r[1]);
        if(id === 'plan'){ // blueprint rug under the table
          var X0 = bx(r[0] + 1) + 4, Y0 = by(r[1] + 1) + 4, w = 5 * PT - 8;
          P(c, X0, Y0, w, w, '#3d5f8c'); c.strokeStyle = '#7aa3d4'; c.lineWidth = 1;
          for(var g = 8; g < w; g += 12){ c.beginPath(); c.moveTo(X0 + g + 0.5, Y0 + 2); c.lineTo(X0 + g + 0.5, Y0 + 7); c.lineTo(X0 + g + 5.5, Y0 + 7); c.stroke(); }
        }
      });
      // outer hull
      P(c, 0, 0, PW, PT, WALL_LINE); P(c, 2, 2, PW - 4, 6, WALL_TOP); P(c, 2, 8, PW - 4, PT - 8, WALL_FACE);
      for(var s = 0; s < PW; s += 48) P(c, s + 20, 10, 10, 3, '#2c3a52');
      P(c, 0, 0, PT - 6, PH, WALL_LINE); P(c, 2, 2, PT - 10, PH - 4, WALL_TOP);
      P(c, PW - PT + 6, 0, PT - 6, PH, WALL_LINE); P(c, PW - PT + 8, 2, PT - 10, PH - 4, WALL_TOP);
      P(c, 0, PH - PT + 6, PW, PT - 6, WALL_LINE); P(c, 2, PH - PT + 8, PW - 4, PT - 10, WALL_TOP);
      // room walls with door gaps; the top wall shows its face (3/4 view)
      ROOM_IDS.forEach(function(id){
        var rm = ROOMS[id], r = rm.r, d = DOORS[id], X0 = bx(r[0]), Y0 = by(r[1]), X1 = bx(r[2]), Y1 = by(r[3]);
        var dX = bx(r[0] + 3), dY = by(r[1] + 3);
        function hwall(y, thick, face, door){
          var segs = door ? [[X0 - 3, dX], [dX + PT, X1 + 3]] : [[X0 - 3, X1 + 3]];
          segs.forEach(function(sg){ P(c, sg[0], y - 1, sg[1] - sg[0], thick + face + 2, WALL_LINE); P(c, sg[0], y, sg[1] - sg[0], thick, WALL_TOP); if(face) P(c, sg[0], y + thick, sg[1] - sg[0], face, WALL_FACE); });
          if(door){ P(c, dX - 2, y - 2, 2, thick + face + 4, '#2c3a52'); P(c, dX + PT, y - 2, 2, thick + face + 4, '#2c3a52'); }
        }
        function vwall(x, door){
          var segs = door ? [[Y0 - 3, dY], [dY + PT, Y1 + 3]] : [[Y0 - 3, Y1 + 3]];
          segs.forEach(function(sg){ P(c, x - 1, sg[0], 5, sg[1] - sg[0], WALL_LINE); P(c, x, sg[0], 3, sg[1] - sg[0], WALL_TOP); });
          if(door){ P(c, x - 2, dY - 2, 7, 2, '#2c3a52'); P(c, x - 2, dY + PT, 7, 2, '#2c3a52'); }
        }
        hwall(Y0 - 3, 3, 6, d.top); hwall(Y1, 3, 0, d.bottom); vwall(X0 - 3, d.left); vwall(X1, d.right);
        // a colour strip on the top wall marks the room
        P(c, X0 + 4, Y0 + 1, 18, 2, rm.hue);
      });
      // furniture, back to front
      PROPS.slice().concat(PLANTS).sort(function(a, b){ return a[2] - b[2]; }).forEach(function(p){ if(!p[5]) pxProp(c, p[0], p[1], p[2]); });
    }

    // Pixel astronaut, 10 x 16, feet at (X, Y). dir: 'down' | 'up' | 'left' | 'right'.
    function pxAstronaut(c, X, Y, dir, frame, suit, kind, alpha){
      c.globalAlpha = alpha;
      P(c, X - 5, Y - 1, 10, 2, 'rgba(0,0,0,0.35)');
      var lg = frame ? 1 : 0, dark = shade(suit, -0.35);
      P(c, X - 3, Y - 4 - lg, 2, 4 + lg, '#2a3142'); P(c, X + 1, Y - 4 - (1 - lg), 2, 4 + (1 - lg), '#2a3142');
      P(c, X - 5, Y - 11, 10, 8, '#1a1f2e'); P(c, X - 4, Y - 10, 8, 6, suit); P(c, X - 4, Y - 5, 8, 1, dark);
      if(dir === 'up'){ P(c, X - 3, Y - 10, 6, 5, '#cfd8e3'); P(c, X - 3, Y - 10, 6, 1, '#eef2f6'); }
      else { P(c, X - 1, Y - 9, 2, 2, shade(suit, 0.4)); }
      P(c, X - 6, Y - 9 + (frame ? 1 : 0), 2, 4, dark); P(c, X + 4, Y - 9 + (frame ? 0 : 1), 2, 4, dark);
      if(kind === 'alien'){
        P(c, X - 5, Y - 18, 10, 8, '#1a1f2e'); P(c, X - 4, Y - 17, 8, 6, '#6fd86f'); P(c, X - 4, Y - 17, 8, 1, '#a6f0a6');
        P(c, X - 1, Y - 21, 1, 4, '#6fd86f'); P(c, X - 2, Y - 22, 3, 1, '#ff6bd6');
        if(dir !== 'up'){ var ex = dir === 'left' ? -1 : dir === 'right' ? 1 : 0; P(c, X - 3 + ex, Y - 15, 2, 2, '#10131c'); P(c, X + 1 + ex, Y - 15, 2, 2, '#10131c'); }
      } else {
        P(c, X - 5, Y - 19, 10, 9, '#1a1f2e'); P(c, X - 4, Y - 18, 8, 7, '#eef2f6'); P(c, X - 4, Y - 12, 8, 1, '#b9c4d1');
        var visor = kind === 'astronautB' ? '#d9a63c' : '#1f3a5f';
        if(dir === 'down'){ P(c, X - 3, Y - 16, 6, 3, visor); P(c, X - 2, Y - 16, 2, 1, '#9fd4ff'); }
        else if(dir === 'left'){ P(c, X - 4, Y - 16, 4, 3, visor); P(c, X - 4, Y - 16, 1, 1, '#9fd4ff'); }
        else if(dir === 'right'){ P(c, X, Y - 16, 4, 3, visor); P(c, X + 3, Y - 16, 1, 1, '#9fd4ff'); }
        P(c, X - 4, Y - 18, 3, 1, suit);
      }
      c.globalAlpha = 1;
    }
    function pxDrone(c, X, Y, suit, t, alpha){
      c.globalAlpha = alpha;
      var hv = Math.round(Math.sin(t * 6) * 1);
      P(c, X - 3, Y - 1, 6, 1, 'rgba(0,0,0,0.3)');
      P(c, X - 4, Y - 10 + hv, 8, 5, '#1a1f2e'); P(c, X - 3, Y - 9 + hv, 6, 3, suit); P(c, X - 1, Y - 8 + hv, 2, 1, '#eef2f6');
      var blade = Math.floor(t * 12) % 2 ? 4 : 2;
      P(c, X - 3 - blade, Y - 12 + hv, blade * 2 + 6, 1, 'rgba(220,230,245,0.7)');
      c.globalAlpha = 1;
    }
    var ICONS = {
      reading:    ['.##.##.', '#..#..#', '#..#..#', '#..#..#', '.##.##.'],
      fetching:   ['#.....#', '.#.#.#.', '..###..', '...#...', '..###..'],
      planning:   ['#.####.', '.......', '#.####.', '.......', '#.###..'],
      coding:     ['..#.#..', '.#...#.', '#.....#', '.#...#.', '..#.#..'],
      running:    ['......#', '.....#.', '#...#..', '.#.#...', '..#....'],
      thinking:   ['.......', '.......', '#.#.#..', '.......', '.......'],
      delegating: ['##...##', '##...##', '..###..', '##...##', '##...##'],
      waiting:    ['.###...', '#...#..', '...#...', '..#....', '..#....'],
      dormant:    ['#####..', '...#...', '..#....', '.#.....', '#####..'],
      compacting: ['.###...', '#...#..', '#.#.#..', '#...#..', '.###...'],
      arriving:   ['..#....', '..#....', '#####..', '.###...', '..#....']
    };
    function pxBubble(c, X, Y, stage, t){
      var ic = ICONS[stage]; if(!ic) return;
      var col = (STAGES[stage] || STAGES.waiting).color, bob = Math.round(Math.sin(t * 3) * 1);
      Y += bob;
      P(c, X - 6, Y - 9, 13, 9, '#1a1f2e'); P(c, X - 5, Y - 8, 11, 7, '#eef2f6'); P(c, X - 1, Y, 3, 1, '#1a1f2e'); P(c, X, Y, 1, 2, '#1a1f2e');
      for(var r = 0; r < ic.length; r++) for(var k = 0; k < 7; k++){
        if(ic[r][k] !== '#') continue;
        if(stage === 'thinking' && Math.floor(t * 3) % 3 < k / 2) continue;
        P(c, X - 3 + k, Y - 7 + r + (r > 4 ? 0 : 0), 1, 1, col);
      }
    }

    function drawTop(now){
      var t = now / 1000;
      if(!pxBake) buildPixel();
      var c = pxFrame.getContext('2d');
      c.drawImage(pxBake, 0, 0);
      // blinking lights
      pxLeds.forEach(function(l, i){ if(Math.sin(t * (1.5 + (i % 5) * 0.7) + i) > -0.2) P(c, l[0], l[1], 1, 1, l[2]); });
      var occ = {};
      Object.keys(agents).forEach(function(id){ var a = agents[id]; if(a.room && !a.walking) occ[a.room] = (occ[a.room] || 0) + 1; });
      // occupied rooms glow on the floor edge
      ROOM_IDS.forEach(function(id){
        if(!occ[id]) return; var r = ROOMS[id].r, pu = 0.25 + 0.2 * Math.sin(t * 3);
        c.fillStyle = hexA(ROOMS[id].hue, pu);
        c.fillRect(bx(r[0]), by(r[3]) - 2, RW * PT, 2); c.fillRect(bx(r[0]), by(r[1]) + 3, 2, RW * PT - 5); c.fillRect(bx(r[2]) - 2, by(r[1]) + 3, 2, RW * PT - 5);
      });
      // think orb
      var F = ROOMS.think.focus, OX = bx(F[0]), OY = by(F[1]) - 6, orr = 4 + (occ.think ? Math.round(Math.sin(t * 4)) : 0);
      c.fillStyle = hexA('#c084fc', occ.think ? 0.35 : 0.15); c.beginPath(); c.arc(OX, OY, orr + 6, 0, 6.283); c.fill();
      P(c, OX - 3, OY - 3, 6, 6, '#e9d5ff'); P(c, OX - 2, OY - 2, 2, 2, '#ffffff');
      var orbA = t * 2; P(c, OX + Math.round(Math.cos(orbA) * 8), OY + Math.round(Math.sin(orbA) * 3), 2, 2, '#c084fc');
      // plan hologram
      F = ROOMS.plan.focus; var HX = bx(F[0]), HY = by(F[1]);
      if(occ.plan){ for(var hk = 0; hk < 3; hk++){ var hy = HY - 10 - ((t * 10 + hk * 6) % 18); c.fillStyle = hexA('#a3e635', 0.7 - ((t * 10 + hk * 6) % 18) / 30); c.fillRect(HX - 8, Math.round(hy), 16, 1); } }
      P(c, HX - 6, HY - 6, 12, 1, hexA('#d9f99d', occ.plan ? 0.9 : 0.4));
      // scanner beams in the test chamber
      if(occ.test){ PROPS.forEach(function(p){ if(/^gate_simple/.test(p[0])){ var gy = by(p[2]) - 8 + Math.round((Math.sin(t * 4 + p[1]) + 1) * 8); P(c, bx(p[1]) + 4, gy, 8, 1, '#5dff9d'); } }); }
      // reactor
      F = ROOMS.core.focus; var RX = bx(F[0]), RY = by(F[1]);
      var kk = clamp(Math.log10(1 + burn.rate) / 6, 0.08, 1), beat = 0.6 + 0.4 * Math.sin(t * (2 + kk * 8));
      c.fillStyle = hexA('#7dd3fc', 0.25 + 0.4 * kk * beat); c.fillRect(RX - 3, RY - 30, 6, 30);
      c.fillStyle = hexA('#e0f2fe', 0.5 * beat + 0.3 * kk); c.fillRect(RX - 1, RY - 30, 2, 30);
      c.fillStyle = hexA('#38bdf8', 0.18 * beat + 0.1); c.beginPath(); c.arc(RX, RY - 12, 10 + 8 * kk, 0, 6.283); c.fill();
      // tethers
      Object.keys(agents).forEach(function(id){
        var a = agents[id]; if(!a.parent || !agents[a.parent]) return; var pa = agents[a.parent];
        var x1 = bx(a.x), y1 = by(a.y) - 8, x2 = bx(pa.x), y2 = by(pa.y) - 10, n = Math.max(2, Math.round(Math.hypot(x2 - x1, y2 - y1) / 4));
        for(var i = 0; i <= n; i++){ if((i + Math.floor(t * 6)) % 2) continue; P(c, x1 + (x2 - x1) * i / n, y1 + (y2 - y1) * i / n, 1, 1, a.color); }
      });
      // rovers
      rovers.forEach(function(rv){ var X = bx(rv.x), Y = by(rv.y); box(c, X - 3, Y - 4, 6, 3, 2, '#e09a3a', '#b0721f'); P(c, X - 1, Y - 4, 2, 1, Math.floor(t * 4) % 2 ? '#5dff9d' : '#1a1f2e'); });
      // agents, sorted by y so nearer ones overlap
      var list = Object.keys(agents).map(function(id){ return agents[id]; }).sort(function(a, b){ return a.y - b.y; });
      list.forEach(function(a){
        var X = Math.round(bx(a.x)), Y = Math.round(by(a.y)) + 4;
        var d4 = {SE: 'right', E: 'right', NE: 'up', N: 'up', NW: 'left', W: 'left', SW: 'down', S: 'down'}[a.face] || 'down';
        // tile space: +x is right on screen, +y is down
        d4 = {SE: 'right', E: 'right', S: 'down', SW: 'down', W: 'left', NW: 'left', N: 'up', NE: 'up'}[a.face] || 'down';
        var frame = a.walking ? Math.floor(t * 8 + a.phase * 4) % 2 : 0;
        var kind = a.spriteBase === 'alien' ? 'alien' : a.spriteBase;
        P(c, X - 6, Y, 12, 1, hexA(a.color, 0.9));
        if(a.sub) pxDrone(c, X, Y, a.color, t + a.phase * 3, Math.max(0, a.alpha));
        else pxAstronaut(c, X, Y, d4, frame, a.color, kind, Math.max(0, a.alpha));
        var dormant = (a.roomStage || a.stage) === 'dormant' && !a.walking && !a.pending;
        if(dormant){ c.fillStyle = 'rgba(125,211,252,0.3)'; c.fillRect(X - 5, Y - 19, 10, 19); }
        if(!a.walking && !a.pending && a.alpha > 0.6) pxBubble(c, X + (a.sub ? 0 : 1), Y - (a.sub ? 13 : 22), a.roomStage || a.stage, t + a.phase);
        var k = cam.s;
        a.box = [cam.ox + (X - 6) * k, cam.oy + (Y - 22) * k, 12 * k, 24 * k];
        a.head = [cam.ox + X * k, cam.oy + (Y - 22) * k];
      });
      // scale the frame up, pixels stay crisp
      ctx.setTransform(1, 0, 0, 1, 0, 0);
      ctx.fillStyle = '#0b0d16'; ctx.fillRect(0, 0, canvas.width, canvas.height);
      ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
      ctx.imageSmoothingEnabled = false;
      ctx.drawImage(pxFrame, cam.ox, cam.oy, PW * cam.s, PH * cam.s);
      ctx.imageSmoothingEnabled = true;
      // text overlays at screen resolution: room names, tok/min, agent names
      var fs = clamp(cam.s * 5.2, 9, 13);
      ctx.font = '700 ' + fs + 'px Consolas, "Cascadia Mono", monospace'; ctx.textBaseline = 'top'; ctx.textAlign = 'left';
      ROOM_IDS.forEach(function(id){
        var r = ROOMS[id].r; ctx.fillStyle = 'rgba(10,14,24,0.65)';
        var tw = ctx.measureText(ROOMS[id].name).width;
        ctx.fillRect(cam.ox + (bx(r[0]) + 24) * cam.s - 3, cam.oy + by(r[1]) * cam.s - 2, tw + 6, fs + 3);
        ctx.fillStyle = ROOMS[id].hue; ctx.fillText(ROOMS[id].name, cam.ox + (bx(r[0]) + 24) * cam.s, cam.oy + by(r[1]) * cam.s);
      });
      ctx.textAlign = 'center'; ctx.fillStyle = '#bae6fd';
      ctx.fillText(fmtTokens(Math.round(burn.rate)) + ' tok/min', cam.ox + RX * cam.s, cam.oy + (RY + 20) * cam.s);
      if(cam.s >= 1.6){
        ctx.font = '600 ' + clamp(cam.s * 4.4, 9, 12) + 'px "Segoe UI", system-ui, sans-serif';
        list.forEach(function(a){
          if(a.sub || a.alpha < 0.4) return;
          var nm = shortProject((a.data || {}).project), tw2 = ctx.measureText(nm).width;
          var X = cam.ox + bx(a.x) * cam.s, Y = cam.oy + (by(a.y) + 7) * cam.s;
          ctx.fillStyle = 'rgba(10,14,24,0.75)'; ctx.fillRect(X - tw2 / 2 - 3, Y, tw2 + 6, 14);
          ctx.fillStyle = a.color; ctx.fillRect(X - tw2 / 2 - 3, Y, 2, 14);
          ctx.fillStyle = '#e6edf6'; ctx.fillText(nm, X, Y + 1);
        });
      }
      ctx.textBaseline = 'alphabetic';
      var lb = hoverId && agents[hoverId];
      if(lb && lb.box){ ctx.strokeStyle = '#ffffff'; ctx.lineWidth = 1.5; ctx.strokeRect(lb.box[0] - 2, lb.box[1] - 2, lb.box[2] + 4, lb.box[3] + 4); }
    }

    function wallFn(name, x, y){ return function(){ return sprite(name, x, y, 1, 1, true); }; }

    function overlap(a, b){ return a[0] < b[0] + b[2] && b[0] < a[0] + a[2] && a[1] < b[1] + b[3] && b[1] < a[1] + a[3]; }

    // Bakes every wall and prop sprite, in depth order, into propCv at the
    // current camera, and keeps each one's screen box for the frame's
    // re-draw test. Waits for the atlas image.
    function buildProps(){
      if(!ready || !canvas) return;
      propItems = [];
      for(var w = 0; w < GY; w++) propItems.push({d: dep(0.5, w + 0.5) - 1.4, f: wallFn('corridor_wall_SW', 0.5, w + 0.5)});
      for(var v = 0; v < GX; v++) propItems.push({d: dep(v + 0.5, 0.5) - 1.4, f: wallFn('corridor_wall_NW', v + 0.5, 0.5)});
      PROPS.forEach(function(p){
        propItems.push({d: dep(p[1] + 0.5, p[2] + 0.5), f: propFn(p[0], p[1] + 0.5, p[2] + 0.5, p[4]), lights: propLights(p[0], p[1] + 0.5, p[2] + 0.5)});
      });
      propItems.sort(function(a, b){ return a.d - b.d; });
      propCv = document.createElement('canvas');
      propCv.width = canvas.width; propCv.height = canvas.height;
      var keep = ctx;
      ctx = propCv.getContext('2d');
      ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
      propItems.forEach(function(it){ it.r = it.f(); });
      ctx = keep;
      dirtyProps = false;
    }
    // The kit draws desks, chairs and dishes small next to its astronauts;
    // scale those up so a workstation reads as one at a glance.
    var PROP_SCALE = [[/^desk_chair/, 1.9], [/^desk_/, 1.9], [/^satelliteDish/, 1.6], [/^machine_wireless/, 1.6], [/^gate_/, 1.5],
                      [/^barrel/, 1.5], [/^machine_barrel_/, 1.6], [/^machine_generator_/, 1.5], [/^pipe_ringSupport/, 1.4],
                      [/^platform_small/, 1.0], [/^structure_SE/, 0.9]];
    function propScale(name){ for(var i = 0; i < PROP_SCALE.length; i++) if(PROP_SCALE[i][0].test(name)) return PROP_SCALE[i][1]; return 1; }
    function propFn(name, x, y, override){
      var sc = override || propScale(name);
      return function(){ return sprite(name, x, y, sc, 1, true); };
    }
    // A prop's animated lights, drawn per frame over its baked sprite; null
    // for a prop without any.
    function propLights(name, x, y){
      if(!/^structure_closed|^structure_detailed|desk_computer|machine_|satellite|gate_/.test(name)) return null;
      return function(){
        if(/^structure_closed|^structure_detailed/.test(name)){
          // blinking data lights inside the archive racks
          for(var k = 0; k < 6; k++){
            var on = Math.sin(performance.now() / (180 + k * 37) + x * 5 + y * 3 + k) > 0.2;
            if(!on) continue;
            ctx.fillStyle = k % 3 ? 'rgba(96,165,250,0.9)' : 'rgba(125,211,252,0.95)';
            var LX = sx(x, y) + (-22 + (k % 3) * 18) * cam.s, LY = sy(x, y) - (40 + Math.floor(k / 3) * 30 + (k % 3) * 6) * cam.s;
            ctx.fillRect(LX, LY, Math.max(1.5, 4 * cam.s), Math.max(1.5, 3 * cam.s));
          }
        }
        // status lights on machines
        if(/desk_computer|machine_|satellite|gate_/.test(name)){
          var X = sx(x, y), Y = sy(x, y) - 22 * cam.s, b = (Math.sin(performance.now() / 300 + x * 3 + y) + 1) / 2;
          ctx.fillStyle = 'rgba(52,211,153,' + (0.3 + b * 0.6) + ')';
          ctx.fillRect(X - 1.5, Y, Math.max(2, 3 * cam.s), Math.max(2, 3 * cam.s));
        }
      };
    }

    function orb(t, n){
      var F = ROOMS.think.focus, X = sx(F[0], F[1]), Y = sy(F[0], F[1]) - 100 * cam.s, r = (16 + (n ? 4 * Math.sin(t * 3) : 0)) * cam.s;
      var g = ctx.createRadialGradient(X, Y, 0, X, Y, r * 4);
      g.addColorStop(0, hexA('#c084fc', n ? 0.55 : 0.25)); g.addColorStop(1, 'rgba(192,132,252,0)');
      ctx.fillStyle = g; ctx.beginPath(); ctx.arc(X, Y, r * 4, 0, 6.283); ctx.fill();
      ctx.fillStyle = '#efe4ff'; ctx.beginPath(); ctx.arc(X, Y, r * 0.6, 0, 6.283); ctx.fill();
      ctx.strokeStyle = hexA('#c084fc', 0.7); ctx.lineWidth = Math.max(1, 1.5 * cam.s);
      for(var i = 0; i < 2; i++){ ctx.beginPath(); ctx.ellipse(X, Y, r * 1.8, r * 0.6, t * (i ? -0.8 : 0.6) + i, 0, 6.283); ctx.stroke(); }
    }

    function holoTable(t, n){
      var F = ROOMS.plan.focus, X = sx(F[0], F[1]), Y = sy(F[0], F[1]) - 75 * cam.s, s = cam.s * 1.5;
      var g = ctx.createLinearGradient(X, Y + 40 * s, X, Y - 50 * s);
      g.addColorStop(0, hexA('#a3e635', n ? 0.35 : 0.12)); g.addColorStop(1, 'rgba(163,230,53,0)');
      ctx.fillStyle = g;
      ctx.beginPath(); ctx.moveTo(X - 55 * s, Y + 30 * s); ctx.lineTo(X + 55 * s, Y + 30 * s); ctx.lineTo(X + 30 * s, Y - 40 * s); ctx.lineTo(X - 30 * s, Y - 40 * s); ctx.closePath(); ctx.fill();
      // a rotating wireframe of task cards
      ctx.strokeStyle = hexA('#d9f99d', n ? 0.85 : 0.35); ctx.lineWidth = Math.max(1, 1.3 * s);
      for(var i = 0; i < 4; i++){
        var ang = t * 0.7 + i * Math.PI / 2, cx = X + Math.cos(ang) * 28 * s, cy = Y - 8 * s + Math.sin(ang) * 9 * s - i * 6 * s;
        ctx.strokeRect(cx - 9 * s, cy - 6 * s, 18 * s, 12 * s);
        ctx.beginPath(); ctx.moveTo(cx - 6 * s, cy - 2 * s); ctx.lineTo(cx + 5 * s, cy - 2 * s); ctx.moveTo(cx - 6 * s, cy + 2 * s); ctx.lineTo(cx + 2 * s, cy + 2 * s); ctx.stroke();
      }
    }

    function reactor(t){
      var F = ROOMS.core.focus, X = sx(F[0], F[1]), Y = sy(F[0], F[1]), s = cam.s;
      var k = clamp(Math.log10(1 + burn.rate) / 6, 0.08, 1); // 1M tokens/min reads as full power
      var beat = 0.6 + 0.4 * Math.sin(t * (2 + k * 8));
      var g = ctx.createLinearGradient(X, Y, X, Y - 230 * s);
      g.addColorStop(0, hexA('#38bdf8', 0.15 + 0.5 * k * beat)); g.addColorStop(0.5, hexA('#7dd3fc', 0.2 + 0.4 * k * beat)); g.addColorStop(1, 'rgba(125,211,252,0)');
      ctx.fillStyle = g; ctx.fillRect(X - (10 + 10 * k) * s, Y - 230 * s, (20 + 20 * k) * s, 230 * s);
      var hg = ctx.createRadialGradient(X, Y - 80 * s, 0, X, Y - 80 * s, (60 + 50 * k) * s);
      hg.addColorStop(0, hexA('#e0f2fe', 0.35 * beat + 0.2 * k)); hg.addColorStop(1, 'rgba(56,189,248,0)');
      ctx.fillStyle = hg; ctx.beginPath(); ctx.arc(X, Y - 80 * s, (60 + 50 * k) * s, 0, 6.283); ctx.fill();
      for(var i = 0; i < 3; i++){
        var ph = (t * (0.3 + k) + i / 3) % 1;
        ctx.strokeStyle = hexA('#7dd3fc', 0.8 * (1 - ph)); ctx.lineWidth = Math.max(1, 2 * s);
        ctx.beginPath(); ctx.ellipse(X, Y - 20 * s - ph * 160 * s, (34 - ph * 14) * s, (12 - ph * 5) * s, 0, 0, 6.283); ctx.stroke();
      }
      ctx.font = '700 ' + Math.max(9, 13 * s * 1.6) + 'px "Segoe UI", system-ui, sans-serif'; ctx.textAlign = 'center';
      ctx.fillStyle = hexA('#bae6fd', 0.9);
      ctx.fillText(fmtTokens(Math.round(burn.rate)) + ' tok/min', X, Y - 250 * s);
    }

    function airlockGlow(t){
      var X = sx(AIRLOCK_DOOR[0], 0.5), Y = sy(AIRLOCK_DOOR[0], 0.5) - 80 * cam.s, s = cam.s * 1.6;
      var recent = false;
      Object.keys(agents).forEach(function(id){ var a = agents[id]; if((a.stage === 'arriving' || a.leaving || a.alpha < 1) && Math.hypot(a.x - AIRLOCK_DOOR[0], a.y - 1) < 4) recent = true; });
      var g = ctx.createRadialGradient(X, Y, 0, X, Y, 45 * s);
      g.addColorStop(0, recent ? 'rgba(186,230,253,0.75)' : 'rgba(148,163,184,0.25)'); g.addColorStop(1, 'rgba(148,163,184,0)');
      ctx.fillStyle = g; ctx.beginPath(); ctx.arc(X, Y, 45 * s, 0, 6.283); ctx.fill();
    }

    function agentFn(a, t, now){
      return function(){
        var sc = a.sub ? SUB_SCALE : AGENT_SCALE;
        var X = sx(a.x, a.y), Y = sy(a.x, a.y), s = cam.s * sc;
        var dormant = (a.roomStage || a.stage) === 'dormant' && !a.walking && !a.pending;
        // floor ring in the session colour
        ctx.strokeStyle = hexA(a.color, 0.85 * a.alpha); ctx.lineWidth = Math.max(1, 2 * cam.s);
        ctx.shadowColor = a.color; ctx.shadowBlur = 8 * cam.s;
        ctx.beginPath(); ctx.ellipse(X, Y, 20 * s, 10 * s, 0, 0, 6.283); ctx.stroke(); ctx.shadowBlur = 0;
        if(hoverId === a.id){ ctx.fillStyle = hexA(a.color, 0.25); ctx.fill(); }
        var bob = a.walking ? Math.abs(Math.sin(t * 11 + a.phase * 6)) * 4 * s : Math.sin(t * 2 + a.phase * 6) * 0.8 * s;
        var name = a.spriteBase + '_' + a.face;
        // teleport shimmer on arrival and departure
        if(a.alpha < 1){
          ctx.fillStyle = 'rgba(186,230,253,' + (0.5 * (1 - a.alpha)) + ')';
          ctx.fillRect(X - 16 * s, Y - 90 * s, 32 * s, 90 * s);
        }
        var box = spriteAt(name, X, Y - bob, s, a.alpha * (dormant ? 0.55 : 1));
        if(dormant && box){
          ctx.fillStyle = 'rgba(125,211,252,0.25)'; ctx.fillRect(box[0], box[1], box[2], box[3]);
        }
        a.box = box;
        a.head = [X, Y - 72 * s - bob];
        effect(a, t, X, Y - 72 * s - bob, s);
      };
    }
    function spriteAt(name, X, Y, s, alpha){
      var p = MAP[name]; if(!p || !ready) return null;
      if(alpha < 1) ctx.globalAlpha = Math.max(0, alpha);
      ctx.drawImage(img, p[0], p[1], p[2], p[3], X + p[4] * s, Y + p[5] * s, p[2] * s, p[3] * s);
      ctx.globalAlpha = 1;
      return [X + p[4] * s, Y + p[5] * s, p[2] * s, p[3] * s];
    }

    // Stage effects around the agent's head. Deterministic in t, no particles kept.
    function effect(a, t, X, Y, s){
      if(a.walking || a.alpha < 0.6) return;
      if(a.pending) return;
      var st = a.roomStage || a.stage, c = STAGES[st] ? STAGES[st].color : '#fff', k, ph, i;
      ctx.save();
      ctx.globalAlpha = a.alpha;
      if(st === 'reading'){
        for(i = 0; i < 2; i++){
          ph = (t * 0.8 + i * 0.5 + a.phase) % 1;
          ctx.strokeStyle = hexA(c, 0.9 * (1 - ph)); ctx.fillStyle = hexA(c, 0.15 * (1 - ph)); ctx.lineWidth = Math.max(1, s);
          var px = X + 14 * s, py = Y - 8 * s - ph * 22 * s;
          ctx.fillRect(px, py, 14 * s, 18 * s); ctx.strokeRect(px, py, 14 * s, 18 * s);
          for(k = 0; k < 3; k++){ ctx.beginPath(); ctx.moveTo(px + 3 * s, py + (5 + k * 4) * s); ctx.lineTo(px + 11 * s, py + (5 + k * 4) * s); ctx.stroke(); }
        }
      } else if(st === 'fetching'){
        ctx.strokeStyle = hexA(c, 0.5); ctx.lineWidth = Math.max(1, 1.5 * s);
        ctx.beginPath(); ctx.moveTo(X, Y - 4 * s); ctx.lineTo(X, Y - 70 * s); ctx.stroke();
        for(i = 0; i < 4; i++){ ph = (t * 1.6 + i / 4 + a.phase) % 1; ctx.fillStyle = hexA(c, 1 - ph * 0.5); ctx.fillRect(X - 2 * s, Y - 70 * s + ph * 66 * s, 4 * s, 4 * s); }
      } else if(st === 'planning'){
        ctx.strokeStyle = hexA(c, 0.85); ctx.lineWidth = Math.max(1, 1.4 * s);
        for(i = 0; i < 3; i++){ var yy = Y - 6 * s - i * 7 * s; ctx.strokeRect(X + 12 * s, yy, 5 * s, 5 * s);
          if((t * 1.2 + a.phase) % 3 > i){ ctx.beginPath(); ctx.moveTo(X + 13 * s, yy + 2.5 * s); ctx.lineTo(X + 14.5 * s, yy + 4 * s); ctx.lineTo(X + 17 * s, yy + 0.5 * s); ctx.stroke(); }
          ctx.beginPath(); ctx.moveTo(X + 20 * s, yy + 2.5 * s); ctx.lineTo(X + 30 * s, yy + 2.5 * s); ctx.stroke(); }
      } else if(st === 'coding'){
        ctx.font = '700 ' + Math.max(8, 11 * s) + 'px Consolas, monospace'; ctx.textAlign = 'center';
        var glyphs = ['</>', '{ }', '=>', ';'];
        for(i = 0; i < 3; i++){ ph = (t * 0.7 + i / 3 + a.phase) % 1; ctx.fillStyle = hexA(c, 1 - ph);
          ctx.fillText(glyphs[(i + Math.floor(t * 0.7 + a.phase)) % 4], X + (i - 1) * 12 * s, Y - ph * 28 * s); }
        if(Math.sin(t * 17 + a.phase * 9) > 0.6){ ctx.fillStyle = '#fff7d6'; ctx.fillRect(X + 18 * s, Y + 40 * s, 2 * s, 2 * s); ctx.fillRect(X + 22 * s, Y + 37 * s, 2 * s, 2 * s); }
      } else if(st === 'running'){
        ph = (Math.sin(t * 3 + a.phase * 6) + 1) / 2;
        var top = Y - 6 * s, bot = Y + 72 * s, ly = top + (bot - top) * ph;
        ctx.strokeStyle = hexA(c, 0.9); ctx.lineWidth = Math.max(1, 2 * s); ctx.shadowColor = c; ctx.shadowBlur = 8 * s;
        ctx.beginPath(); ctx.moveTo(X - 20 * s, ly); ctx.lineTo(X + 20 * s, ly); ctx.stroke(); ctx.shadowBlur = 0;
        var ok = Math.floor(t * 0.5 + a.phase * 3) % 5 !== 4;
        ctx.fillStyle = ok ? hexA('#34d399', 0.95) : hexA('#f87171', 0.95);
        ctx.font = '700 ' + Math.max(9, 12 * s) + 'px "Segoe UI", sans-serif'; ctx.textAlign = 'center';
        ctx.fillText(ok ? 'PASS' : 'FAIL', X, Y - 12 * s);
      } else if(st === 'thinking'){
        for(i = 0; i < 3; i++){ var an = t * 2.4 + i * 2.094 + a.phase * 6;
          ctx.fillStyle = hexA(c, 0.95); ctx.beginPath(); ctx.arc(X + Math.cos(an) * 16 * s, Y - 6 * s + Math.sin(an) * 6 * s, 2.6 * s + 0.5, 0, 6.283); ctx.fill(); }
      } else if(st === 'delegating'){
        for(i = 0; i < 2; i++){ ph = (t * 0.9 + i * 0.5) % 1; ctx.strokeStyle = hexA(c, 1 - ph); ctx.lineWidth = Math.max(1, 1.4 * s);
          ctx.beginPath(); ctx.arc(X, Y - 4 * s, 8 * s + ph * 26 * s, Math.PI * 1.15, Math.PI * 1.85); ctx.stroke(); }
      } else if(st === 'waiting'){
        var bx = X + 14 * s, by = Y - 18 * s, bw = 22 * s, bh = 16 * s;
        ctx.fillStyle = hexA('#0f172a', 0.9); ctx.strokeStyle = hexA(c, 0.95); ctx.lineWidth = Math.max(1, 1.4 * s);
        roundRect(ctx, bx, by, bw, bh, 4 * s); ctx.fill(); ctx.stroke();
        ctx.beginPath(); ctx.moveTo(bx + 4 * s, by + bh); ctx.lineTo(bx, by + bh + 5 * s); ctx.lineTo(bx + 9 * s, by + bh); ctx.fill(); ctx.stroke();
        ctx.fillStyle = hexA(c, 0.5 + 0.5 * Math.abs(Math.sin(t * 2))); ctx.font = '800 ' + Math.max(9, 12 * s) + 'px "Segoe UI", sans-serif'; ctx.textAlign = 'center';
        ctx.fillText('?', bx + bw / 2, by + bh * 0.78);
      } else if(st === 'dormant'){
        ctx.fillStyle = hexA(c, 0.9); ctx.textAlign = 'left';
        for(i = 0; i < 3; i++){ ph = (t * 0.35 + i / 3 + a.phase) % 1; ctx.globalAlpha = a.alpha * (1 - ph);
          ctx.font = '700 ' + Math.max(8, (8 + ph * 8) * s) + 'px "Segoe UI", sans-serif'; ctx.fillText('z', X + 10 * s + ph * 14 * s, Y - ph * 26 * s); }
      } else if(st === 'compacting'){
        for(i = 0; i < 8; i++){ var ang2 = t * 3 + i * 0.785, rr = (1 - ((t * 0.8 + i / 8) % 1)) * 26 * s;
          ctx.fillStyle = hexA(c, 0.9); ctx.fillRect(X + Math.cos(ang2) * rr - 1.5 * s, Y + 20 * s + Math.sin(ang2) * rr * 0.5, 3 * s, 3 * s); }
      } else if(st === 'arriving'){
        ph = (t * 1.2) % 1; ctx.strokeStyle = hexA(c, 1 - ph); ctx.lineWidth = Math.max(1, 1.5 * s);
        ctx.beginPath(); ctx.ellipse(X, Y + 72 * s, 10 * s + ph * 20 * s, 5 * s + ph * 10 * s, 0, 0, 6.283); ctx.stroke();
      }
      ctx.restore();
    }

    function roundRect(x, X, Y, w, h, r){
      x.beginPath(); x.moveTo(X + r, Y); x.lineTo(X + w - r, Y); x.quadraticCurveTo(X + w, Y, X + w, Y + r);
      x.lineTo(X + w, Y + h - r); x.quadraticCurveTo(X + w, Y + h, X + w - r, Y + h); x.lineTo(X + r, Y + h);
      x.quadraticCurveTo(X, Y + h, X, Y + h - r); x.lineTo(X, Y + r); x.quadraticCurveTo(X, Y, X + r, Y); x.closePath();
    }

    function drawLabel(a, t, now){
      if(!a.head || a.alpha < 0.3) return;
      var compact = mode === 'panel' || cam.s < 0.33;
      if(compact && hoverId !== a.id) return;
      var d = a.data || {};
      var line1 = shortProject(d.project) + (a.sub ? ' (sub)' : '');
      var stg = STAGES[a.stage] || STAGES.waiting;
      var moving = !!a.pending || (a.walking && a.path && a.path.length > 1);
      var line2 = a.leaving ? 'Leaving' : (moving ? '\u2192 ' : '') + stg.label + (d.tool ? ' \u00b7 ' + d.tool : '');
      var fs = clamp(13 * Math.sqrt(cam.s / 0.45), 10, 16);
      ctx.font = '600 ' + fs + 'px "Segoe UI", system-ui, sans-serif';
      var w1 = ctx.measureText(line1).width;
      ctx.font = '400 ' + (fs - 1) + 'px "Segoe UI", system-ui, sans-serif';
      var w2 = ctx.measureText(line2).width;
      var bw = Math.max(w1, w2) + 16, bh = fs * 2 + 14;
      var X = a.head[0] - bw / 2, Y = a.head[1] - bh - 10 * cam.s - (a.sub ? 0 : 6);
      for(var tries = 0; tries < 8; tries++){
        var clash = null;
        for(var pi = 0; pi < placed.length; pi++){
          var q = placed[pi];
          if(X < q[0] + q[2] && X + bw > q[0] && Y < q[1] + q[3] && Y + bh > q[1]){ clash = q; break; }
        }
        if(!clash) break;
        Y = clash[1] - bh - 3;
      }
      placed.push([X, Y, bw, bh]);
      if(Y + bh < a.head[1] - 14 * cam.s - 8){
        ctx.strokeStyle = hexA(a.color, 0.4 * a.alpha); ctx.lineWidth = 1;
        ctx.beginPath(); ctx.moveTo(a.head[0], a.head[1] - 6 * cam.s); ctx.lineTo(a.head[0], Y + bh); ctx.stroke();
      }
      ctx.globalAlpha = a.alpha;
      ctx.fillStyle = 'rgba(6,10,18,0.82)'; ctx.strokeStyle = hexA(a.color, 0.9); ctx.lineWidth = 1;
      roundRect(ctx, X, Y, bw, bh, 4); ctx.fill(); ctx.stroke();
      ctx.fillStyle = a.color; ctx.fillRect(X, Y + 3, 3, bh - 6);
      ctx.textAlign = 'left'; ctx.textBaseline = 'top';
      ctx.fillStyle = '#e6edf6'; ctx.font = '600 ' + fs + 'px "Segoe UI", system-ui, sans-serif';
      ctx.fillText(line1, X + 9, Y + 4);
      ctx.fillStyle = stg.color; ctx.font = '400 ' + (fs - 1) + 'px "Segoe UI", system-ui, sans-serif';
      ctx.fillText(line2, X + 9, Y + 6 + fs);
      // context bar like an oxygen gauge
      if(d.ctx != null){
        var cpct = clamp(d.ctx, 0, 1), cw = bw - 12;
        ctx.fillStyle = 'rgba(255,255,255,0.08)'; ctx.fillRect(X + 6, Y + bh - 4, cw, 2);
        ctx.fillStyle = cpct > 0.85 ? '#f87171' : cpct > 0.6 ? '#fbbf24' : '#34d399';
        ctx.fillRect(X + 6, Y + bh - 4, cw * cpct, 2);
      }
      ctx.textBaseline = 'alphabetic'; ctx.globalAlpha = 1;
    }

    // ---- HUD (DOM) --------------------------------------------------------------
    function renderHud(){
      if(!hud) return;
      var counts = {}, total = 0;
      Object.keys(agents).forEach(function(id){ var a = agents[id]; if(a.leaving) return; counts[a.stage] = (counts[a.stage] || 0) + 1; total++; });
      var rows = STAGE_ORDER.map(function(k){
        var n = counts[k] || 0;
        return '<div class="bms-row' + (n ? '' : ' bms-zero') + '"><i style="background:' + STAGES[k].color + '"></i><span>' + STAGES[k].label + '</span><b>' + n + '</b></div>';
      }).join('');
      hud.innerHTML =
        '<div class="bms-title">STATION <span>' + total + ' agent' + (total === 1 ? '' : 's') + ' aboard</span></div>' +
        (mode === 'full' ? '<div class="bms-legend">' + rows + '</div>' :
          '<div class="bms-strip">' + STAGE_ORDER.filter(function(k){ return counts[k]; }).map(function(k){ return '<span><i style="background:' + STAGES[k].color + '"></i>' + STAGES[k].label + ' ' + counts[k] + '</span>'; }).join('') + '</div>') +
        '<div class="bms-hint">' + (mode === 'full' ? 'P or Esc to leave' : 'O to leave') + ' \u00b7 drag to pan \u00b7 wheel to zoom \u00b7 double-click to reset</div>';
    }

    function showTip(a, mx, my){
      if(!tip) return;
      if(!a){ tip.style.display = 'none'; return; }
      var d = a.data || {}, stg = STAGES[a.stage] || STAGES.waiting;
      var since = d.since ? fmtAgo(modelNow - d.since) : '';
      tip.innerHTML =
        '<div class="bms-tt-h" style="border-color:' + a.color + '">' + esc(AGENT_LABEL[d.agent] || d.agent || 'agent') + (a.sub ? ' subagent' : '') + '</div>' +
        '<div><span>Project</span>' + esc(d.project || 'n/a') + '</div>' +
        '<div><span>Model</span>' + esc(d.model || 'n/a') + '</div>' +
        '<div><span>Stage</span><b style="color:' + stg.color + '">' + esc(stg.label) + '</b>' + (since ? ' for ' + since : '') + '</div>' +
        (d.tool ? '<div><span>Tool</span>' + esc(d.tool) + '</div>' : '') +
        '<div><span>Tokens</span>' + fmtTokens(d.tokens) + (d.ctx != null ? ' \u00b7 context ' + Math.round(d.ctx * 100) + '%' : '') + '</div>' +
        (opts.onAgentClick ? '<div class="bms-tt-f">click for the latest turn</div>' : '');
      tip.style.display = 'block';
      var tw = tip.offsetWidth, th = tip.offsetHeight;
      tip.style.left = clamp(mx + 14, 4, W - tw - 4) + 'px';
      tip.style.top = clamp(my + 14, 4, H - th - 4) + 'px';
    }

    function hit(mx, my){
      var best = null, bd = Infinity;
      Object.keys(agents).forEach(function(id){
        var a = agents[id]; if(!a.box || a.leaving) return;
        var b = a.box, pad = 6;
        if(mx >= b[0] - pad && mx <= b[0] + b[2] + pad && my >= b[1] - pad && my <= b[1] + b[3] + pad){
          var d = Math.hypot(mx - (b[0] + b[2] / 2), my - (b[1] + b[3] / 2));
          if(d < bd){ bd = d; best = a; }
        }
      });
      return best;
    }

    // ---- input ----------------------------------------------------------------
    function onMove(e){
      var r = canvas.getBoundingClientRect(), mx = e.clientX - r.left, my = e.clientY - r.top;
      if(drag){
        cam.ox = drag.ox + (e.clientX - drag.x); cam.oy = drag.oy + (e.clientY - drag.y);
        cam.user = true; dirtyFloor = true; drag.moved = true; showTip(null); kick(); return;
      }
      var a = hit(mx, my);
      hoverId = a ? a.id : null;
      canvas.style.cursor = a ? 'pointer' : 'grab';
      showTip(a, mx, my);
    }
    function onDown(e){ drag = {x: e.clientX, y: e.clientY, ox: cam.ox, oy: cam.oy, moved: false}; canvas.style.cursor = 'grabbing'; }
    function onUp(e){
      var wasDrag = drag && drag.moved; drag = null; canvas.style.cursor = 'grab';
      if(wasDrag) return;
      var r = canvas.getBoundingClientRect(), a = hit(e.clientX - r.left, e.clientY - r.top);
      if(a && opts.onAgentClick) opts.onAgentClick(a.id, a.data);
    }
    function onWheel(e){
      e.preventDefault();
      var r = canvas.getBoundingClientRect(), mx = e.clientX - r.left, my = e.clientY - r.top;
      var f = Math.exp(-e.deltaY * 0.0015), ns = view === 'top' ? clamp(cam.s * f, 0.6, 9) : clamp(cam.s * f, 0.12, 1.6);
      f = ns / cam.s;
      cam.ox = mx - (mx - cam.ox) * f; cam.oy = my - (my - cam.oy) * f; cam.s = ns;
      cam.user = true; dirtyFloor = true; kick();
    }
    function onDbl(){ fit(); kick(); }
    function onLeave(){ hoverId = null; showTip(null); drag = null; }

    // ---- loop -------------------------------------------------------------------
    // Frames run on a timer at the rate the scene needs, one animation frame
    // each, not an animation frame every display refresh: every request
    // makes WebView2 composite a frame even when the Station skips drawing
    // it, and that cost more than the drawing (measured 2026-10-02: 62
    // requests a second for a 30 fps Station, WebView2 42 percent of a core
    // against 26 at 23 requests; STATUS alpha.9).
    function busy(){
      if(drag || kickUntil > performance.now()) return true;
      for(var id in agents){ var a = agents[id]; if(a.walking || a.pending || a.leaving || a.alpha < 1) return true; }
      return false;
    }
    function loop(){
      timer = setTimeout(function(){
        timer = 0;
        if(!host) return;
        raf = requestAnimationFrame(function(now){
          raf = 0;
          if(!host) return;
          var dt = Math.min(0.2, (now - (lastFrame || now)) / 1000);
          var second = Math.floor(now / 1000) !== Math.floor((lastFrame || now) / 1000);
          lastFrame = now;
          step(dt, now);
          if(view === 'top') drawTop(now); else if(ready) draw(now);
          if(second) renderHud();
          loop();
        });
      }, busy() ? BUSY_MS : IDLE_MS);
    }
    // Input wants the fast rate now, not after the idle wait.
    function kick(){
      kickUntil = performance.now() + 800;
      if(timer){ clearTimeout(timer); timer = 0; loop(); }
    }

    function mount(el, m){
      unmount();
      host = el; mode = m || 'full'; view = mode === 'panel' ? 'top' : 'iso';
      el.classList.add('bms-host');
      canvas = document.createElement('canvas'); canvas.className = 'bms-canvas';
      hud = document.createElement('div'); hud.className = 'bms-hud bms-' + mode;
      tip = document.createElement('div'); tip.className = 'bms-tip';
      el.appendChild(canvas); el.appendChild(hud); el.appendChild(tip);
      ctx = canvas.getContext('2d');
      canvas.addEventListener('mousemove', onMove);
      canvas.addEventListener('mousedown', onDown);
      window.addEventListener('mouseup', onUp);
      canvas.addEventListener('wheel', onWheel, {passive: false});
      canvas.addEventListener('dblclick', onDbl);
      canvas.addEventListener('mouseleave', onLeave);
      cam.user = false;
      resize();
      if(window.ResizeObserver){ ro = new ResizeObserver(resize); ro.observe(el); }
      renderHud();
      lastFrame = 0; loop();
    }

    function unmount(){
      if(raf) cancelAnimationFrame(raf); raf = 0;
      if(timer) clearTimeout(timer); timer = 0;
      if(ro){ ro.disconnect(); ro = null; }
      window.removeEventListener('mouseup', onUp);
      if(host){
        [canvas, hud, tip].forEach(function(n){ if(n && n.parentNode) n.parentNode.removeChild(n); });
        host.classList.remove('bms-host');
      }
      host = null; canvas = ctx = hud = tip = null; floorCv = starCv = propCv = null; propItems = [];
    }

    return {mount: mount, unmount: unmount, update: update,
            isMounted: function(){ return !!host; }, mode: function(){ return host ? mode : null; }};
  }

  // ---- snapshot adapter --------------------------------------------------------
  // Flattens bmLive's Sessions (with nested Subagents) into the update shape.
  // Each Session carries stage/stage_since/stage_tool from internal/stage.
  function fromSnapshot(snap, colorFn){
    var out = [], now = snap && snap.generated_at ? Date.parse(snap.generated_at) : Date.now();
    function walk(list, parent){
      (list || []).forEach(function(s){
        var id = s.session_id;
        out.push({
          id: id, parent: parent, agent: s.agent, project: s.project, model: s.model,
          color: colorFn ? colorFn(id, s.agent) : null,
          stage: s.stage || 'waiting', since: s.stage_since ? Date.parse(s.stage_since) : null,
          tool: s.stage_tool || '', tokens: s.tokens,
          ctx: s.context_window ? s.context / s.context_window : null
        });
        walk(s.subagents, id);
      });
    }
    walk(snap && snap.sessions, null);
    return {now: now, sessions: out};
  }

  // ---- styles -------------------------------------------------------------------
  var css =
    '.bms-host{position:relative;overflow:hidden;background:#03050a}' +
    '.bms-canvas{position:absolute;inset:0;display:block;cursor:grab}' +
    '.bms-hud{position:absolute;left:14px;top:12px;pointer-events:none;font:12px "Segoe UI",system-ui,sans-serif;color:#cbd5e1}' +
    '.bms-hud.bms-panel{left:8px;top:6px}' +
    '.bms-title{font-weight:700;letter-spacing:.18em;color:#7dd3fc;font-size:13px;text-shadow:0 0 8px rgba(56,189,248,.6)}' +
    '.bms-title span{letter-spacing:.04em;font-weight:400;color:#94a3b8;margin-left:8px;text-shadow:none}' +
    '.bms-legend{margin-top:8px;background:rgba(6,10,18,.72);border:1px solid rgba(125,211,252,.18);padding:6px 9px;border-radius:4px;min-width:170px}' +
    '.bms-row{display:flex;align-items:center;gap:7px;line-height:19px}' +
    '.bms-row i{width:8px;height:8px;border-radius:2px;display:inline-block;box-shadow:0 0 6px currentColor}' +
    '.bms-row span{flex:1}.bms-row b{font-variant-numeric:tabular-nums;color:#e2e8f0}' +
    '.bms-zero{opacity:.38}' +
    '.bms-strip{margin-top:3px;display:flex;flex-wrap:wrap;gap:4px 10px;font-size:11px}' +
    '.bms-strip i{width:7px;height:7px;border-radius:50%;display:inline-block;margin-right:4px}' +
    '.bms-hint{position:fixed;right:14px;bottom:10px;color:#64748b;font-size:11px}' +
    '.bms-panel .bms-hint{position:fixed;display:none}' +
    '.bms-tip{position:absolute;display:none;pointer-events:none;background:rgba(6,10,18,.94);border:1px solid #1c2532;color:#cbd5e1;font:12px "Segoe UI",system-ui,sans-serif;padding:8px 10px;border-radius:4px;min-width:190px;z-index:5}' +
    '.bms-tip div{line-height:18px}.bms-tip span{display:inline-block;width:58px;color:#64748b}' +
    '.bms-tt-h{font-weight:700;color:#e2e8f0;border-left:3px solid;padding-left:6px;margin-bottom:4px}' +
    '.bms-tt-f{color:#64748b;font-size:11px;margin-top:4px}';
  var styleEl = document.createElement('style'); styleEl.textContent = css;
  (document.head || document.documentElement).appendChild(styleEl);

  window.BMStation = {create: create, fromSnapshot: fromSnapshot, STAGES: STAGES, ROOMS: ROOMS};
})();
