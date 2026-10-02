# Side-by-side check sheet for render_iso.py: row 1 is the reference render
# (the kit's own PNG, or an empty cell when there is none), row 2 is ours.
#   python make_sheet.py <reference dir> <render dir> <out.png>
import os, sys
from PIL import Image

ref, ours, out = sys.argv[1], sys.argv[2], sys.argv[3]
fs = sorted(f for f in os.listdir(ours) if f.endswith('.png'))
C = 200
sheet = Image.new('RGBA', (len(fs) * C, 2 * C), (40, 44, 52, 255))
for i, f in enumerate(fs):
    for j, src in enumerate((ref, ours)):
        p = os.path.join(src, f)
        if os.path.exists(p):
            im = Image.open(p).convert('RGBA').crop((156, 180, 356, 380))
            sheet.alpha_composite(im, (i * C, j * C))
sheet.save(out)
print('sheet ->', out)
