# Builds the self-contained Station preview (fake sessions, P and O keys)
# into 04_assets. Run from this folder: python build.py
import os
S = os.path.join('..', '..', '..', 'cmd', 'burnmon-dev', 'station')
OUT = os.path.join('..', '..', '..', '04_assets', '2026-10-01_station_preview.html')
def rd(p):
    return open(p, encoding='utf-8').read()
parts = ['<!doctype html><html><head><meta charset="utf-8"><title>BurnMon Station preview</title></head><body>',
         rd('preview_body.html'),
         '<script>' + rd(os.path.join(S, 'station_atlas.js')) + '</script>',
         '<script>' + rd(os.path.join(S, 'station.js')) + '</script>',
         '<script>' + rd('harness.js') + '</script>',
         '<script>' + rd('wire.js') + '</script>', '</body></html>']
open(OUT, 'w', encoding='utf-8').write('\n'.join(parts))
print('wrote', OUT)
