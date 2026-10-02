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
  'site_bricks_SE site_scaffold_SE site_excavator_SW site_hut_SE site_frame_SW site_screen_SW site_generator_SE site_generator_SW site_pallet_SE ' +
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

console.log(fails ? fails + ' failure(s)' : 'all checks passed' + (warns ? ' (' + warns + ' warning(s))' : ''));
process.exit(fails ? 1 : 0);
