# Measures, per sprite, how far render_iso.py's 30 degree render at the
# kit's own view (azimuth 45) sits from the kit's PNG: the shift between the
# two bounding-box centres, in pixels. render_iso.py turns each shift into a
# ground offset and applies it, so every sprite lands where Kenney put it.
#   python shifts_iso.py <kit> <renders at 30, azimuth 45> <shifts.json>
import json, os, sys
from PIL import Image

KIT, REF, OUT = sys.argv[1], sys.argv[2], sys.argv[3]
def centre(p):
    bb = Image.open(p).convert('RGBA').getbbox()
    return ((bb[0] + bb[2]) / 2, (bb[1] + bb[3]) / 2)
shifts = {}
for f in sorted(os.listdir(REF)):
    if f.endswith('.png'):
        k, m = centre(os.path.join(KIT, 'Isometric', f)), centre(os.path.join(REF, f))
        shifts[f[:-4]] = [round(k[0] - m[0], 2), round(k[1] - m[1], 2)]
json.dump(shifts, open(OUT, 'w', newline='\n'), indent=1, sort_keys=True)
print(len(shifts), 'shifts ->', OUT)
