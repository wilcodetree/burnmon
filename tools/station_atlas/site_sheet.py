# Contact sheet for render_site.py's output, at 1x, every sprite on its tile
# centre (magenta cross), with the space atlas's astronautA_SE standing left of
# it as a ruler, and for the fence the space atlas's corridor_wall as a ghost
# on the same anchor. Run from this folder:
#   python site_sheet.py <renders dir> <out.png>
import base64, io, json, math, os, re, sys
from PIL import Image, ImageDraw

REN, OUT = sys.argv[1], sys.argv[2]
ATLAS = os.path.join('..', '..', 'cmd', 'burnmon-dev', 'station', 'station_atlas.js')
txt = open(ATLAS, encoding='utf-8').read()
png = Image.open(io.BytesIO(base64.b64decode(re.search(r"base64,([^']*)'", txt).group(1)))).convert('RGBA')
smap = json.loads(re.search(r"map: (\{.*\})\n\};", txt).group(1))
AX, AY = 256, 311.5


def space(n):
    x, y, w, h, ox, oy = smap[n]
    return png.crop((x, y, x + w, y + h)), ox, oy


def load(f):
    im = Image.open(os.path.join(REN, f)).convert('RGBA')
    bb = im.getbbox()
    return im.crop(bb), bb[0] - AX, bb[1] - AY


fs = sorted(f for f in os.listdir(REN) if f.endswith('.png') and f.startswith('site_'))
workers = sorted(f for f in os.listdir(REN) if f.endswith('.png') and f.startswith('worker'))
BG = (70, 76, 90, 255)
CW, CH, GY = 200, 210, 160          # cell size, ground line y in a cell
TALL = [f for f in fs if load(f)[2] < -GY + 20]
NORM = [f for f in fs if f not in TALL]
cols = 8
rows_n = (len(NORM) + cols - 1) // cols
tall_h = 400
W = cols * CW
sheet = Image.new('RGBA', (W, tall_h + rows_n * CH + 4 * 100 + 40), BG)
dr = ImageDraw.Draw(sheet)
astro, aox, aoy = space('astronautA_SE')


def put(f, x0, y0, gy, ghost=None):
    im, ox, oy = load(f)
    ax = x0 + CW // 2 + 30
    ay = y0 + gy
    if ghost:
        g, gx, gy2 = space(ghost)
        g = g.copy(); g.putalpha(g.getchannel('A').point(lambda v: v // 2))
        sheet.alpha_composite(g, (int(ax + gx), int(ay + gy2)))
    sheet.alpha_composite(astro, (int(ax - 70 + aox), int(ay + aoy)))
    sheet.alpha_composite(im, (int(ax + ox), int(ay + oy)))
    dr.line((ax - 4, ay, ax + 4, ay), fill=(255, 0, 255, 255)); dr.line((ax, ay - 4, ax, ay + 4), fill=(255, 0, 255, 255))
    dr.text((x0 + 3, y0 + 3), f[:-4] + ' %dx%d' % im.size, fill=(255, 255, 255, 255))


for i, f in enumerate(TALL):
    put(f, i * CW, 0, tall_h - 40)
for i, f in enumerate(NORM):
    ghost = 'corridor_wall_' + f[-6:-4] if f.startswith('site_fence') else None
    put(f, (i % cols) * CW, tall_h + (i // cols) * CH, GY, ghost)
y0 = tall_h + rows_n * CH
D8 = ['N', 'NE', 'E', 'SE', 'S', 'SW', 'W', 'NW']
for r, w in enumerate(('workerA', 'workerB', 'workerC')):
    for k, d in enumerate(D8):
        for fr in (0, 1):
            fn = '%s_%s_%d.png' % (w, d, fr)
            if not os.path.exists(os.path.join(REN, fn)):
                continue
            im, ox, oy = load(fn)
            x = (k * 2 + fr) * (W // 16) + (W // 32)
            y = y0 + r * 100 + 85
            sheet.alpha_composite(im, (int(x + ox), int(y + oy)))
            dr.line((x - 3, y, x + 3, y), fill=(255, 0, 255, 255))
            if r == 0 and fr == 0:
                dr.text((x - 20, y0 + r * 100 + 2), d, fill=(255, 255, 255, 255))
sheet.alpha_composite(astro, (int(14 + aox + 20), int(y0 + 85 + aoy)))
sheet = sheet.crop((0, 0, W, y0 + 300 + 10))
sheet.save(OUT)
print('sheet ->', OUT, sheet.size)
