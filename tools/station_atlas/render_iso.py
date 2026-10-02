# Renders Kenney Space Kit 2.0 models (CC0, https://kenney.nl/assets/space-kit)
# as isometric sprites at a chosen camera elevation, for build_atlas.py. The
# kit's own Isometric PNGs are fixed at 30 degrees; the Station's full view
# (P) uses a lower camera (Wilco, 2026-10-01: about 15 degrees, more of the
# front). Run headless from this folder:
#   blender -b -P render_iso.py -- <unzipped kit> <out dir> <elevation deg> [name ...]
# Each output is <name>_<dir>.png, 512 px wide, a 1x1 tile 130 px wide and
# its centre at (ANCHOR_X, anchor_y(elev)), so build_atlas.py can anchor
# every sprite on a tile centre the same way it does for the kit's renders.
import math, os, sys
import json
import bpy
from bpy_extras.object_utils import world_to_camera_view
from mathutils import Vector

argv = sys.argv[sys.argv.index('--') + 1:]
KIT, OUT, ELEV = argv[0], argv[1], float(argv[2])
ONLY = set(argv[3:])

SIZE = 512
TILE_PX = 130.0                       # a 1x1 tile's diamond width, as in the kit's renders
ANCHOR_X = 256.0
# Kenney's 30 degree renders put the tile centre at y=311.5 of 512. A lower
# camera shows less floor and more height, so the centre moves down to keep
# tall models in frame: the same distance below the image centre, scaled by
# cos(elev) / cos(30) (how tall a vertical metre renders).
def anchor_y(elev):
    return 256.0 + (311.5 - 256.0) * math.cos(math.radians(elev)) / math.cos(math.radians(30))

# Kenney direction suffixes as a rotation of the model about the vertical
# axis, degrees. Calibrated 2026-10-01 against the kit's own 30 degree PNGs
# (calibrate_iso.py, a grid over sign and offset): sign -1, offset 45 and a
# 4.5 px anchor nudge give mean IoU 0.906 on 20 samples, 0.954 over all 100
# once build_atlas.py registers each sprite's shift. The environment
# variables only exist to repeat that search.
YAW = {'N': 0, 'NE': 45, 'E': 90, 'SE': 135, 'S': 180, 'SW': 225, 'W': 270, 'NW': 315}
YAW_OFFSET = float(os.environ.get('BMS_YAW_OFFSET', '45'))
YAW_SIGN = float(os.environ.get('BMS_YAW_SIGN', '-1'))
DY = float(os.environ.get('BMS_DY', '4.5'))             # px, nudges the anchor down
AZIMUTH = float(os.environ.get('BMS_AZIMUTH', '45'))   # camera azimuth, degrees; the kit's is 45
# BMS_SHIFTS: shifts_iso.py's per-sprite pixel shifts, measured at elevation
# 30 and azimuth 45 against the kit's PNGs; each becomes a ground offset.
SHIFTS = json.load(open(os.environ['BMS_SHIFTS'])) if os.environ.get('BMS_SHIFTS') else {}

D4 = ['NE', 'NW', 'SE', 'SW']
D8 = ['N', 'NE', 'E', 'SE', 'S', 'SW', 'W', 'NW']
JOBS = []
def add(base, dirs):
    JOBS.extend((base, d) for d in dirs)
add('corridor_wall', D4); add('corridor_wallCorner', D4)
add('gate_complex', D4); add('gate_simple', D4)
add('desk_computer', D4); add('desk_computerScreen', D4); add('desk_computerCorner', D4)
add('desk_chair', D4); add('desk_chairArms', D4); add('desk_chairStool', ['SE'])
add('structure_closed', ['SE']); add('structure_detailed', ['SE']); add('structure', ['SE'])
add('satelliteDish_large', ['SE', 'SW']); add('satelliteDish', ['SE', 'SW']); add('satelliteDish_detailed', ['SE', 'SW'])
add('machine_wireless', ['SE', 'SW']); add('machine_wirelessCable', ['SE', 'SW'])
add('platform_low', ['SE']); add('platform_small', ['SE']); add('platform_center', ['SE'])
add('machine_generator', ['SE', 'SW']); add('machine_generatorLarge', ['SE', 'SW'])
add('machine_barrel', ['SE', 'SW']); add('machine_barrelLarge', ['SE', 'SW'])
add('barrels', ['SE']); add('barrel', ['SE']); add('pipe_ringSupport', ['SE', 'SW'])
add('rail', D4); add('rail_middle', ['SE', 'SW']); add('supports_low', ['SE']); add('rover', D4)
for a in ['astronautA', 'astronautB', 'alien']:
    add(a, D8)


def reset():
    bpy.ops.wm.read_factory_settings(use_empty=True)
    sc = bpy.context.scene
    sc.render.engine = 'BLENDER_EEVEE_NEXT' if 'BLENDER_EEVEE_NEXT' in [e.identifier for e in bpy.types.RenderSettings.bl_rna.properties['engine'].enum_items] else 'BLENDER_EEVEE'
    sc.render.resolution_x = sc.render.resolution_y = SIZE
    sc.render.film_transparent = True
    sc.render.image_settings.file_format = 'PNG'
    sc.render.image_settings.color_mode = 'RGBA'
    sc.view_settings.view_transform = 'Standard'
    w = bpy.data.worlds.new('w'); sc.world = w
    w.use_nodes = True
    bg = w.node_tree.nodes['Background']
    bg.inputs[0].default_value = (1, 1, 1, 1)
    bg.inputs[1].default_value = float(os.environ.get('BMS_AMBIENT', '0.55'))
    sun = bpy.data.lights.new('sun', 'SUN'); sun.energy = float(os.environ.get('BMS_SUN', '2.2'))
    so = bpy.data.objects.new('sun', sun); sc.collection.objects.link(so)
    so.rotation_euler = (math.radians(40), math.radians(15), math.radians(AZIMUTH - 20))
    cam = bpy.data.cameras.new('cam'); cam.type = 'ORTHO'
    # A 1x1 tile's diagonal (sqrt 2 world units) spans TILE_PX of SIZE px.
    cam.ortho_scale = SIZE * math.sqrt(2) / TILE_PX
    co = bpy.data.objects.new('cam', cam); sc.collection.objects.link(co); sc.camera = co
    return sc, co


def place_camera(co, elev, azimuth=None):
    e, az = math.radians(elev), math.radians(AZIMUTH if azimuth is None else azimuth)
    d = 50.0
    target = Vector((0, 0, 0))
    co.location = target + Vector((d * math.cos(e) * math.sin(az), -d * math.cos(e) * math.cos(az), d * math.sin(e)))
    co.rotation_euler = (target - co.location).to_track_quat('-Z', 'Y').to_euler()
    # Shift the frame so the world origin (the tile centre) lands on the anchor.
    cam = co.data
    cam.shift_x = -(ANCHOR_X - SIZE / 2) / SIZE
    cam.shift_y = (anchor_y(elev) + DY - SIZE / 2) / SIZE


def axes(sc, co):
    # Pixels per world unit along world X and Y: [[sx/X, sx/Y], [sy/X, sy/Y]].
    bpy.context.view_layer.update()
    def px(p):
        v = world_to_camera_view(sc, co, Vector(p))
        return (v.x * SIZE, (1 - v.y) * SIZE)
    o, a, b = px((0, 0, 0)), px((1, 0, 0)), px((0, 1, 0))
    return [[a[0] - o[0], b[0] - o[0]], [a[1] - o[1], b[1] - o[1]]]


def solve(m, dx, dy):
    det = m[0][0] * m[1][1] - m[0][1] * m[1][0]
    return ((dx * m[1][1] - dy * m[0][1]) / det, (m[0][0] * dy - m[1][0] * dx) / det)


def render(base, d):
    sc, co = reset()
    path = os.path.join(KIT, 'Models', 'GLTF format', base + '.glb')
    bpy.ops.import_scene.gltf(filepath=path)
    bpy.context.view_layer.update()
    tops = [o for o in sc.objects if o.parent is None and o.type not in ('CAMERA', 'LIGHT')]
    # Centre the footprint (the meshes' ground bounding box) on the origin,
    # which is the tile centre the anchor marks; height stays as modelled.
    xs, ys = [], []
    for o in sc.objects:
        if o.type == 'MESH':
            for c in o.bound_box:
                w = o.matrix_world @ Vector(c)
                xs.append(w.x); ys.append(w.y)
    cx, cy = (min(xs) + max(xs)) / 2, (min(ys) + max(ys)) / 2
    root = bpy.data.objects.new('root', None); sc.collection.objects.link(root)
    for o in tops:
        o.location.x -= cx; o.location.y -= cy
        o.parent = root
    root.rotation_euler = (0, 0, math.radians(YAW_SIGN * YAW[d] + YAW_OFFSET))
    sh = SHIFTS.get(base + '_' + d)
    if sh:
        place_camera(co, 30, 45)
        u, v = solve(axes(sc, co), sh[0], sh[1])
        root.location = (u, v, 0)
    place_camera(co, ELEV)
    sc.render.filepath = os.path.join(OUT, base + '_' + d + '.png')
    bpy.ops.render.render(write_still=True)


def write_proj():
    # The deck's tile axes on screen for this camera, as station.js uses
    # them: station x and y are the world axes whose images at the kit's
    # own view (30, 45) are (TW/2, TH/2) and (-TW/2, TH/2).
    sc, co = reset()
    place_camera(co, 30, 45)
    m45 = axes(sc, co)
    want = [(65.0, 32.5), (-65.0, 32.5)]
    pick = []
    for wx, wy in want:
        best = None
        for i in (0, 1):
            for sg in (1, -1):
                err = abs(sg * m45[0][i] - wx) + abs(sg * m45[1][i] - wy)
                if best is None or err < best[0]:
                    best = (err, i, sg)
        pick.append(best)
    place_camera(co, ELEV)
    m = axes(sc, co)
    proj = [pick[0][2] * m[0][pick[0][1]], pick[1][2] * m[0][pick[1][1]],
            pick[0][2] * m[1][pick[0][1]], pick[1][2] * m[1][pick[1][1]]]
    out = {'elev': ELEV, 'azimuth': AZIMUTH, 'proj': [round(v, 3) for v in proj],
           'fit_error_px_at_45': round(pick[0][0] + pick[1][0], 3)}
    json.dump(out, open(os.path.join(OUT, 'proj.json'), 'w', newline='\n'), indent=1)
    print('proj', out)


os.makedirs(OUT, exist_ok=True)
write_proj()
for base, d in JOBS:
    if ONLY and base not in ONLY and (base + '_' + d) not in ONLY:
        continue
    render(base, d)
print('rendered into', OUT)
