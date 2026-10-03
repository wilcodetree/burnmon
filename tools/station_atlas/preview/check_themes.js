// Theme self-check for station.js (no browser needed): node check_themes.js
// Loads station.js and the atlases into a fake window and asserts that the
// site theme keeps the space theme's footprint tile for tile, that every
// site sprite is a documented name, and that every room has a site name.
'use strict';
var fs = require('fs'), path = require('path'), vm = require('vm');
var ST = path.join(__dirname, '..', '..', '..', 'cmd', 'burnmon-dev', 'station');
var fails = 0, warns = 0;
function ok(cond, msg){ if(cond) console.log('ok    ' + msg); else { fails++; console.log('FAIL  ' + msg); } }
function warn(msg){ warns++; console.log('WARN  ' + msg); }

global.window = {};
global.document = {createElement: function(){ return {textContent: ''}; }, head: {appendChild: function(){}}};
function load(f){ var p = path.join(ST, f); if(!fs.existsSync(p)) return false; vm.runInThisContext(fs.readFileSync(p, 'utf8'), {filename: p}); return true; }
load('station_atlas.js');
load('station_atlas_site.js');
var haveSite = !!window.BM_STATION_ATLAS_SITE;   // the file can be a placeholder that sets null
load('station.js');
var B = window.BMStation, SPACE = B.THEME_DEF.space, SITE = B.THEME_DEF.site, GX = B.GRID[0], GY = B.GRID[1];

var DOC = ('site_fence_SW site_fence_NW site_light_SE site_shelf_SE site_board_SW site_cone_SE site_cones_SE site_truck_SW site_crates_SE site_sign_SW ' +
  'site_desk_SW site_desk_SE site_chair_NE site_chair_NW site_chair_SE site_chair_SW site_table_SE site_barrier_SW site_barricade_SE site_wall_half_SW ' +
  'site_bricks_SE site_scaffold_SE site_excavator_SW site_excavator_SE site_hut_SE site_frame_SW site_frame_SE site_screen_SW site_generator_SE site_generator_SW site_pallet_SE ' +
  'site_container_SE site_skip_SE site_bin_SE site_car_SW site_van_SW site_crane_SE site_tank_SE site_loader_NE site_loader_NW site_loader_SE site_loader_SW').split(' ');
var DIR8 = ['N', 'NE', 'E', 'SE', 'S', 'SW', 'W', 'NW'];
['workerA', 'workerB', 'workerC'].forEach(function(w){ DIR8.forEach(function(d){ DOC.push(w + '_' + d + '_0', w + '_' + d + '_1'); }); });

ok(JSON.stringify(B.THEMES) === '["site","space"]', 'BMStation.THEMES is ["site","space"]');
ok(B.SITE_UNMAPPED.length === 0, 'every space prop has a site mapping' + (B.SITE_UNMAPPED.length ? ': ' + B.SITE_UNMAPPED.join(', ') : ''));

function grid(props){ var g = new Uint8Array(GX * GY); props.forEach(function(p){ if(p[3]) g[p[2] * GX + p[1]] = 1; }); return g; }
var gs = grid(SPACE.props), gt = grid(SITE.props), same = true, n = 0;
for(var i = 0; i < gs.length; i++){ if(gs[i] !== gt[i]) same = false; n += gs[i]; }
ok(same && n > 0, 'site blocked grid equals space blocked grid (' + n + ' blocked tiles)');
var tiles = SPACE.props.length === SITE.props.length && SPACE.props.every(function(p, k){ var q = SITE.props[k]; return p[1] === q[1] && p[2] === q[2] && p[3] === q[3] && p[5] === q[5]; });
ok(tiles, 'site props match space props one to one (' + SITE.props.length + ' props: tile, blocks, iso flag)');

var used = {};
SITE.props.forEach(function(p){ used[p[0]] = 1; });
used[SITE.wall[0]] = used[SITE.wall[1]] = used[SITE.pylon] = 1;
['NE', 'NW', 'SE', 'SW'].forEach(function(d){ used[SITE.rover[0] + d] = 1; });
var bad = Object.keys(used).filter(function(nm){ return DOC.indexOf(nm) < 0; });
ok(bad.length === 0, 'every site sprite name is documented (' + Object.keys(used).length + ' used)' + (bad.length ? ': ' + bad.join(', ') : ''));
ok(Object.keys(SITE.vendor).every(function(k){ return /^worker[ABC]$/.test(SITE.vendor[k]); }) && /^worker[ABC]$/.test(SITE.vendorDef), 'site vendors map to workerA|B|C');

var rooms = Object.keys(B.ROOMS);
ok(rooms.length === 12 && rooms.every(function(r){ return SITE.names[r] && SITE.names[r].length <= 14 && SPACE.names[r]; }), 'all ' + rooms.length + ' rooms have a site name of at most 14 characters');
console.log('      ' + rooms.map(function(r){ return r + '=' + SITE.names[r]; }).join(', '));

// every used prop name has a pixel drawing in pxSite (a regex on name in station.js)
var src = fs.readFileSync(path.join(ST, 'station.js'), 'utf8'), fn = src.slice(src.indexOf('function pxSite('), src.indexOf('function buildPixel('));
var pats = []; fn.replace(/\/\^(site_[a-z_]+)\/\.test\(name\)/g, function(m, g){ pats.push(g); });
var nopx = Object.keys(used).filter(function(nm){ return !/^site_(fence|light|loader)/.test(nm) && !pats.some(function(pt){ return nm.indexOf(pt) === 0; }); });
ok(nopx.length === 0, 'every used prop has a plan-view drawing (pxSite)' + (nopx.length ? ': ' + nopx.join(', ') : ''));

if(haveSite){
  var A = window.BM_STATION_ATLAS_SITE, S = window.BM_STATION_ATLAS;
  ok(JSON.stringify(A.proj) === JSON.stringify(S.proj), 'site atlas proj equals the space atlas proj');
  var miss = Object.keys(used).filter(function(nm){ return !A.map[nm]; });
  if(miss.length) warn('site atlas lacks: ' + miss.join(', '));
  var wm = []; ['workerA', 'workerB', 'workerC'].forEach(function(w){ DIR8.forEach(function(d){ [0, 1].forEach(function(f){ if(!A.map[w + '_' + d + '_' + f]) wm.push(w + '_' + d + '_' + f); }); }); });
  if(wm.length) warn('site atlas lacks ' + wm.length + ' worker frames, e.g. ' + wm.slice(0, 3).join(', '));
} else warn('no site atlas yet (missing or placeholder): sprite names not checked against it');

// ---- Station UI pass of 2026-10-03 (02_roadmap\2026-10-03_station_ui_and_backlog.md, step 1)
var RWT = B.ROOM_W;
// 1. room names: bottom of the room, centred, and never on an object of that room.
// LABEL_W is each name's width in tiles at full size (Segoe UI Black 800, 58 px, both themes,
// measured with Pillow); labelSpot(id, width, minScale) must keep every one off every prop tile
// and plant corner of its room, in the space and the site theme (same tiles), and inside the room.
var LABEL_W = {space: {archive: 2.61, uplink: 2.19, plan: 3.6, airlock: 2.56, fab: 3.77, think: 3.66, test: 4.52, brief: 2.72, lounge: 2.39, recycler: 2.87, cryo: 2.97, core: 1.53},
  site: {archive: 5.09, uplink: 4.49, plan: 3.33, airlock: 4.54, fab: 3.54, think: 4.91, test: 3.55, brief: 4.49, lounge: 2.76, recycler: 1.31, cryo: 2.71, core: 2}};
var spots = {}, onObj = [], outside = [], moved = [], notBottom = [], uniform = {};
['space', 'site'].forEach(function(th){
  var lay = B.labelLayout(LABEL_W[th], 0.6), need = 1;
  Object.keys(B.ROOMS).forEach(function(r){ need = Math.min(need, B.labelSpot(r, LABEL_W[th][r], 0.6, 1).scale); });
  uniform[th] = Math.abs(lay.scale - need) < 1e-9 ? '' : th + ': layout scale ' + lay.scale + ' but the smallest any room needs is ' + need;
  Object.keys(B.ROOMS).forEach(function(r){
    var R = B.ROOMS[r], sp = lay.spots[r], half = LABEL_W[th][r] * lay.scale / 2;
    spots[r + ':' + th] = sp;
    if(sp.cx - half < 0 || sp.cx + half > RWT) outside.push(r + ':' + th);
    var props = (th === 'space' ? SPACE : SITE).props.filter(function(p){ return p[1] >= R.x0 && p[1] < R.x0 + RWT && p[2] >= R.y0 && p[2] < R.y0 + RWT && p[5] !== 'iso'; });
    props.forEach(function(p){
      var tx = p[1] - R.x0, ty = p[2] - R.y0;
      if(ty === sp.row && tx < sp.cx + half && tx + 1 > sp.cx - half) onObj.push(r + ':' + th + ' on ' + p[0] + '@' + tx + ',' + ty);
    });
    B.PLANT_TILES(r).forEach(function(t){ if(t[1] === sp.row && t[0] < sp.cx + half && t[0] + 1 > sp.cx - half) onObj.push(r + ':' + th + ' on a plant'); });
    if(lay.scale < 0.6 - 1e-9) notBottom.push(th + ' scale ' + lay.scale);
    if(sp.row !== RWT - 1) moved.push(r + ':' + th + ' row ' + sp.row);
  });
});
ok(!uniform.space && !uniform.site, 'all room names share one size, the smallest any room needs (space and site)' + (uniform.space || uniform.site ? ': ' + [uniform.space, uniform.site].filter(Boolean).join('; ') : ''));
ok(onObj.length === 0, 'no room name sits on a prop or plant of its room, space and site, 24 names' + (onObj.length ? ': ' + onObj.join('; ') : ''));
ok(outside.length === 0 && notBottom.length === 0, 'every room name fits inside its room at 60 percent size or more' + (outside.concat(notBottom).length ? ': ' + outside.concat(notBottom).join('; ') : ''));
ok(moved.length === 0, 'every room name stays on the bottom row of its room' + (moved.length ? ': ' + moved.join('; ') : ''));
ok(Object.keys(B.ROOMS).filter(function(r){ return r !== 'lounge' && r !== 'core'; }).every(function(r){ return spots[r + ':space'].cx === RWT / 2 && spots[r + ':site'].cx === RWT / 2; }), 'ten room names are centred on their room in both themes');
ok(['lounge', 'core'].every(function(r){ return spots[r + ':space'].cx > RWT / 2 + 0.5 && spots[r + ':site'].cx > RWT / 2 + 0.5; }), 'Lounge and Core names sit right of centre in both themes');
ok((src.match(/labelLayout\(/g) || []).length >= 3, 'both views (floor labels, plan labels) place room names with labelLayout');
// 2. one rover or loader only: the one that kept to the lounge is gone
ok(B.ROVER_STARTS.length === 1 && !B.ROVER_STARTS[0].area, 'one rover or loader only, the corridor patroller (none keeps to the lounge)');
// 3. uplink: a chair at the middle table, off the slots, one shared blocked grid
var UP = B.ROOMS.uplink, UCX = UP.x0 + 3, UCY = UP.y0 + 4;
var uch = SPACE.props.filter(function(p){ return p[1] === UCX && p[2] === UCY; }), usch = SITE.props.filter(function(p){ return p[1] === UCX && p[2] === UCY; });
ok(uch.length === 1 && uch[0][0] === 'desk_chair_NE' && uch[0][3] === 0, 'uplink has a desk_chair_NE that does not block at the middle table (tile 3,4)');
ok(usch.length === 1 && usch[0][0] === 'site_chair_NE' && usch[0][3] === 0, 'uplink site theme has the matching site_chair_NE at the same tile');
var uslots = B.SLOTS.uplink;
ok(!uslots.some(function(s){ return s[0] === UCX && s[1] === UCY; }) && uslots.some(function(s){ return s[0] === UP.x0 + 2 && s[1] === UP.y0 + 3; }) && uslots.some(function(s){ return s[0] === UP.x0 + 4 && s[1] === UP.y0 + 3; }), 'the new chair tile is no slot, slots [2,3] and [4,3] remain');
// 5. seated pose on chair slots (site theme): the chair's facing picks the sprite
ok(B.chairFace(UP.x0 + 1, UP.y0 + 6) === 'NE' && B.chairFace(B.ROOMS.lounge.x0 + 0, B.ROOMS.lounge.y0 + 2) === 'SE' && B.chairFace(UP.x0 + 2, UP.y0 + 3) === null, 'chairFace reads the facing of a chair tile and null elsewhere');
ok(B.seatFace(UP.x0 + 1.5, UP.y0 + 6.5) === 'NE' && B.seatFace(UP.x0 + 1.9, UP.y0 + 6.5) === null && B.seatFace(UP.x0 + 1.5, UP.y0 + 6.1) === null && B.seatFace(UP.x0 + 2.5, UP.y0 + 3.5) === null, 'seatFace seats an agent standing at the centre of a chair tile (slots are tile centres), not one walking across it');
ok(B.seatedSprite('workerA', 'NE') === 'workerA_sit_NE', 'seated worker sprite is <worker>_sit_<chair facing>');
ok(B.SEATS.length > 0 && B.SEATS.every(function(sl){ return B.seatFace(sl[0] + 0.5, sl[1] + 0.5) !== null; }), 'every slot that has a chair seats its agent (' + B.SEATS.length + ' seats)');
if(haveSite){
  var sitMiss = []; ['workerA', 'workerB', 'workerC'].forEach(function(w){ ['NE', 'NW', 'SE', 'SW'].forEach(function(d){ if(!window.BM_STATION_ATLAS_SITE.map[w + '_sit_' + d]) sitMiss.push(w + '_sit_' + d); }); });
  ok(sitMiss.length === 0, 'site atlas has the seated worker in all 12 sprites' + (sitMiss.length ? ', missing ' + sitMiss.slice(0, 3).join(', ') + (sitMiss.length > 3 ? ' ...' : '') : ''));
}

// ---- second round, 2026-10-03: site theme layout, hologram, plan table rug
function siteAt(room, lx, ly){ return SITE.props.filter(function(p){ return p[1] === B.ROOMS[room].x0 + lx && p[2] === B.ROOMS[room].y0 + ly; }).map(function(p){ return p[0]; }).join(','); }
function siteIn(room){ var R = B.ROOMS[room]; return SITE.props.filter(function(p){ return p[1] >= R.x0 && p[1] < R.x0 + RWT && p[2] >= R.y0 && p[2] < R.y0 + RWT && p[5] !== 'iso'; }).map(function(p){ return p[0]; }); }
ok(siteAt('lounge', 0, 6) === 'site_crates_SE' && siteAt('lounge', 6, 6) === 'site_crates_SE' && siteIn('lounge').indexOf('site_container_SE') < 0, 'canteen has crates in both bottom corners and no container');
ok(siteIn('airlock').filter(function(n){ return n === 'site_container_SE'; }).length === 1, 'the container stands at the site entrance (once)');
ok(siteAt('fab', 0, 0) === 'site_excavator_SE' && siteAt('fab', 6, 0) === 'site_excavator_SW' && siteIn('fab').indexOf('site_scaffold_SE') < 0, 'build zone has an excavator in both top corners, no scaffold tower');
ok([1, 3, 5].every(function(x){ return siteAt('test', x, 2) === 'site_frame_SE'; }), 'inspection frames are turned 90 degrees (site_frame_SE) to line up like the station gates');
if(haveSite){
  var nm2 = ['site_excavator_SE', 'site_frame_SE'].filter(function(n){ return !window.BM_STATION_ATLAS_SITE.map[n]; });
  ok(nm2.length === 0, 'site atlas has site_excavator_SE and site_frame_SE' + (nm2.length ? ', missing ' + nm2.join(', ') : ''));
}
var HO = B.HOLO, HOLD = {coneIdle: 0.12, coneBusy: 0.35, cardIdle: 0.35, cardBusy: 0.85, planIdle: 0.4, planBusy: 0.9};
ok(Object.keys(HOLD).every(function(k){ return HO && HO[k] > HOLD[k]; }), 'the plan table holograms are brighter than before in every state (cone, cards, plan view)');
var rug = src.slice(src.indexOf('blueprint rug under the table'), src.indexOf('// outer hull'));
ok(rug.length > 50 && rug.indexOf('lineTo(X0 + g + 5.5') < 0, 'plan view: the L marks along the top of the blueprint rug are gone');

console.log(fails ? fails + ' failure(s)' : 'all checks passed' + (warns ? ' (' + warns + ' warning(s))' : ''));
process.exit(fails ? 1 : 0);
