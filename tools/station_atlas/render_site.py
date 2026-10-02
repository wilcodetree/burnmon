# Renders the construction-site sprites for the Station's site theme from
# site_manifest.json, with the exact camera render_iso.py uses for the space
# atlas: this file executes render_iso.py's definitions (reset, place_camera,
# write_proj, YAW, YAW_SIGN, YAW_OFFSET, DY, TILE_PX, SIZE, the anchor) instead
# of copying them, so both atlases share one calibrated projection. Run
# headless from this folder, with the kits unzipped under kits\ (see CREDITS.txt):
#   blender -b -P render_site.py -- site_manifest.json <out dir> [name ...]
# Camera: elevation 30, azimuth 30 (BMS_AZIMUTH defaults to 30 here). Writes
# <name>_<dir>.png (workers <name>_<dir>_<frame>.png) and proj.json into <out dir>.
#
# Manifest: {"entries": {<base name>: spec}}. A spec has
#   file     path under kits\ (.glb .gltf .fbx .obj .blend), or "proc:<name>"
#            for a prop built in site_proc.py; or "parts": [spec, ...], each part
#            with its own file, size, "pos": [x, y, z] (tiles, station axes
#            x, y, z up), yaw, recolour; the entry's own size then fits the group
#   dirs     direction suffixes to render, e.g. ["SE", "SW"]
#   size     {"height": h} | {"long": L} | {"x": w} | {"y": w}, in tiles (a tile
#            is 1 unit), measured before the yaw, on the frame-0 pose
#   yaw      degrees added to the direction's yaw (the model's native facing)
#   per_dir  {"SW": {"yaw_abs": 90, "offset": [x, y]}}: per direction, an
#            absolute yaw instead of the formula, and a ground offset in tiles
#            (station x, y) from the tile centre, for edge pieces like the fence
#   recolour {material name or "*": "#rrggbb"}; replaces the material
#   frames   animation frames to render (sprite suffix _0, _1 ...), with
#   action   the action name to play; frame is a single still frame for a
#            .blend scene with animated parts
#   exclude  object names to drop from a .blend
# Pure ASCII.
import json, math, os, sys
import bpy
from mathutils import Vector

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)
sys.dont_write_bytecode = True
import site_proc

argv = sys.argv[sys.argv.index('--') + 1:]
MANIFEST, OUT = os.path.abspath(argv[0]), os.path.abspath(argv[1])
ONLY = set(argv[2:])
KITS = os.environ.get('BMS_KITS') or os.path.join(os.path.dirname(MANIFEST), 'kits')
ELEV = 30.0
os.environ.setdefault('BMS_AZIMUTH', '30')

# render_iso.py's definitions: everything above its job loop, executed here.
_src = open(os.path.join(HERE, 'render_iso.py'), encoding='utf-8').read()
_cut = _src.index('os.makedirs(OUT, exist_ok=True)')
_saved = sys.argv
sys.argv = ['blender', '--', KITS, OUT, str(ELEV)]
RI = {'__name__': 'render_iso'}
exec(compile(_src[:_cut], 'render_iso.py', 'exec'), RI)
sys.argv = _saved
YAW, YAW_SIGN, YAW_OFFSET = RI['YAW'], RI['YAW_SIGN'], RI['YAW_OFFSET']
used_files = set()


def world_bounds(objs, dg):
    lo, hi = Vector((1e9, 1e9, 1e9)), Vector((-1e9, -1e9, -1e9))
    for o in objs:
        if o.type != 'MESH':
            continue
        eo = o.evaluated_get(dg)
        me = eo.to_mesh()
        for v in me.vertices:
            w = eo.matrix_world @ v.co
            lo = Vector((min(lo.x, w.x), min(lo.y, w.y), min(lo.z, w.z)))
            hi = Vector((max(hi.x, w.x), max(hi.y, w.y), max(hi.z, w.z)))
        eo.to_mesh_clear()
    return lo, hi


def all_children(o):
    for c in o.children:
        yield c
        yield from all_children(c)


def simple_material(m, hexcol):
    m.use_nodes = True
    nt = m.node_tree
    nt.nodes.clear()
    b = nt.nodes.new('ShaderNodeBsdfPrincipled')
    out = nt.nodes.new('ShaderNodeOutputMaterial')
    b.inputs['Base Color'].default_value = site_proc.rgb(hexcol) + (1.0,)
    b.inputs['Roughness'].default_value = 0.75
    nt.links.new(b.outputs[0], out.inputs[0])
    m.diffuse_color = site_proc.rgb(hexcol) + (1.0,)


def recolour(objs, table):
    if not table:
        return
    seen = set()
    for o in objs:
        for slot in getattr(o, 'material_slots', []):
            m = slot.material
            if m is None or m.name in seen:
                continue
            seen.add(m.name)
            base = m.name.split('.')[0] if m.name[-4:-3] == '.' else m.name
            col = table.get(m.name) or table.get(base) or table.get('*')
            if col:
                simple_material(m, col)


def bake(frame, loaded):
    """Freezes a .blend scene's animated, constraint-driven parts at one frame
    into plain world-space meshes, so they scale and rotate as one model."""
    sc = bpy.context.scene
    sc.frame_set(int(frame))
    bpy.context.view_layer.update()
    dg = bpy.context.evaluated_depsgraph_get()
    olds = [o for o in loaded if o.type == 'MESH']
    news = []
    for o in olds:
        eo = o.evaluated_get(dg)
        me = bpy.data.meshes.new_from_object(eo)
        me.transform(eo.matrix_world)
        n = bpy.data.objects.new(o.name + '_b', me)
        sc.collection.objects.link(n)
        news.append(n)
    for o in loaded:
        bpy.data.objects.remove(o)


def load(file, spec):
    before = set(bpy.data.objects)
    if file.startswith('proc:'):
        site_proc.BUILD[file[5:]]()
    else:
        path = os.path.join(KITS, file)
        used_files.add(file.replace('\\', '/'))
        ext = os.path.splitext(path)[1].lower()
        if ext in ('.glb', '.gltf'):
            bpy.ops.import_scene.gltf(filepath=path)
        elif ext == '.fbx':
            bpy.ops.import_scene.fbx(filepath=path)
        elif ext == '.obj':
            bpy.ops.wm.obj_import(filepath=path)
        elif ext == '.blend':
            with bpy.data.libraries.load(path) as (src, dst):
                dst.objects = [n for n in src.objects if n not in spec.get('exclude', [])]
            loaded = [o for o in dst.objects if o is not None and o.type not in ('CAMERA', 'LIGHT')]
            for o in loaded:
                bpy.context.scene.collection.objects.link(o)
            bake(spec.get('frame', 1), loaded)
        else:
            raise SystemExit('unsupported model type: ' + file)
    # skips glTF bone shapes, which sit in a collection that is not rendered
    new = [o for o in bpy.data.objects if o not in before and not o.hide_render
           and all(not c.hide_render for c in o.users_collection)]
    drop = [o for o in new if o.type in ('CAMERA', 'LIGHT')]
    new = [o for o in new if o.type not in ('CAMERA', 'LIGHT')]
    for o in drop:
        bpy.data.objects.remove(o)
    return [o for o in new if o.parent is None or o.parent not in new]


def set_pose(arms, action, frame):
    for arm in arms:
        ad = arm.animation_data or arm.animation_data_create()
        for t in list(ad.nla_tracks):
            ad.nla_tracks.remove(t)
        act = bpy.data.actions[action]
        ad.action = act
        if hasattr(ad, 'action_slot') and getattr(act, 'slots', None):
            try:
                ad.action_slot = act.slots[0]
            except Exception:
                pass
    bpy.context.scene.frame_set(int(frame))


def build_part(spec, frame_ref, action):
    """Loads one model, scaled to its size target, footprint centred on the
    origin and standing on z=0. Returns its unit empty."""
    sc = bpy.context.scene
    tops = load(spec['file'], spec)
    unit = bpy.data.objects.new('unit', None)
    sc.collection.objects.link(unit)
    for o in tops:
        o.parent = unit
    kids = list(all_children(unit))
    recolour(kids, spec.get('recolour'))
    arms = [o for o in kids if o.type == 'ARMATURE']
    if action and arms:
        set_pose(arms, action, frame_ref)
    elif spec.get('frame') is not None:
        sc.frame_set(int(spec['frame']))
    bpy.context.view_layer.update()
    dg = bpy.context.evaluated_depsgraph_get()
    size = spec.get('size')
    if size:
        lo, hi = world_bounds(kids, dg)
        d = hi - lo
        k = list(size.keys())[0]
        ref = {'height': d.z, 'long': max(d.x, d.y), 'x': d.x, 'y': d.y}[k]
        unit.scale = (size[k] / ref,) * 3
        bpy.context.view_layer.update()
        dg = bpy.context.evaluated_depsgraph_get()
    lo, hi = world_bounds(kids, dg)
    cx, cy = (lo.x + hi.x) / 2, (lo.y + hi.y) / 2
    py = math.radians(spec.get('part_yaw', 0))
    unit.rotation_euler = (0, 0, py)
    unit.location = (-(cx * math.cos(py) - cy * math.sin(py)), -(cx * math.sin(py) + cy * math.cos(py)), -lo.z)
    pos = spec.get('pos')
    if pos:
        unit.location += Vector((pos[0], -pos[1], pos[2]))
    return unit


def build_entry(spec, frame_ref, action):
    group = bpy.data.objects.new('group', None)
    bpy.context.scene.collection.objects.link(group)
    if 'parts' in spec:
        for p in spec['parts']:
            u = build_part(p, frame_ref, action)
            u.parent = group
        if spec.get('size'):
            bpy.context.view_layer.update()
            dg = bpy.context.evaluated_depsgraph_get()
            kids = list(all_children(group))
            lo, hi = world_bounds(kids, dg)
            d = hi - lo
            k = list(spec['size'].keys())[0]
            ref = {'height': d.z, 'long': max(d.x, d.y), 'x': d.x, 'y': d.y}[k]
            group.scale = (spec['size'][k] / ref,) * 3
            bpy.context.view_layer.update()
            dg = bpy.context.evaluated_depsgraph_get()
            lo, hi = world_bounds(kids, dg)
            group.location = (-(lo.x + hi.x) / 2, -(lo.y + hi.y) / 2, -lo.z)
    else:
        u = build_part(spec, frame_ref, action)
        u.parent = group
    return group


def render(name, spec, d, frame_idx):
    sc, co = RI['reset']()
    action = spec.get('action')
    frames = spec.get('frames')
    ref = frames[0] if frames else None
    group = build_entry(spec, ref, action)
    # the pose may differ from the reference frame: set it after sizing
    if frames and frame_idx is not None:
        arms = [o for o in all_children(group) if o.type == 'ARMATURE']
        set_pose(arms, action, frames[frame_idx])
    pivot = bpy.data.objects.new('pivot', None)
    sc.collection.objects.link(pivot)
    group.parent = pivot
    pd = spec.get('per_dir', {}).get(d, {})
    if 'yaw_abs' in pd:
        yaw = pd['yaw_abs']
    else:
        yaw = YAW_SIGN * YAW[d] + YAW_OFFSET + spec.get('yaw', 0) + pd.get('yaw', 0) + (180 if str(spec.get('file', '')).startswith('proc:') else 0)
    pivot.rotation_euler = (0, 0, math.radians(yaw))
    off = pd.get('offset')
    if off:
        pivot.location = (off[0], -off[1], 0)
    bpy.context.view_layer.update()
    if os.environ.get('BMS_DEBUG'):
        lo, hi = world_bounds(list(all_children(pivot)), bpy.context.evaluated_depsgraph_get())
        print('BOUNDS', name, d, [round(v, 2) for v in lo], [round(v, 2) for v in hi])
    RI['place_camera'](co, ELEV)
    fn = name + '_' + d + ('' if frame_idx is None else '_' + str(frame_idx)) + '.png'
    sc.render.filepath = os.path.join(OUT, fn)
    bpy.ops.render.render(write_still=True)
    return fn


os.makedirs(OUT, exist_ok=True)
RI['write_proj']()
manifest = json.load(open(MANIFEST, encoding='utf-8'))
done = []
for name, spec in manifest['entries'].items():
    for d in spec['dirs']:
        idxs = range(len(spec['frames'])) if spec.get('frames') else [None]
        for i in idxs:
            fn = name + '_' + d + ('' if i is None else '_' + str(i))
            if ONLY and name not in ONLY and fn not in ONLY:
                continue
            done.append(render(name, spec, d, i))
if not ONLY:
    json.dump(sorted(used_files), open(os.path.join(OUT, 'sources.json'), 'w', newline='\n'), indent=1)
print('rendered', len(done), 'sprites into', OUT)
