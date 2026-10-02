# Checks render_iso.py against the kit's own 30 degree Isometric PNGs: for
# each sample sprite, the alpha masks' overlap (IoU) and the shift between
# their bounding boxes. A faithful setup scores IoU near 1 and a shift of a
# pixel or two. Run from this folder after rendering the samples at 30:
#   blender -b -P render_iso.py -- <kit> <out> 30 desk_computer gate_simple astronautA
#   python calibrate_iso.py <kit> <out>
import os, sys
from PIL import Image, ImageChops

KIT, OUT = sys.argv[1], sys.argv[2]
scores = []
for f in sorted(os.listdir(OUT)):
    if not f.endswith('.png'):
        continue
    a = Image.open(os.path.join(KIT, 'Isometric', f)).convert('RGBA').getchannel('A').point(lambda v: 255 if v > 32 else 0)
    b = Image.open(os.path.join(OUT, f)).convert('RGBA').getchannel('A').point(lambda v: 255 if v > 32 else 0)
    inter = ImageChops.multiply(a, b).histogram()[255]
    union = ImageChops.lighter(a, b).histogram()[255]
    ba, bb = a.getbbox(), b.getbbox()
    shift = (bb[0] - ba[0], bb[1] - ba[1], bb[2] - ba[2], bb[3] - ba[3]) if ba and bb else None
    scores.append(inter / max(1, union))
    if '-q' not in sys.argv: print('%-32s IoU %.3f  bbox kit %s mine %s  shift %s' % (f, inter / max(1, union), ba, bb, shift))
print('mean IoU %.3f over %d sprites, min %.3f' % (sum(scores) / len(scores), len(scores), min(scores)))
