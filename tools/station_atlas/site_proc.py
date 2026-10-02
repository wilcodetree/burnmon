# Procedural props for render_site.py (manifest "file": "proc:<name>"). Built from
# boxes, cylinders and bars in Blender, in the flat saturated look of the Kenney
# kits, for the site pieces no CC0 kit has (fence panel, desk, chair, barrier,
# light tower, scaffold, crane mast ...). Original work for this repo, no
# third-party licence. Units are tiles (1 unit = 1 tile), the model sits on z=0,
# its native front faces -Y (the same as a glTF model's +Z after import).
# Every builder returns the objects it created; render_site.py scales and
# centres them from the manifest's size target. Pure ASCII.
import math
import bpy
from mathutils import Vector

_mats = {}


def lin(c):
    c = c / 255.0
    return c / 12.92 if c <= 0.04045 else ((c + 0.055) / 1.055) ** 2.4


def rgb(hexcol):
    h = hexcol.lstrip('#')
    return tuple(lin(int(h[i:i + 2], 16)) for i in (0, 2, 4))


def mat(hexcol, rough=0.75):
    k = (hexcol, rough)
    if k in _mats:
        try:
            _mats[k].name
        except ReferenceError:      # render_site.py resets the scene per sprite
            del _mats[k]
    if k not in _mats:
        m = bpy.data.materials.new('p' + hexcol)
        m.use_nodes = True
        b = m.node_tree.nodes['Principled BSDF']
        b.inputs['Base Color'].default_value = rgb(hexcol) + (1.0,)
        b.inputs['Roughness'].default_value = rough
        m.diffuse_color = rgb(hexcol) + (1.0,)
        _mats[k] = m
    return _mats[k]


def box(c, s, col, rot=None):
    # c: centre, s: size (x, y, z).
    bpy.ops.mesh.primitive_cube_add(size=1, location=c)
    o = bpy.context.object
    o.scale = s
    if rot:
        o.rotation_euler = rot
    o.data.materials.append(mat(col))
    return o


def cyl(c, r, h, col, rot=None, v=14):
    bpy.ops.mesh.primitive_cylinder_add(vertices=v, radius=r, depth=h, location=c)
    o = bpy.context.object
    if rot:
        o.rotation_euler = rot
    o.data.materials.append(mat(col))
    return o


def bar(p0, p1, t, col):
    # A square bar of thickness t from p0 to p1.
    a, b = Vector(p0), Vector(p1)
    d = b - a
    bpy.ops.mesh.primitive_cube_add(size=1, location=(a + b) / 2)
    o = bpy.context.object
    o.scale = (t, t, d.length)
    o.rotation_euler = d.to_track_quat('Z', 'Y').to_euler()
    o.data.materials.append(mat(col))
    return o


def fence():
    # A Heras style panel along X, one tile long, standing on two concrete feet.
    out = []
    L, H, z0 = 0.96, 0.62, 0.05
    steel, foot = '#c9ced6', '#8b9099'
    for x in (-L / 2 + 0.02, L / 2 - 0.02):
        out.append(box((x, 0, H / 2 + z0), (0.04, 0.04, H), steel))
    for z in (z0 + 0.02, z0 + H - 0.02):
        out.append(box((0, 0, z), (L, 0.04, 0.04), steel))
    for i in range(1, 6):
        out.append(box((-L / 2 + 0.02 + i * (L - 0.04) / 6, 0, z0 + H / 2), (0.02, 0.02, H), steel))
    out.append(bar((-L / 2 + 0.02, 0, z0 + 0.02), (L / 2 - 0.02, 0, z0 + H - 0.02), 0.02, steel))
    for x in (-0.36, 0.36):
        out.append(box((x, 0, 0.03), (0.16, 0.34, 0.06), foot))
    return out


def light():
    # A light tower: wide base, mast, lamp bar with four lamps.
    out = [box((0, 0, 0.04), (0.5, 0.5, 0.08), '#f0b323'), box((0, 0, 0.14), (0.3, 0.3, 0.12), '#2f343c')]
    out.append(cyl((0, 0, 1.0), 0.03, 1.7, '#9aa1ab'))
    out.append(box((0, 0, 1.9), (0.8, 0.12, 0.08), '#2f343c'))
    for x in (-0.3, -0.1, 0.1, 0.3):
        out.append(box((x, -0.08, 1.9), (0.16, 0.08, 0.2), '#fff3b0'))
    return out


def shelf():
    # A plan cabinet: open rack with rolled drawings.
    out = []
    for x in (-0.4, 0.4):
        for y in (-0.22, 0.22):
            out.append(box((x, y, 0.45), (0.05, 0.05, 0.9), '#8b9099'))
    for z, c in ((0.04, '#5a606b'), (0.3, '#b9c0c9'), (0.58, '#b9c0c9'), (0.88, '#5a606b')):
        out.append(box((0, 0, z), (0.84, 0.5, 0.05), c))
    for z, cols in ((0.35, ('#f7f2e8', '#cfe3f5', '#f7f2e8')), (0.63, ('#cfe3f5', '#f7f2e8', '#f7f2e8'))):
        for i, c in enumerate(cols):
            out.append(cyl((-0.25 + i * 0.25, 0, z + 0.05), 0.05, 0.4, c, rot=(math.pi / 2, 0, 0)))
    return out


def board():
    # A whiteboard on an A-frame stand, face to the front (-Y).
    out = [box((0, 0, 0.62), (0.9, 0.04, 0.6), '#f4f6f8'), box((0, 0.0, 0.62), (0.96, 0.03, 0.66), '#9aa1ab')]
    out[0].location.y = -0.02
    for x in (-0.38, 0.38):
        out.append(bar((x, 0.0, 0.3), (x, 0.28, 0.0), 0.04, '#5a606b'))
        out.append(bar((x, 0.0, 0.3), (x, -0.28, 0.0), 0.04, '#5a606b'))
    out.append(box((-0.15, -0.045, 0.7), (0.4, 0.01, 0.03), '#2a6fdb'))
    out.append(box((0.1, -0.045, 0.6), (0.5, 0.01, 0.03), '#d83a2e'))
    out.append(box((-0.1, -0.045, 0.5), (0.3, 0.01, 0.03), '#2a6fdb'))
    return out


def sign():
    # A warning sign (triangle on a plate) on a post.
    out = [box((0, 0, 0.04), (0.3, 0.3, 0.08), '#5a606b'), cyl((0, 0, 0.5), 0.025, 0.9, '#9aa1ab')]
    out.append(box((0, -0.03, 0.88), (0.5, 0.03, 0.5), '#f0b323', rot=(0, math.pi / 4, 0)))
    out.append(box((0, -0.05, 0.84), (0.07, 0.02, 0.22), '#1f2328'))
    out.append(box((0, -0.05, 0.69), (0.07, 0.02, 0.07), '#1f2328'))
    return out


def desk():
    # Desk with a pedestal of drawers on the right, front to -Y.
    out = [box((0, 0, 0.42), (0.9, 0.55, 0.06), '#d8a867')]
    for x in (-0.42,):
        for y in (-0.22, 0.22):
            out.append(box((x, y, 0.2), (0.05, 0.05, 0.4), '#7a808a'))
    out.append(box((0.3, 0, 0.2), (0.3, 0.5, 0.4), '#b98c4e'))
    for z in (0.12, 0.28):
        out.append(box((0.3, -0.26, z), (0.2, 0.02, 0.1), '#e8c58f'))
    return out


def chair():
    # A small office chair: five-star base, seat, back; front to -Y.
    out = [cyl((0, 0, 0.17), 0.025, 0.2, '#2f343c')]
    for a in range(5):
        t = a * 2 * math.pi / 5
        out.append(bar((0, 0, 0.07), (0.17 * math.cos(t), 0.17 * math.sin(t), 0.03), 0.035, '#2f343c'))
    out.append(box((0, 0, 0.29), (0.32, 0.32, 0.07), '#2a6fdb'))
    out.append(box((0, 0.15, 0.5), (0.32, 0.06, 0.34), '#2a6fdb'))
    return out


def table():
    # A square table: a top and four legs.
    out = [box((0, 0, 0.4), (0.96, 0.96, 0.07), '#d8a867')]
    for x in (-0.4, 0.4):
        for y in (-0.4, 0.4):
            out.append(box((x, y, 0.18), (0.07, 0.07, 0.36), '#7a808a'))
    return out


def barrier():
    # A boom barrier: post housing and a striped arm, arm along X, 2 tiles wide.
    out = [box((-0.9, 0, 0.3), (0.22, 0.22, 0.6), '#f0b323'), box((-0.9, -0.12, 0.42), (0.1, 0.02, 0.14), '#2f343c')]
    n = 10
    for i in range(n):
        x0 = -0.78 + i * (1.78 / n)
        out.append(box((x0 + 0.89 / n, 0, 0.55), (1.78 / n, 0.06, 0.08), '#d83a2e' if i % 2 == 0 else '#f4f6f8'))
    out.append(box((0.98, 0, 0.18), (0.08, 0.08, 0.36), '#9aa1ab'))
    return out


def barricade():
    # A road barricade: two boards, red and white, on A-frame legs.
    out = []
    for z, flip in ((0.52, 0), (0.34, 1)):
        n = 6
        for i in range(n):
            out.append(box((-0.45 + (i + 0.5) * 0.9 / n, 0, z), (0.9 / n, 0.04, 0.14), '#d83a2e' if (i + flip) % 2 == 0 else '#f4f6f8'))
    for x in (-0.38, 0.38):
        out.append(bar((x, 0, 0.6), (x, 0.2, 0.0), 0.04, '#9aa1ab'))
        out.append(bar((x, 0, 0.6), (x, -0.2, 0.0), 0.04, '#9aa1ab'))
    return out


def bricks():
    # A pallet of bricks: three slats, rows of brick blocks.
    out = [box((0, 0, 0.05), (0.9, 0.9, 0.04), '#b98c4e')]
    for y in (-0.38, 0, 0.38):
        out.append(box((0, y, 0.025), (0.9, 0.12, 0.05), '#a47a40'))
    for r in range(4):
        for i in range(3):
            for j in range(2):
                out.append(box((-0.28 + i * 0.28, -0.2 + j * 0.4, 0.14 + r * 0.1), (0.26, 0.38, 0.09), '#c4552e' if (i + j + r) % 3 else '#b4472a'))
    return out


def pallet():
    out = [box((0, 0, 0.1), (0.9, 0.9, 0.04), '#c9965a')]
    for y in (-0.38, 0, 0.38):
        out.append(box((0, y, 0.04), (0.9, 0.14, 0.08), '#b4834a'))
    for x in (-0.4, 0, 0.4):
        out.append(box((x, 0, 0.0), (0.12, 0.9, 0.04), '#a47a40'))
    return out


def deck():
    # A scaffold deck board.
    return [box((0, 0, 0.02), (0.9, 0.9, 0.04), '#c9965a'), box((0, 0.2, 0.045), (0.9, 0.02, 0.01), '#a47a40'), box((0, -0.2, 0.045), (0.9, 0.02, 0.01), '#a47a40')]


def wallstep():
    # The unfinished end of a brick wall: courses of bricks stepping down along Y,
    # plus two loose bricks. Built to sit beside Building Kit's wall-low.
    out = []
    for i, n in enumerate((5, 4, 2, 1)):
        for r in range(n):
            out.append(box((0, -0.15 + i * 0.1, 0.05 + r * 0.1), (0.09, 0.095, 0.095), '#c4552e' if (i + r) % 2 else '#b4472a'))
    out.append(box((0.12, 0.12, 0.03), (0.07, 0.1, 0.06), '#c4552e'))
    out.append(box((-0.12, 0.05, 0.03), (0.07, 0.1, 0.06), '#b4472a'))
    return out


def hut():
    # A site cabin: green-grey container body with a door, two windows and a roof lip.
    out = [box((0, 0, 0.45), (1.6, 0.8, 0.8), '#6c8b93'), box((0, 0, 0.88), (1.66, 0.86, 0.06), '#4f6870')]
    out.append(box((-0.45, -0.405, 0.4), (0.3, 0.03, 0.65), '#e8d9a8'))
    for x in (0.1, 0.5):
        out.append(box((x, -0.405, 0.55), (0.28, 0.03, 0.26), '#aee0f5'))
        out.append(box((x, -0.42, 0.4), (0.32, 0.03, 0.04), '#4f6870'))
    out.append(box((-0.45, -0.5, 0.04), (0.5, 0.2, 0.08), '#8b9099'))
    return out


def generator():
    # A site generator: frame, engine block, exhaust, fuel tank on top; front -Y.
    out = [box((0, 0, 0.06), (1.0, 0.6, 0.12), '#2f343c'), box((0, 0, 0.42), (0.9, 0.52, 0.6), '#f0b323')]
    out.append(box((0, -0.27, 0.42), (0.7, 0.03, 0.4), '#2f343c'))
    out.append(box((-0.2, -0.3, 0.42), (0.2, 0.03, 0.22), '#7fd1ff'))
    out.append(box((0.2, -0.3, 0.45), (0.14, 0.03, 0.14), '#d83a2e'))
    out.append(cyl((0.35, 0.15, 0.85), 0.04, 0.3, '#5a606b'))
    out.append(box((0, 0, 0.76), (0.9, 0.52, 0.06), '#c98f12'))
    return out


def skip():
    # An open skip: tapered body made of slabs, two lifting eyes.
    out = [box((0, 0, 0.04), (1.0, 0.7, 0.08), '#2f343c')]
    out.append(box((0, -0.34, 0.3), (1.0, 0.04, 0.5), '#e8a21a', rot=(math.radians(-14), 0, 0)))
    out.append(box((0, 0.34, 0.3), (1.0, 0.04, 0.5), '#e8a21a', rot=(math.radians(14), 0, 0)))
    for x in (-0.49, 0.49):
        out.append(box((x, 0, 0.3), (0.04, 0.8, 0.5), '#e8a21a'))
    out.append(box((0, 0, 0.12), (0.9, 0.6, 0.04), '#8a8f98'))
    for i in range(3):
        out.append(box((-0.3 + i * 0.3, 0.0, 0.2 + (i % 2) * 0.05), (0.22, 0.2, 0.14), '#b4834a'))
    return out


def bin_():
    # A wheelie bin: body, lid, two wheels.
    out = [box((0, 0, 0.28), (0.4, 0.36, 0.52), '#3b8d4c'), box((0, 0.01, 0.57), (0.44, 0.4, 0.07), '#2c6f3a')]
    out.append(box((0, -0.19, 0.52), (0.3, 0.03, 0.04), '#2c6f3a'))
    for x in (-0.19, 0.19):
        out.append(cyl((x, 0.16, 0.08), 0.08, 0.05, '#1f2328', rot=(0, math.pi / 2, 0)))
    return out


def crane():
    # A tower crane mast: concrete base, lattice mast, slewing head. The jib is drawn in code.
    out = [box((0, 0, 0.1), (1.1, 1.1, 0.2), '#8b9099')]
    H, w = 3.5, 0.3
    steel, dia = '#f0b323', '#d99a10'
    for x in (-w, w):
        for y in (-w, w):
            out.append(box((x, y, 0.2 + H / 2), (0.05, 0.05, H), steel))
    n = 10
    for i in range(n):
        z0, z1 = 0.2 + i * H / n, 0.2 + (i + 1) * H / n
        a, b = (-w, w) if i % 2 == 0 else (w, -w)
        for x in (-w, w):
            out.append(bar((x, a, z0), (x, b, z1), 0.03, dia))
        for y in (-w, w):
            out.append(bar((a, y, z0), (b, y, z1), 0.03, dia))
        out.append(box((0, 0, z1), (2 * w + 0.05, 0.04, 0.04), steel))
        out.append(box((0, 0, z1), (0.04, 2 * w + 0.05, 0.04), steel))
    top = 0.2 + H
    out.append(box((0, 0, top + 0.1), (0.8, 0.8, 0.2), '#2f343c'))
    out.append(box((0, 0, top + 0.35), (0.6, 0.6, 0.3), '#f0b323'))
    out.append(box((0, -0.32, top + 0.38), (0.3, 0.04, 0.2), '#aee0f5'))
    return out


BUILD = {k: v for k, v in globals().items() if callable(v) and k in (
    'fence', 'light', 'shelf', 'board', 'sign', 'desk', 'chair', 'table', 'barrier', 'barricade', 'bricks',
    'pallet', 'deck', 'wallstep', 'hut', 'generator', 'skip', 'crane')}
BUILD['bin'] = bin_
