import struct, io, os
from PIL import Image
HERE = os.path.dirname(os.path.abspath(__file__))
ROOT = os.path.dirname(HERE)
VER = open(os.path.join(ROOT, 'version.txt')).read().strip() if os.path.exists(os.path.join(ROOT, 'version.txt')) else '1.0.0'
VP = [int(x) for x in (VER.split('.') + ['0','0','0'])[:3]]
src = Image.open(os.path.join(HERE, 'icon256.png')).convert('RGBA')
sizes = [16,20,24,32,40,48,64,128,256]
imgs = []
for s in sizes:
    im = src.resize((s,s), Image.LANCZOS)
    b = io.BytesIO(); im.save(b,'PNG'); imgs.append((s,b.getvalue()))
manifest = open(os.path.join(HERE, 'app.manifest'),'rb').read().replace(b'1.0.0.0', ('%d.%d.%d.0' % tuple(VP)).encode())

# recursos: tipo -> [(id, data)]
res = {3: [(i+1, d) for i,(s,d) in enumerate(imgs)]}
grp = struct.pack('<HHH',0,1,len(imgs))
for i,(s,d) in enumerate(imgs):
    w = 0 if s>=256 else s
    grp += struct.pack('<BBBBHHIH', w, w, 0, 0, 1, 32, len(d), i+1)
res[14] = [(1, grp)]
res[24] = [(1, manifest)]
LANG = 0x0409
def pad4(b): return b + b'\0'*((4-len(b)%4)%4)
def node(key, value=b'', vlen=0, typ=0, children=b''):
    k = (key+'\0').encode('utf-16-le')
    body = pad4(struct.pack('<HHH',0,vlen,typ)+k)
    body = pad4(body + value) + children
    return struct.pack('<H',len(body)) + body[2:]
def sval(key, text):
    v=(text+'\0').encode('utf-16-le'); return pad4(node(key, v, len(v)//2, 1))
fixed = struct.pack('<13I',0xFEEF04BD,0x00010000,(VP[0]<<16)|VP[1],VP[2]<<16,(VP[0]<<16)|VP[1],VP[2]<<16,0x3F,0,0x40004,1,0,0,0)
strs = b''.join(sval(k,v) for k,v in [('CompanyName','@byalvaro__'),('FileDescription','Vantix Utilities'),('FileVersion',VER),('InternalName','Vantix'),('LegalCopyright','© 2026 @byalvaro__'),('OriginalFilename','Vantix.exe'),('ProductName','Vantix Utilities'),('ProductVersion',VER)])
st = pad4(node('0C0A04B0', b'', 0, 1, strs))
sfi = pad4(node('StringFileInfo', b'', 0, 1, st))
var = pad4(node('Translation', struct.pack('<HH',0x0C0A,0x04B0), 4, 0))
vfi = pad4(node('VarFileInfo', b'', 0, 1, var))
ver = node('VS_VERSION_INFO', fixed, len(fixed), 0, sfi+vfi)
res[16] = [(1, ver)]

# Disposición: dir raíz, dirs de tipo, dirs de idioma, entradas de datos, datos
def dirsize(n): return 16 + 8*n
types = sorted(res)
off = dirsize(len(types))
type_off = {}
for t in types: type_off[t]=off; off += dirsize(len(res[t]))
lang_off = {}
for t in types:
    for (i,_) in res[t]: lang_off[(t,i)]=off; off += dirsize(1)
entry_off = {}
for t in types:
    for (i,_) in res[t]: entry_off[(t,i)]=off; off += 16
data_off = {}
for t in types:
    for (i,d) in res[t]:
        off = (off+7)&~7; data_off[(t,i)]=off; off += len(d)
total = (off+7)&~7
buf = bytearray(total)
def wdir(at, entries):
    struct.pack_into('<IIHHHH', buf, at, 0,0,0,0,0,len(entries))
    for k,(name,target,isdir) in enumerate(entries):
        struct.pack_into('<II', buf, at+16+8*k, name, target | (0x80000000 if isdir else 0))
wdir(0, [(t, type_off[t], True) for t in types])
relocs=[]
for t in types:
    wdir(type_off[t], [(i, lang_off[(t,i)], True) for (i,_) in res[t]])
    for (i,d) in res[t]:
        wdir(lang_off[(t,i)], [(LANG, entry_off[(t,i)], False)])
        struct.pack_into('<IIII', buf, entry_off[(t,i)], data_off[(t,i)], len(d), 0, 0)
        relocs.append(entry_off[(t,i)])
        buf[data_off[(t,i)]:data_off[(t,i)]+len(d)] = d

hdr_size = 20 + 40
raw_ptr = hdr_size
rel_ptr = raw_ptr + total
sym_ptr = rel_ptr + 10*len(relocs)
out = bytearray()
out += struct.pack('<HHIIIHH', 0x8664, 1, 0, sym_ptr, 1, 0, 0)
out += struct.pack('<8sIIIIIIHHI', b'.rsrc\0\0\0', 0, 0, total, raw_ptr, rel_ptr, 0, len(relocs), 0, 0x40000040)
out += buf
for r in relocs: out += struct.pack('<IIH', r, 0, 3)
out += struct.pack('<8sIhHBB', b'.rsrc\0\0\0', 0, 1, 0, 3, 0)
out += struct.pack('<I', 4)
open(os.path.join(ROOT, 'rsrc_windows_amd64.syso'),'wb').write(out)
src.resize((256,256)).save(os.path.join(HERE, 'vantix.ico'), sizes=[(s,s) for s in [16,24,32,48,64,128,256]])
print('ok', len(out))
